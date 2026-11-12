//go:build integration

package tests

import (
	"fmt"
	"math/rand/v2"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/cog-forge/rolecue/api/internal/features/interview/domain"
	"github.com/cog-forge/rolecue/api/internal/features/interview/repository"
	"github.com/cog-forge/rolecue/api/internal/features/interview/service"
	"github.com/cog-forge/rolecue/api/internal/pkg/llmbudget"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Metric-coverage sampling (KAN-333): a session's questions cover every active metric of its posting,
// a start that cannot be covered is refused without side effects, and every turn maps to the core question
// it belongs to. The fixtures use rubrics seeded here, never the shared system templates.

// catalogMetrics returns the metric catalog ordered by id.
func (s *seed) catalogMetrics() []uuid.UUID {
	s.t.Helper()
	rows, err := s.pool.Query(s.ctx, `SELECT id FROM public.metrics ORDER BY id`)
	if err != nil {
		s.t.Fatal(err)
	}
	defer rows.Close()
	var out []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			s.t.Fatal(err)
		}
		out = append(out, id)
	}
	if err := rows.Err(); err != nil {
		s.t.Fatal(err)
	}
	if len(out) < 4 {
		s.t.Fatalf("the metric catalog has %d rows, the sampling tests need at least 4", len(out))
	}
	return out
}

// rubric adds an active system rubric with one criterion for each of the metrics (none gives a rubric with
// no criteria) and returns its id. The rubric is removed with the seed.
func (s *seed) rubric(metrics ...uuid.UUID) uuid.UUID {
	s.t.Helper()
	id := uuid.New()
	s.exec(`INSERT INTO public.rubrics (id, name, question_type, scope) VALUES ($1, $2, 'sampling_test', 'system')`,
		id, "sampling test "+id.String())
	s.rubrics = append(s.rubrics, id.String())
	for i, metric := range metrics {
		s.exec(`INSERT INTO public.rubric_criteria (rubric_id, metric_id, name, max_points, scoring_guide, position)
			VALUES ($1, $2, $3, 4, 'guide', $4)`, id, metric, fmt.Sprintf("criterion %d", i+1), i+1)
	}
	return id
}

// bankOf adds one core question to the Job Description for each rubric; a nil entry is a question
// without a rubric. It returns the question ids in the same order.
func (s *seed) bankOf(jdID uuid.UUID, rubrics []*uuid.UUID) []uuid.UUID {
	s.t.Helper()
	ids := make([]uuid.UUID, 0, len(rubrics))
	for _, rubric := range rubrics {
		id := uuid.New()
		s.exec(`INSERT INTO public.core_questions (id, jd_id, content, rubric_id) VALUES ($1, $2, $3, $4)`,
			id, jdID, fmt.Sprintf("bank question %s", id), rubric)
		ids = append(ids, id)
	}
	return ids
}

// rubrics returns pointers to the rubric ids, for bankOf.
func rubricPtrs(ids ...uuid.UUID) []*uuid.UUID {
	out := make([]*uuid.UUID, len(ids))
	for i := range ids {
		out[i] = &ids[i]
	}
	return out
}

// weigh saves posting weights; a zero percentage is a zero weight.
func (s *seed) weigh(postingID uuid.UUID, weights map[uuid.UUID]float64) {
	s.t.Helper()
	for metric, percentage := range weights {
		s.exec(`INSERT INTO public.metrics_percentage (job_posting_id, metric_id, percentage) VALUES ($1, $2, $3)`,
			postingID, metric, percentage)
	}
}

// postingWithBank adds an open posting with room for slot interviews and a bank with one question for each
// entry of rubrics (nil for none).
func (s *seed) postingWithBank(slot int32, rubrics []*uuid.UUID) (uuid.UUID, []uuid.UUID) {
	s.t.Helper()
	recruiterID := s.user("recruiter")
	jdID := s.jobDescription()
	bank := s.bankOf(jdID, rubrics)
	postingID := uuid.New()
	s.exec(`INSERT INTO public.job_postings (id, status, jd_id, recruiter_id, title, interview_slot)
		VALUES ($1, 'open', $2, $3, 'sampling posting', $4)`, postingID, jdID, recruiterID, slot)
	s.postings = append(s.postings, postingID.String())
	return postingID, bank
}

// coveredMetrics returns the metrics the stored questions of a session assess.
func (s *seed) coveredMetrics(interviewID uuid.UUID) map[uuid.UUID]bool {
	s.t.Helper()
	rows, err := s.pool.Query(s.ctx, `SELECT DISTINCT rc.metric_id
		FROM public.interview_questions iq
		JOIN public.core_questions cq ON cq.id = iq.core_question_id
		JOIN public.rubric_criteria rc ON rc.rubric_id = cq.rubric_id
		WHERE iq.interview_id = $1`, interviewID)
	if err != nil {
		s.t.Fatal(err)
	}
	defer rows.Close()
	out := map[uuid.UUID]bool{}
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			s.t.Fatal(err)
		}
		out[id] = true
	}
	if err := rows.Err(); err != nil {
		s.t.Fatal(err)
	}
	return out
}

func (s *seed) assertCovered(interviewID uuid.UUID, active []uuid.UUID) {
	s.t.Helper()
	covered := s.coveredMetrics(interviewID)
	for _, metric := range active {
		if !covered[metric] {
			s.t.Fatalf("session %s does not cover active metric %s", interviewID, metric)
		}
	}
}

// questionRowsOfPosting counts the stored sampled questions of every session of a posting.
func (s *seed) questionRowsOfPosting(postingID uuid.UUID) int {
	s.t.Helper()
	return s.count(`SELECT count(*) FROM public.interview_questions iq
		JOIN public.interviews i ON i.id = iq.interview_id
		JOIN public.applications a ON a.id = i.application_id WHERE a.job_posting_id = $1`, postingID)
}

// newSeededService is newService over a repository whose random source is a seeded generator, so every
// start draws the same sequence.
func newSeededService(t *testing.T, pool *pgxpool.Pool, questionLimit int32, seed uint64) *service.Service {
	t.Helper()
	repo := repository.NewRepository(pool)
	repo.SetRandomSource(func() domain.RandomSource { return rand.New(rand.NewPCG(seed, seed+1)) })
	svc, err := service.New(repo, service.Settings{QuestionLimit: questionLimit})
	if err != nil {
		t.Fatal(err)
	}
	return svc
}

// sampleKey identifies a set of questions regardless of order.
func sampleKey(ids []uuid.UUID) string {
	sorted := slices.Clone(ids)
	slices.SortFunc(sorted, func(a, b uuid.UUID) int { return slices.Compare(a[:], b[:]) })
	return fmt.Sprint(sorted)
}

// TestRecruitmentStartCoversEveryWeightedMetric starts many sessions on one posting. Three metrics carry a
// weight and one has weight zero; every sampled set covers the three and is not the same for everyone.
func TestRecruitmentStartCoversEveryWeightedMetric(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	m := s.catalogMetrics()
	rA, rB, rC := s.rubric(m[0]), s.rubric(m[1], m[3]), s.rubric(m[2])
	var rubrics []*uuid.UUID
	for _, r := range []uuid.UUID{rA, rA, rA, rA, rB, rB, rB, rB, rC, rC, rC, rC} {
		rubrics = append(rubrics, &r)
	}
	const candidates = 10
	posting, bank := s.postingWithBank(candidates, rubrics)
	// m[3] has weight zero: the zero weight never makes it active, and nothing has to cover it.
	s.weigh(posting, map[uuid.UUID]float64{m[0]: 50, m[1]: 30, m[2]: 20, m[3]: 0})
	active := m[:3]
	svc := newService(t, pool, 3)

	sets := map[string]bool{}
	for range candidates {
		candidate := s.user("candidate")
		session, created, err := svc.Create(ctx, candidate, "candidate", recruitmentInput(s.eligibleApplication(posting, candidate)))
		if err != nil || !created {
			t.Fatalf("Create() = %v, created %v", err, created)
		}
		snap := s.snapshot(session.ID)
		if len(snap) != 3 || session.QuestionCount != 3 {
			t.Fatalf("snapshot %v / count %d, want 3 questions", snap, session.QuestionCount)
		}
		assertSampleOf(t, snap, bank)
		s.assertCovered(session.ID, active)
		sets[sampleKey(snap)] = true
	}
	if len(sets) < 2 {
		t.Fatalf("%d starts gave %d different sets, want the sampling to stay random", candidates, len(sets))
	}
}

// TestRecruitmentStartRetryAndResumeKeepTheAssignedSet proves a retry of the same start is the same session
// with the same questions and takes no further slot, and that a resume reads the stored set.
func TestRecruitmentStartRetryAndResumeKeepTheAssignedSet(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	m := s.catalogMetrics()
	rA, rB := s.rubric(m[0]), s.rubric(m[1])
	posting, _ := s.postingWithBank(1, rubricPtrs(rA, rA, rA, rB, rB, rB))
	s.weigh(posting, map[uuid.UUID]float64{m[0]: 50, m[1]: 50})
	svc := newService(t, pool, 4)
	candidate := s.user("candidate")
	application := s.eligibleApplication(posting, candidate)

	first, created, err := svc.Create(ctx, candidate, "candidate", recruitmentInput(application))
	if err != nil || !created {
		t.Fatalf("first start = %v, created %v", err, created)
	}
	want := s.snapshot(first.ID)
	for i := range 3 {
		again, created, err := svc.Create(ctx, candidate, "candidate", recruitmentInput(application))
		if err != nil || created || again.ID != first.ID {
			t.Fatalf("retry %d = %v, created %v, id %s", i, err, created, again.ID)
		}
	}
	resumed, err := svc.Resume(ctx, candidate, "candidate", first.ID)
	if err != nil || resumed.ID != first.ID || resumed.QuestionCount != 4 {
		t.Fatalf("Resume() = %v, %v", resumed, err)
	}
	if got := s.snapshot(first.ID); !slices.Equal(got, want) {
		t.Fatalf("snapshot changed from %v to %v", want, got)
	}
	if got := s.interviewsOfPosting(posting); got != 1 {
		t.Fatalf("%d interviews of the posting, want 1", got)
	}
	if got := s.questionRowsOfPosting(posting); got != 4 {
		t.Fatalf("%d stored questions, want 4: a retry never adds questions", got)
	}
}

// TestPracticeStartCoversTheCatalogAndRetryChargesOnce proves the coverage rule for a practice bank that
// carries rubrics (the whole metric catalog is active, since a practice session has no posting weights),
// and that retries and a resume keep the set, the question count and the coin balance.
func TestPracticeStartCoversTheCatalogAndRetryChargesOnce(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	m := s.catalogMetrics()
	var rubrics []*uuid.UUID
	for i := range 12 {
		r := s.rubric(m[i%len(m)], m[(i+1)%len(m)])
		rubrics = append(rubrics, &r)
	}
	candidate := s.user("candidate")
	s.wallet(candidate, 20)
	jdID := s.jobDescription()
	bank := s.bankOf(jdID, rubrics)
	profile := uuid.New()
	s.exec(`INSERT INTO public.role_profiles (id, jd_id, candidate_id, title, status)
		VALUES ($1, $2, $3, 'sampling', 'confirmed')`, profile, jdID, candidate)
	svc := newService(t, pool, 8)
	input := practiceInput(profile)
	input.IdempotencyKey = "sampling-retry"

	first, created, err := svc.Create(ctx, candidate, "candidate", input)
	if err != nil || !created {
		t.Fatalf("first start = %v, created %v", err, created)
	}
	snap := s.snapshot(first.ID)
	if len(snap) != 8 {
		t.Fatalf("snapshot has %d questions, want the limit 8", len(snap))
	}
	assertSampleOf(t, snap, bank)
	s.assertCovered(first.ID, m)
	charged := s.balance(candidate)
	if charged != 15 {
		t.Fatalf("balance = %d, want 15 after one start", charged)
	}

	for _, key := range []string{"sampling-retry", "another-key", ""} {
		again := input
		again.IdempotencyKey = key
		got, created, err := svc.Create(ctx, candidate, "candidate", again)
		if err != nil || created || got.ID != first.ID {
			t.Fatalf("retry with key %q = %v, created %v, id %s", key, err, created, got.ID)
		}
	}
	if _, err := svc.Resume(ctx, candidate, "candidate", first.ID); err != nil {
		t.Fatalf("Resume() = %v", err)
	}
	if got := s.snapshot(first.ID); !slices.Equal(got, snap) {
		t.Fatalf("snapshot changed from %v to %v", snap, got)
	}
	if got := s.balance(candidate); got != charged {
		t.Fatalf("balance = %d after retries, want %d", got, charged)
	}
	if got := len(s.coinRows(candidate)); got != 1 {
		t.Fatalf("%d ledger rows, want 1", got)
	}
	if got := s.questionRows(profile); got != 8 {
		t.Fatalf("%d stored questions, want 8", got)
	}
}

// TestRecruitmentStartRefusedWhenTheBankCannotCover covers the refusals: a metric nobody assesses, a limit
// too small for the cover, and weights that leave a metric uncovered by default. Each is the stable code and
// leaves no interview, no stored question, no consumed slot and no LLM budget reservation behind.
func TestRecruitmentStartRefusedWhenTheBankCannotCover(t *testing.T) {
	cases := []struct {
		name  string
		limit int32
		// bank lists, for each question, the catalog indexes of the metrics its rubric assesses; weights maps
		// a catalog index to its saved weight (nil means no saved weights: the whole catalog is active).
		bank    [][]int
		weights map[int]float64
	}{
		{name: "metric nobody assesses", limit: 8, bank: [][]int{{0}, {1}, {0, 1}, {0}}, weights: map[int]float64{0: 40, 1: 40, 2: 20}},
		{name: "limit smaller than the cover", limit: 2, bank: [][]int{{0}, {1}, {2}, {0}}, weights: map[int]float64{0: 40, 1: 40, 2: 20}},
		{name: "default weights make the whole catalog active", limit: 8, bank: [][]int{{0}, {1}, {0, 1}}},
		{name: "rubrics that only assess a weightless metric", limit: 8, bank: [][]int{{1}, {1}}, weights: map[int]float64{0: 100, 1: 0}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, pool := openPool(t)
			s := newSeed(t, ctx, pool)
			m := s.catalogMetrics()
			var rubrics []*uuid.UUID
			for _, metrics := range tc.bank {
				var ids []uuid.UUID
				for _, i := range metrics {
					ids = append(ids, m[i])
				}
				r := s.rubric(ids...)
				rubrics = append(rubrics, &r)
			}
			posting, _ := s.postingWithBank(2, rubrics)
			if tc.weights != nil {
				weights := map[uuid.UUID]float64{}
				for i, percentage := range tc.weights {
					weights[m[i]] = percentage
				}
				s.weigh(posting, weights)
			}
			svc := newService(t, pool, tc.limit)
			budget := llmbudget.New(llmbudget.NewPostgresStore(pool), 1)
			svc.SetLLMBudget(budget)
			candidate := s.user("candidate")
			application := s.eligibleApplication(posting, candidate)

			_, created, err := svc.Create(ctx, candidate, "candidate", recruitmentInput(application))
			if created || !apperror.IsCode(err, apperror.CodeQuestionBankCoverageInsufficient) {
				t.Fatalf("Create() = created %v, %v, want %s", created, err, apperror.CodeQuestionBankCoverageInsufficient)
			}
			if got := s.interviewsOfApplication(application); got != 0 {
				t.Fatalf("%d interviews written", got)
			}
			if got := s.interviewsOfPosting(posting); got != 0 {
				t.Fatalf("%d slots consumed, want 0", got)
			}
			if got := s.questionRowsOfPosting(posting); got != 0 {
				t.Fatalf("%d questions stored, want 0", got)
			}
			if got := s.applicationStatus(application); got != "interview_eligible" {
				t.Fatalf("application status = %s, want it unchanged", got)
			}
			// The reservation was given back: the one-call budget shows no call used.
			if got := s.count(`SELECT COALESCE(max(calls), 0) FROM public.llm_usage WHERE user_id = $1`, candidate); got != 0 {
				t.Fatalf("budget calls = %d after a refusal, want 0", got)
			}
		})
	}
}

// TestPracticeStartsOnABankThatCannotCoverAndChargesOnce proves the practice sampler never refuses for
// coverage (KAN-378): a Candidate cannot fix the bank, so a bank that assesses only two of the five catalog
// metrics still starts, covers as many metrics as it can, charges the practice price once, and a retry or a
// resume gives back the same session without charging again.
func TestPracticeStartsOnABankThatCannotCoverAndChargesOnce(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	m := s.catalogMetrics()
	// The bank has rubrics, but together they assess only two metrics of the catalog.
	rA, rB := s.rubric(m[0]), s.rubric(m[1])
	candidate := s.user("candidate")
	s.wallet(candidate, 20)
	jdID := s.jobDescription()
	bank := s.bankOf(jdID, rubricPtrs(rA, rB, rA, rB))
	profile := uuid.New()
	s.exec(`INSERT INTO public.role_profiles (id, jd_id, candidate_id, title, status)
		VALUES ($1, $2, $3, 'sampling', 'confirmed')`, profile, jdID, candidate)
	svc := newService(t, pool, 3)

	input := practiceInput(profile)
	input.IdempotencyKey = "best-effort"
	session, created, err := svc.Create(ctx, candidate, "candidate", input)
	if err != nil || !created {
		t.Fatalf("Create() = created %v, %v, want a practice session on the bank that cannot cover", created, err)
	}
	snap := s.snapshot(session.ID)
	if len(snap) != 3 {
		t.Fatalf("snapshot %v, want 3 questions (the limit)", snap)
	}
	assertSampleOf(t, snap, bank)
	s.assertCovered(session.ID, m[:2])
	if got := s.balance(candidate); got != 20-domain.PracticeInterviewCoins {
		t.Fatalf("balance = %d, want one practice price charged", got)
	}

	// A retry with the same key and a resume without a key both return the stored session for free.
	for _, key := range []string{"best-effort", ""} {
		again := practiceInput(profile)
		again.IdempotencyKey = key
		got, created, err := svc.Create(ctx, candidate, "candidate", again)
		if err != nil || created || got.ID != session.ID {
			t.Fatalf("Create(key %q) = %v created %v, %v; want the same session, not created", key, got.ID, created, err)
		}
		if !slices.Equal(s.snapshot(got.ID), snap) {
			t.Fatalf("snapshot changed on a retry or resume")
		}
	}
	if got := s.balance(candidate); got != 20-domain.PracticeInterviewCoins {
		t.Fatalf("balance = %d after retry and resume, want still one charge", got)
	}
	if got := len(s.coinRows(candidate)); got != 1 {
		t.Fatalf("%d ledger rows, want 1", got)
	}
}

// TestRubricLessBanksKeepTheirPlainRandomSample proves a bank without rubric data starts as it did before
// Barem, even when the posting weights metrics, and that a posting whose saved weights are all zero has no
// active metric and never fails coverage.
func TestRubricLessBanksKeepTheirPlainRandomSample(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	m := s.catalogMetrics()
	svc := newService(t, pool, 3)

	posting, bank := s.postingWithBank(2, make([]*uuid.UUID, 6))
	s.weigh(posting, map[uuid.UUID]float64{m[0]: 60, m[1]: 40})
	candidate := s.user("candidate")
	session, created, err := svc.Create(ctx, candidate, "candidate", recruitmentInput(s.eligibleApplication(posting, candidate)))
	if err != nil || !created {
		t.Fatalf("rubric-less start = %v, created %v", err, created)
	}
	if snap := s.snapshot(session.ID); len(snap) != 3 {
		t.Fatalf("snapshot %v, want 3 questions", snap)
	} else {
		assertSampleOf(t, snap, bank)
	}

	rA := s.rubric(m[0])
	zero, _ := s.postingWithBank(2, rubricPtrs(rA, rA, rA))
	s.weigh(zero, map[uuid.UUID]float64{m[0]: 0, m[1]: 0})
	other := s.user("candidate")
	if _, created, err := svc.Create(ctx, other, "candidate", recruitmentInput(s.eligibleApplication(zero, other))); err != nil || !created {
		t.Fatalf("all-zero weights start = %v, created %v: no active metric needs coverage", err, created)
	}
}

// TestMixedBankCoversWithItsRubricQuestions proves a bank where some questions have no rubric (or a rubric
// with no criteria) covers through the others, and the rubric-less ones only fill.
func TestMixedBankCoversWithItsRubricQuestions(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	m := s.catalogMetrics()
	rA, rB, empty := s.rubric(m[0]), s.rubric(m[1]), s.rubric()
	posting, _ := s.postingWithBank(8, []*uuid.UUID{nil, nil, &empty, &rA, nil, &rB, &empty, nil})
	s.weigh(posting, map[uuid.UUID]float64{m[0]: 50, m[1]: 50})
	svc := newService(t, pool, 3)
	for range 8 {
		candidate := s.user("candidate")
		session, _, err := svc.Create(ctx, candidate, "candidate", recruitmentInput(s.eligibleApplication(posting, candidate)))
		if err != nil {
			t.Fatalf("Create() = %v", err)
		}
		s.assertCovered(session.ID, m[:2])
	}
}

// TestConcurrentRecruitmentStartsStayCoveredAndQueueOnThePostingLock starts more sessions than the posting
// has slots at once. Exactly the funded slots are consumed, every created session covers every metric,
// and the others are told the posting is full.
func TestConcurrentRecruitmentStartsStayCoveredAndQueueOnThePostingLock(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	m := s.catalogMetrics()
	rA, rB, rC := s.rubric(m[0]), s.rubric(m[1]), s.rubric(m[2])
	posting, _ := s.postingWithBank(3, rubricPtrs(rA, rA, rA, rB, rB, rB, rC, rC, rC))
	s.weigh(posting, map[uuid.UUID]float64{m[0]: 40, m[1]: 40, m[2]: 20})
	svc := newService(t, pool, 4)
	const starts = 8
	applications := make([]uuid.UUID, starts)
	candidates := make([]uuid.UUID, starts)
	for i := range starts {
		candidates[i] = s.user("candidate")
		applications[i] = s.eligibleApplication(posting, candidates[i])
	}

	var wg sync.WaitGroup
	sessions := make([]domain.Session, starts)
	errs := make([]error, starts)
	for i := range starts {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sessions[i], _, errs[i] = svc.Create(ctx, candidates[i], "candidate", recruitmentInput(applications[i]))
		}()
	}
	wg.Wait()

	created, full := 0, 0
	for i := range starts {
		switch {
		case errs[i] == nil:
			created++
			s.assertCovered(sessions[i].ID, m[:3])
		case apperror.IsCode(errs[i], apperror.CodeInterviewCapacityFull):
			full++
		default:
			t.Fatalf("start %d: unexpected error %v", i, errs[i])
		}
	}
	if created != 3 || full != starts-3 {
		t.Fatalf("created %d, full %d, want 3 and %d", created, full, starts-3)
	}
	if got := s.interviewsOfPosting(posting); got != 3 {
		t.Fatalf("%d slots consumed, want 3", got)
	}
	if got := s.questionRowsOfPosting(posting); got != 12 {
		t.Fatalf("%d stored questions, want 3 sessions of 4", got)
	}
}

// TestSamplingUsesTheInjectedRandomSource proves the result depends only on the injected source: the same
// seed gives the same questions for two Candidates, and the default source does not.
func TestSamplingUsesTheInjectedRandomSource(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	m := s.catalogMetrics()
	rA, rB := s.rubric(m[0]), s.rubric(m[1])
	var rubrics []*uuid.UUID
	for i := range 20 {
		r := rA
		if i%2 == 1 {
			r = rB
		}
		rubrics = append(rubrics, &r)
	}
	posting, _ := s.postingWithBank(40, rubrics)
	s.weigh(posting, map[uuid.UUID]float64{m[0]: 50, m[1]: 50})

	pick := func(svc *service.Service) []uuid.UUID {
		candidate := s.user("candidate")
		session, _, err := svc.Create(ctx, candidate, "candidate", recruitmentInput(s.eligibleApplication(posting, candidate)))
		if err != nil {
			t.Fatal(err)
		}
		return s.snapshot(session.ID)
	}
	seeded := newSeededService(t, pool, 8, 11)
	if a, b := pick(seeded), pick(seeded); !slices.Equal(a, b) {
		t.Fatalf("the same seed gave %v then %v", a, b)
	}
	plain := newService(t, pool, 8)
	sets := map[string]bool{}
	for range 12 {
		sets[sampleKey(pick(plain))] = true
	}
	if len(sets) < 2 {
		t.Fatal("the default random source gave the same set 12 times")
	}
}

// TestTurnsMapToTheirAssignedCoreQuestion answers a session with several follow-ups on some core questions,
// a follow-up that was asked but never answered and a last core question with its own follow-up. Every stored
// turn must map to the interview_questions row of its core question, whatever the conversation position.
func TestTurnsMapToTheirAssignedCoreQuestion(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	repo := repository.NewRepository(pool)
	svc := newService(t, pool, 3)
	candidate := s.user("candidate")
	s.wallet(candidate, 10)
	profile, _ := s.confirmedProfile(candidate, 5)
	started, _, err := svc.Create(ctx, candidate, "candidate", practiceInput(profile))
	if err != nil {
		t.Fatal(err)
	}
	id := started.ID
	snap := s.snapshot(id)
	asked := time.Now().UTC().Truncate(time.Millisecond)
	followUp := func(position, questionNumber int32) domain.AskedQuestion {
		return domain.AskedQuestion{Position: position, QuestionNumber: questionNumber, Kind: domain.TurnFollowUp, Text: "Why?", AskedAt: asked}
	}
	step := func(name string, err error) {
		t.Helper()
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	}
	// Core 1, then two follow-ups on it.
	_, err = repo.AppendTurn(ctx, candidate, id, 1, "answer 1")
	step("core 1", err)
	_, err = repo.AppendFollowUp(ctx, candidate, id, followUp(2, 1), "because", 2)
	step("follow-up 1.1", err)
	_, err = repo.AppendFollowUp(ctx, candidate, id, followUp(3, 1), "also because", 2)
	step("follow-up 1.2", err)
	// Core 2 lands at position 4; a follow-up asked for it is never answered, so nothing is stored and
	// core 3 follows at position 5.
	_, err = repo.AppendTurn(ctx, candidate, id, 4, "answer 2")
	step("core 2", err)
	_, err = repo.AppendTurn(ctx, candidate, id, 5, "answer 3")
	step("core 3", err)
	_, err = repo.AppendFollowUp(ctx, candidate, id, followUp(6, 3), "last because", 2)
	step("follow-up 3.1", err)

	got, err := repo.TurnCoreQuestions(ctx, id)
	step("TurnCoreQuestions", err)
	want := []domain.TurnCoreQuestion{
		{Position: 1, QuestionNumber: 1, Kind: domain.TurnCore, CoreQuestionID: snap[0]},
		{Position: 2, QuestionNumber: 1, Kind: domain.TurnFollowUp, CoreQuestionID: snap[0]},
		{Position: 3, QuestionNumber: 1, Kind: domain.TurnFollowUp, CoreQuestionID: snap[0]},
		{Position: 4, QuestionNumber: 2, Kind: domain.TurnCore, CoreQuestionID: snap[1]},
		{Position: 5, QuestionNumber: 3, Kind: domain.TurnCore, CoreQuestionID: snap[2]},
		{Position: 6, QuestionNumber: 3, Kind: domain.TurnFollowUp, CoreQuestionID: snap[2]},
	}
	if !slices.Equal(got, want) {
		t.Fatalf("TurnCoreQuestions() = %+v, want %+v", got, want)
	}
	// The derived number agrees with the one the session itself reports for each turn.
	if numbers := s.questionNumbers(id); !slices.Equal(numbers, []int32{1, 1, 1, 2, 3, 3}) {
		t.Fatalf("question numbers = %v", numbers)
	}
	// The stored session never exposes the bank id of a turn: a practice Candidate's turns carry text only.
	session, err := svc.Get(ctx, candidate, "candidate", id)
	step("Get", err)
	if session.Turns == nil || len(*session.Turns) != 6 {
		t.Fatalf("session turns = %v", session.Turns)
	}
	// A session with no stored turn maps nothing, and another interview's turns are never mixed in.
	other, _, err := svc.Create(ctx, candidate, "candidate", domain.CreateInput{Type: domain.TypePractice, RoleProfileID: &profile})
	step("resume", err)
	if other.ID != id {
		t.Fatalf("a second start resumed session %s, want %s", other.ID, id)
	}
	empty, err := repo.TurnCoreQuestions(ctx, uuid.New())
	if err != nil || len(empty) != 0 {
		t.Fatalf("unknown session = %+v, %v", empty, err)
	}
}
