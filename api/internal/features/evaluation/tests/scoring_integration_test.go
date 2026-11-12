//go:build integration

// Rubric grading (KAN-334/335/336) and report visibility, run against a real PostgreSQL database that is
// migrated and disposable. The grader is a deterministic fake that goes through the real validator, so the
// storage, the transaction, the replay and failure paths and the report rules are proven without any LLM
// call. Recruitment sessions use a Recruiter's own rubric that is exactly the spec's worked example R01: C1 (2
// points) and C2 (3 points) on Technical Accuracy (metric 101), C3 (5 points) on Problem-Solving (metric 103).
// Practice sessions use the system template 201, which since migration 000034 also has a criterion on Answer
// Relevance (104) and one on Communication Clarity (105). Helpers are prefixed sc so they do not collide with
// the posting weights tests in this package.
package tests

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/cog-forge/rolecue/api/internal/features/evaluation/domain"
	"github.com/cog-forge/rolecue/api/internal/features/evaluation/repository"
	"github.com/cog-forge/rolecue/api/internal/features/evaluation/service"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	scSystem201     = uuid.MustParse("00000000-0000-4000-8000-000000000201")
	scMetricTA      = uuid.MustParse("00000000-0000-4000-8000-000000000101")
	scMetricDepth   = uuid.MustParse("00000000-0000-4000-8000-000000000102")
	scMetricPS      = uuid.MustParse("00000000-0000-4000-8000-000000000103")
	scExpected      = "EXPECTED-SECRET: index lookups avoid full scans; writes maintain the index."
	scWorkedExample = map[int32][]int32{1: {2, 2, 3}, 2: {1, 2, 2}}
)

func scOpenPool(t *testing.T) (context.Context, *pgxpool.Pool) {
	t.Helper()
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL is not set (load ../.env)")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	t.Cleanup(cancel)
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := pool.Ping(ctx); err != nil {
		t.Skipf("database not reachable: %v", err)
	}
	return ctx, pool
}

// scGrader is a deterministic grader: it gives each question the listed points (by question position, in
// criterion order), quotes the core answer, and validates its reply like the routed adapter does. err, when
// set, fails every call.
type scGrader struct {
	mu     sync.Mutex
	points map[int32][]int32
	err    error
	inputs []domain.QuestionGradeInput
}

func (g *scGrader) GradeQuestion(_ context.Context, in domain.QuestionGradeInput) (domain.QuestionGrade, error) {
	g.mu.Lock()
	g.inputs = append(g.inputs, in)
	g.mu.Unlock()
	if g.err != nil {
		return domain.QuestionGrade{}, g.err
	}
	out := domain.QuestionGradeCandidate{Feedback: fmt.Sprintf("feedback %d", in.QuestionPosition), Summary: fmt.Sprintf("summary %d.", in.QuestionPosition)}
	for i, c := range in.Criteria {
		// A criterion the test gives no points to earns 0 (the extra criteria of a system template).
		var points int32
		if i < len(g.points[in.QuestionPosition]) {
			points = g.points[in.QuestionPosition][i]
		}
		quote := ""
		if points > 0 {
			quote = in.Turns[len(in.Turns)-1].Answer
		}
		out.Criteria = append(out.Criteria, domain.CriterionGradeCandidate{CriterionID: c.ID, Points: points, Evidence: quote, Feedback: "why"})
	}
	return service.ValidateQuestionGrade(in, out)
}

func (g *scGrader) calls() int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return len(g.inputs)
}

// scSeed inserts evaluation rows and deletes them in foreign-key order when the test ends.
type scSeed struct {
	t        *testing.T
	ctx      context.Context
	pool     *pgxpool.Pool
	users    []string
	jds      []string
	postings []string
}

func newScSeed(t *testing.T, ctx context.Context, pool *pgxpool.Pool) *scSeed {
	sc := &scSeed{t: t, ctx: ctx, pool: pool}
	t.Cleanup(sc.cleanup)
	return sc
}

func (sc *scSeed) exec(sql string, args ...any) {
	sc.t.Helper()
	if _, err := sc.pool.Exec(sc.ctx, sql, args...); err != nil {
		sc.t.Fatalf("seed %q: %v", sql, err)
	}
}

func (sc *scSeed) user(role string) uuid.UUID {
	id := uuid.New()
	var company *string
	if role == "recruiter" {
		acme := "Acme"
		company = &acme
	}
	sc.exec(`INSERT INTO public.users (id, name, email, role, email_verified, company_name)
		VALUES ($1, $2, $3, $4, true, $5)`, id, "sc "+role, fmt.Sprintf("sc-%s@example.test", id), role, company)
	sc.users = append(sc.users, id.String())
	return id
}

func (sc *scSeed) jd() uuid.UUID {
	sc.t.Helper()
	var levelID uuid.UUID
	if err := sc.pool.QueryRow(sc.ctx, `SELECT id FROM public.levels ORDER BY id LIMIT 1`).Scan(&levelID); err != nil {
		sc.t.Fatalf("no seniority level seeded: %v", err)
	}
	id := uuid.New()
	sc.exec(`INSERT INTO public.job_descriptions (id, content_path, level_id) VALUES ($1, $2, $3)`,
		id, "job-postings/"+id.String()+".pdf", levelID)
	sc.jds = append(sc.jds, id.String())
	return id
}

func (sc *scSeed) roleProfile(candidateID, jdID uuid.UUID) uuid.UUID {
	id := uuid.New()
	sc.exec(`INSERT INTO public.role_profiles (id, jd_id, candidate_id, title, status)
		VALUES ($1, $2, $3, 'evaluation test', 'confirmed')`, id, jdID, candidateID)
	return id
}

func (sc *scSeed) posting(recruiterID, jdID uuid.UUID) uuid.UUID {
	id := uuid.New()
	sc.exec(`INSERT INTO public.job_postings (id, status, jd_id, recruiter_id, title)
		VALUES ($1, 'open'::public.job_posting_status, $2, $3, 'evaluation posting')`, id, jdID, recruiterID)
	sc.postings = append(sc.postings, id.String())
	return id
}

func (sc *scSeed) weight(postingID, metricID uuid.UUID, percentage float64) {
	sc.exec(`INSERT INTO public.metrics_percentage (metric_id, percentage, job_posting_id) VALUES ($1, $2, $3)`,
		metricID, percentage, postingID)
}

func (sc *scSeed) application(postingID, candidateID uuid.UUID, status string) uuid.UUID {
	id := uuid.New()
	sc.exec(`INSERT INTO public.applications (id, job_posting_id, candidate_id, status, cv_path)
		VALUES ($1, $2, $3, $4::public.application_status, 'cvs/evaluation.pdf')`, id, postingID, candidateID, status)
	return id
}

// workedRubric adds a Recruiter's own rubric that is the spec's R01 (C1 2 points and C2 3 points on Technical
// Accuracy, C3 5 points on Problem-Solving) and returns it with its first criterion.
func (sc *scSeed) workedRubric(owner uuid.UUID) (rubric, c1 uuid.UUID) {
	rubric, c1 = uuid.New(), uuid.New()
	sc.exec(`INSERT INTO public.rubrics (id, name, question_type, scope, owner_id) VALUES ($1, 'R01 worked example', 'concept_explanation', 'recruiter', $2)`, rubric, owner)
	sc.exec(`INSERT INTO public.rubric_criteria (id, rubric_id, metric_id, name, max_points, scoring_guide, position) VALUES
		($1, $2, $3, 'Mechanism', 2, '0 = wrong; 1 = partly; 2 = correct.', 1),
		($4, $2, $3, 'Limits', 3, '0 = none; 1 = one; 2 = most; 3 = all.', 2),
		($5, $2, $6, 'Recommendation', 5, '0 = none; 1 = bare; 2 = weak; 3 = sound; 4 = justified; 5 = full.', 3)`,
		c1, rubric, scMetricTA, uuid.New(), uuid.New(), scMetricPS)
	return rubric, c1
}

// question adds a core question of a bank; rubric nil is a question without grading configuration.
func (sc *scSeed) question(jdID uuid.UUID, content string, rubric *uuid.UUID) uuid.UUID {
	id := uuid.New()
	var expected *string
	if rubric != nil {
		expected = &scExpected
	}
	sc.exec(`INSERT INTO public.core_questions (id, jd_id, content, rubric_id, expected_answer) VALUES ($1, $2, $3, $4, $5)`,
		id, jdID, content, rubric, expected)
	return id
}

// interview adds a completed session (a recruitment one needs its Application) with assigned questions.
func (sc *scSeed) interview(roleID uuid.UUID, interviewType string, applicationID *uuid.UUID, questions ...uuid.UUID) uuid.UUID {
	id := uuid.New()
	sc.exec(`INSERT INTO public.interviews (id, role_profile_id, type, application_id, completed_at)
		VALUES ($1, $2, $3::public.interview_type, $4, now())`, id, roleID, interviewType, applicationID)
	for i, q := range questions {
		sc.exec(`INSERT INTO public.interview_questions (interview_id, position, core_question_id) VALUES ($1, $2, $3)`, id, i+1, q)
	}
	return id
}

func (sc *scSeed) turn(interviewID uuid.UUID, position int32, kind, question string, answer *string) {
	sc.exec(`INSERT INTO public.conversation_turns (interview_id, position, kind, question, answer) VALUES ($1, $2, $3, $4, $5)`,
		interviewID, position, kind, question, answer)
}

// workedExample is a session of two questions graded with the given rubric; the first has a follow-up.
func (sc *scSeed) workedExample(roleID uuid.UUID, interviewType string, applicationID *uuid.UUID, jdID, rubric uuid.UUID) uuid.UUID {
	iv := sc.interview(roleID, interviewType, applicationID,
		sc.question(jdID, "Explain SQL indexes.", &rubric), sc.question(jdID, "Explain caching.", &rubric))
	sc.turn(iv, 1, "core", "Explain SQL indexes.", scStr("A B-tree index avoids scanning the whole table."))
	sc.turn(iv, 2, "follow_up", "What does it cost?", scStr("Every write must also update the index."))
	sc.turn(iv, 3, "core", "Explain caching.", scStr("A read-through cache with a TTL keeps hot rows in memory."))
	return iv
}

type scRow struct {
	status  string
	score   *float64
	outcome *string
	reason  *string
	summary *string
}

func (sc *scSeed) readInterview(id uuid.UUID) scRow {
	sc.t.Helper()
	var row scRow
	if err := sc.pool.QueryRow(sc.ctx, `SELECT evaluation_status, score::float8, score_outcome, score_withheld_reason, feedback
		FROM public.interviews WHERE id = $1`, id).Scan(&row.status, &row.score, &row.outcome, &row.reason, &row.summary); err != nil {
		sc.t.Fatal(err)
	}
	return row
}

func (sc *scSeed) count(sql string, args ...any) int {
	sc.t.Helper()
	var n int
	if err := sc.pool.QueryRow(sc.ctx, sql, args...).Scan(&n); err != nil {
		sc.t.Fatal(err)
	}
	return n
}

func (sc *scSeed) cleanup() {
	ctx := context.Background()
	byCandidate := `(SELECT i.id FROM public.interviews i
		JOIN public.role_profiles rp ON rp.id = i.role_profile_id
		WHERE rp.candidate_id::text = ANY($1::text[]))`
	steps := []struct {
		sql  string
		args []any
	}{
		{`DELETE FROM public.interview_criterion_scores WHERE interview_id IN ` + byCandidate, []any{sc.users}},
		{`DELETE FROM public.score_details WHERE interview_id IN ` + byCandidate, []any{sc.users}},
		{`DELETE FROM public.interview_questions WHERE interview_id IN ` + byCandidate, []any{sc.users}},
		{`DELETE FROM public.conversation_turns WHERE interview_id IN ` + byCandidate, []any{sc.users}},
		{`DELETE FROM public.interviews WHERE role_profile_id IN
			(SELECT id FROM public.role_profiles WHERE candidate_id::text = ANY($1::text[]))`, []any{sc.users}},
		{`DELETE FROM public.applications WHERE candidate_id::text = ANY($1::text[])`, []any{sc.users}},
		{`DELETE FROM public.metrics_percentage WHERE job_posting_id::text = ANY($1::text[])`, []any{sc.postings}},
		{`DELETE FROM public.job_postings WHERE id::text = ANY($1::text[])`, []any{sc.postings}},
		{`DELETE FROM public.core_questions WHERE jd_id::text = ANY($1::text[])`, []any{sc.jds}},
		{`DELETE FROM public.rubrics WHERE owner_id::text = ANY($1::text[])`, []any{sc.users}},
		{`DELETE FROM public.role_profiles WHERE candidate_id::text = ANY($1::text[])`, []any{sc.users}},
		{`DELETE FROM public.job_descriptions WHERE id::text = ANY($1::text[])`, []any{sc.jds}},
		{`DELETE FROM public.wallets WHERE user_id::text = ANY($1::text[])`, []any{sc.users}},
		{`DELETE FROM public.users WHERE id::text = ANY($1::text[])`, []any{sc.users}},
	}
	for _, step := range steps {
		if _, err := sc.pool.Exec(ctx, step.sql, step.args...); err != nil {
			sc.t.Errorf("cleanup %q: %v", step.sql, err)
		}
	}
}

func scNewScoring(t *testing.T, repo *repository.Repository, grader service.Grader) *service.Scoring {
	t.Helper()
	scoring, err := service.NewScoring(repo, grader)
	if err != nil {
		t.Fatal(err)
	}
	return scoring
}

func scAssertCode(t *testing.T, err error, want apperror.Code) {
	t.Helper()
	var appErr *apperror.AppError
	if !errors.As(err, &appErr) || appErr.Code != want {
		t.Fatalf("error = %v, want code %s", err, want)
	}
}

func scStr(value string) *string { return &value }

// recruitmentWorkedExample seeds a recruitment session of a posting weighted 60/40 on the two metrics R01
// assesses, graded with the Recruiter's own R01.
func recruitmentWorkedExample(sc *scSeed) (iv, candidate, recruiter, app uuid.UUID) {
	iv, candidate, recruiter, app, _ = recruitmentWorkedExampleWithCriterion(sc)
	return iv, candidate, recruiter, app
}

func recruitmentWorkedExampleWithCriterion(sc *scSeed) (iv, candidate, recruiter, app, c1 uuid.UUID) {
	candidate, recruiter = sc.user("candidate"), sc.user("recruiter")
	var rubric uuid.UUID
	rubric, c1 = sc.workedRubric(recruiter)
	jd := sc.jd()
	posting := sc.posting(recruiter, jd)
	sc.weight(posting, scMetricTA, 60)
	sc.weight(posting, scMetricPS, 40)
	app = sc.application(posting, candidate, "interviewed")
	iv = sc.workedExample(sc.roleProfile(candidate, sc.jd()), "recruitment", &app, jd, rubric)
	return iv, candidate, recruiter, app, c1
}

// --- Grading and storage (KAN-334, KAN-335) ---

func TestEvaluateStoresTheWorkedExampleAtEveryLevel(t *testing.T) {
	ctx, pool := scOpenPool(t)
	sc := newScSeed(t, ctx, pool)
	repo := repository.NewRepository(pool)
	iv, _, _, app := recruitmentWorkedExample(sc)
	grader := &scGrader{points: scWorkedExample}

	if err := scNewScoring(t, repo, grader).Evaluate(ctx, iv); err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}
	if grader.calls() != 2 {
		t.Fatalf("grader calls = %d, want one per core question", grader.calls())
	}
	for _, in := range grader.inputs {
		if in.ExpectedAnswer != scExpected || len(in.Criteria) != 3 || (in.QuestionPosition == 1) != (len(in.Turns) == 2) {
			t.Fatalf("input %d = %d turns, %d criteria; want its own turns, the rubric and the expected answer", in.QuestionPosition, len(in.Turns), len(in.Criteria))
		}
	}
	row := sc.readInterview(iv)
	if row.status != "completed" || row.score == nil || *row.score != 62 || row.outcome == nil || *row.outcome != "complete" || row.reason != nil {
		t.Fatalf("interview = %+v, want completed, overall 62.00, complete", row)
	}
	if row.summary == nil || *row.summary != "summary 1. summary 2." {
		t.Fatalf("summary = %v", row.summary)
	}
	if n := sc.count(`SELECT count(*) FROM public.score_details WHERE interview_id = $1 AND
		((metric_id = $2 AND score = 70.00 AND weight = 60 AND question_count = 2) OR (metric_id = $3 AND score = 50.00 AND weight = 40))`,
		iv, scMetricTA, scMetricPS); n != 2 {
		t.Fatalf("session metrics matching 70 (60) and 50 (40) = %d, want 2", n)
	}
	if n := sc.count(`SELECT count(*) FROM public.interview_questions WHERE interview_id = $1 AND
		((position = 1 AND score = 70.00) OR (position = 2 AND score = 50.00))`, iv); n != 2 {
		t.Fatalf("question totals matching 70 and 50 = %d, want 2", n)
	}
	if n := sc.count(`SELECT count(*) FROM public.interview_criterion_scores WHERE interview_id = $1`, iv); n != 6 {
		t.Fatalf("criterion results = %d, want 6", n)
	}
	// Q01's quotes come from its follow-up (position 2), never from another question.
	if n := sc.count(`SELECT count(*) FROM public.interview_criterion_scores WHERE interview_id = $1 AND question_position = 1 AND evidence_turn_position = 2`, iv); n != 3 {
		t.Fatalf("Q01 criteria quoting its follow-up = %d, want 3", n)
	}
	if n := sc.count(`SELECT count(*) FROM public.conversation_turns WHERE interview_id = $1 AND feedback IS NOT NULL AND kind = 'core'`, iv); n != 2 {
		t.Fatalf("core turns with feedback = %d, want 2", n)
	}
	var status string
	if err := pool.QueryRow(ctx, `SELECT status::text FROM public.applications WHERE id = $1`, app).Scan(&status); err != nil || status != "interviewed" {
		t.Fatalf("application status = %q, %v; evaluation must not change it", status, err)
	}

	// A replay of a completed evaluation calls no model and rewrites nothing.
	again := &scGrader{points: map[int32][]int32{1: {0, 0, 0}, 2: {0, 0, 0}}}
	if err := scNewScoring(t, repo, again).Evaluate(ctx, iv); err != nil || again.calls() != 0 {
		t.Fatalf("replay: err = %v, calls = %d; want a no-op", err, again.calls())
	}
	if row := sc.readInterview(iv); *row.score != 62 {
		t.Fatalf("replay changed the overall to %v", *row.score)
	}
}

func TestEvaluatePracticeWithheldWhenADefaultMetricIsNotCovered(t *testing.T) {
	ctx, pool := scOpenPool(t)
	sc := newScSeed(t, ctx, pool)
	repo := repository.NewRepository(pool)
	catalog, err := repo.ListMetrics(ctx)
	if err != nil || len(catalog) < 3 {
		t.Fatalf("metrics catalog = %d, %v; want the seeded five", len(catalog), err)
	}
	jd := sc.jd()
	iv := sc.workedExample(sc.roleProfile(sc.user("candidate"), jd), "practice", nil, jd, scSystem201)
	if err := scNewScoring(t, repo, &scGrader{points: scWorkedExample}).Evaluate(ctx, iv); err != nil {
		t.Fatal(err)
	}
	row := sc.readInterview(iv)
	if row.status != "completed" || row.score != nil || *row.outcome != "withheld" || *row.reason != "missing_metric_coverage" {
		t.Fatalf("interview = %+v, want completed with the overall withheld for missing coverage", row)
	}
	// Template 201 assesses Technical Accuracy, Problem-Solving, Answer Relevance and Clarity, never Depth.
	if n := sc.count(`SELECT count(*) FROM public.score_details WHERE interview_id = $1 AND score IS NULL AND weight = 1`, iv); n != len(catalog)-4 {
		t.Fatalf("uncovered metrics listed = %d, want %d", n, len(catalog)-4)
	}
	if n := sc.count(`SELECT count(*) FROM public.score_details WHERE interview_id = $1 AND metric_id = $2 AND score IS NULL`, iv, scMetricDepth); n != 1 {
		t.Fatalf("Depth listed without a score = %d, want 1", n)
	}
}

func TestEvaluateWithoutAnswersCompletesWithheldWithoutAModelCall(t *testing.T) {
	ctx, pool := scOpenPool(t)
	sc := newScSeed(t, ctx, pool)
	repo := repository.NewRepository(pool)
	jd := sc.jd()
	iv := sc.interview(sc.roleProfile(sc.user("candidate"), jd), "practice", nil, sc.question(jd, "Q?", &scSystem201))
	sc.turn(iv, 1, "core", "Q?", nil)
	grader := &scGrader{points: scWorkedExample}
	if err := scNewScoring(t, repo, grader).Evaluate(ctx, iv); err != nil {
		t.Fatal(err)
	}
	if row := sc.readInterview(iv); grader.calls() != 0 || row.status != "completed" || *row.reason != "no_answers" || row.score != nil {
		t.Fatalf("calls = %d, interview = %+v; want completed, withheld for no answers", grader.calls(), row)
	}
}

func TestEvaluateFailureWritesNothingAndARetryCompletes(t *testing.T) {
	ctx, pool := scOpenPool(t)
	sc := newScSeed(t, ctx, pool)
	repo := repository.NewRepository(pool)
	iv, _, _, _ := recruitmentWorkedExample(sc)

	err := scNewScoring(t, repo, &scGrader{err: errors.New("provider down")}).Evaluate(ctx, iv)
	scAssertCode(t, err, apperror.CodeInternal)
	if row := sc.readInterview(iv); row.status != "failed" || row.score != nil || row.outcome != nil {
		t.Fatalf("interview = %+v, want failed with nothing stored", row)
	}
	for _, table := range []string{"interview_criterion_scores", "score_details"} {
		if n := sc.count(`SELECT count(*) FROM public.`+table+` WHERE interview_id = $1`, iv); n != 0 {
			t.Fatalf("%s rows after a failure = %d, want 0", table, n)
		}
	}
	if n := sc.count(`SELECT count(*) FROM public.interview_questions WHERE interview_id = $1 AND score IS NOT NULL`, iv); n != 0 {
		t.Fatalf("question totals after a failure = %d, want 0", n)
	}

	// The claim-and-retry path runs Evaluate again; it grades from scratch and completes.
	if err := scNewScoring(t, repo, &scGrader{points: scWorkedExample}).Evaluate(ctx, iv); err != nil {
		t.Fatal(err)
	}
	if row := sc.readInterview(iv); row.status != "completed" || *row.score != 62 {
		t.Fatalf("retried interview = %+v, want completed with 62", row)
	}
}

func TestSaveRollsBackEverythingOnAnError(t *testing.T) {
	ctx, pool := scOpenPool(t)
	sc := newScSeed(t, ctx, pool)
	repo := repository.NewRepository(pool)
	iv, _, _, _, scCriterionC1 := recruitmentWorkedExampleWithCriterion(sc)
	overall := 62.0
	ta := 70.0
	evaluation := domain.Evaluation{
		Outcome: domain.RubricComplete, Overall: &overall,
		Metrics: []domain.MetricResult{{MetricID: scMetricTA, Score: &ta, Weight: 60, Questions: 1}},
		Questions: []domain.QuestionResult{{Position: 1, Total: 70, Criteria: []domain.CriterionGrade{
			{CriterionID: scCriterionC1, Points: 0},
			{CriterionID: uuid.New(), Points: 1, Evidence: "x", EvidencePosition: ptrInt32(1)}, // unknown criterion: FK violation
		}}},
	}
	if err := repo.SaveInterviewEvaluation(ctx, iv, evaluation); err == nil {
		t.Fatal("a save with an unknown criterion succeeded")
	}
	if row := sc.readInterview(iv); row.status != "pending" || row.score != nil {
		t.Fatalf("interview = %+v, want untouched", row)
	}
	if n := sc.count(`SELECT count(*) FROM public.interview_criterion_scores WHERE interview_id = $1`, iv) +
		sc.count(`SELECT count(*) FROM public.score_details WHERE interview_id = $1`, iv) +
		sc.count(`SELECT count(*) FROM public.interview_questions WHERE interview_id = $1 AND score IS NOT NULL`, iv); n != 0 {
		t.Fatalf("partial rows after a rolled back save = %d, want 0", n)
	}
	// The database refuses inconsistent results on its own too: points without evidence.
	evaluation.Questions[0].Criteria = []domain.CriterionGrade{{CriterionID: scCriterionC1, Points: 2}}
	if err := repo.SaveInterviewEvaluation(ctx, iv, evaluation); err == nil {
		t.Fatal("points without evidence were stored")
	}
	// A completed evaluation is never rewritten by a later save.
	evaluation.Questions[0].Criteria = []domain.CriterionGrade{{CriterionID: scCriterionC1, Points: 0}}
	if err := repo.SaveInterviewEvaluation(ctx, iv, evaluation); err != nil {
		t.Fatal(err)
	}
	other := 10.0
	evaluation.Overall = &other
	if err := repo.SaveInterviewEvaluation(ctx, iv, evaluation); err != nil {
		t.Fatal(err)
	}
	if row := sc.readInterview(iv); *row.score != 62 {
		t.Fatalf("overall = %v after a second save, want the first result kept", *row.score)
	}
}

func ptrInt32(v int32) *int32 { return &v }

func TestEvaluateNeedsACompletedInterview(t *testing.T) {
	ctx, pool := scOpenPool(t)
	sc := newScSeed(t, ctx, pool)
	repo := repository.NewRepository(pool)
	iv, _, _, _ := recruitmentWorkedExample(sc)
	sc.exec(`UPDATE public.interviews SET completed_at = NULL WHERE id = $1`, iv)
	grader := &scGrader{points: scWorkedExample}
	scAssertCode(t, scNewScoring(t, repo, grader).Evaluate(ctx, iv), apperror.CodeInterviewNotCompleted)
	if grader.calls() != 0 || sc.readInterview(iv).status != "pending" {
		t.Fatal("an unfinished interview was graded or marked")
	}
	scAssertCode(t, scNewScoring(t, repo, grader).Evaluate(ctx, uuid.New()), apperror.CodeInterviewNotFound)
}

// --- Report (KAN-336) ---

func TestRecruitmentReportExplainsTheOverallToItsRecruiterOnly(t *testing.T) {
	ctx, pool := scOpenPool(t)
	sc := newScSeed(t, ctx, pool)
	repo := repository.NewRepository(pool)
	iv, candidate, recruiter, _ := recruitmentWorkedExample(sc)
	scoring := scNewScoring(t, repo, &scGrader{points: scWorkedExample})

	pending, err := scoring.Report(ctx, recruiter, "recruiter", iv)
	if err != nil || pending.Status != "pending" || pending.Scoring != nil || pending.GradedQuestions != nil {
		t.Fatalf("report before evaluation = %+v, %v; want pending without scores", pending, err)
	}
	if err := scoring.Evaluate(ctx, iv); err != nil {
		t.Fatal(err)
	}
	full, err := scoring.Report(ctx, recruiter, "recruiter", iv)
	if err != nil {
		t.Fatal(err)
	}
	if full.OverallScore == nil || *full.OverallScore != 62 || full.Scoring == nil || *full.Scoring.Outcome != "complete" ||
		full.Scoring.WeightTotal != 100 || full.Scoring.GradedQuestions != 2 || len(full.Breakdown) != 2 || len(full.GradedQuestions) != 2 {
		t.Fatalf("recruiter report = %+v, want the full explanation of 62", full)
	}
	q1 := full.GradedQuestions[0]
	if q1.Total == nil || *q1.Total != 70 || q1.Earned != 7 || q1.Max != 10 || len(q1.Criteria) != 3 || q1.Criteria[1].Points != 2 ||
		q1.Criteria[1].MaxPoints != 3 || q1.Criteria[1].Evidence == nil || len(q1.Metrics) != 2 || q1.RubricName == nil {
		t.Fatalf("question 1 = %+v, want 7/10 with criterion 2/3 and its quote", q1)
	}

	limited, err := scoring.Report(ctx, candidate, "candidate", iv)
	if err != nil {
		t.Fatal(err)
	}
	if limited.OverallScore != nil || limited.Scoring != nil || limited.Breakdown != nil || limited.GradedQuestions != nil || limited.Questions != nil {
		t.Fatalf("candidate report = %+v, want the status only", limited)
	}
	for name, viewer := range map[string]struct {
		id   uuid.UUID
		role string
	}{
		"other recruiter": {sc.user("recruiter"), "recruiter"},
		"other candidate": {sc.user("candidate"), "candidate"},
		"admin":           {recruiter, "admin"},
	} {
		_, err := scoring.Report(ctx, viewer.id, viewer.role, iv)
		if err == nil {
			t.Fatalf("%s read a recruitment report", name)
		}
		scAssertCode(t, err, apperror.CodeInterviewNotFound)
	}
}

func TestPracticeReportShowsWithheldAndUngradedQuestionsToItsOwner(t *testing.T) {
	ctx, pool := scOpenPool(t)
	sc := newScSeed(t, ctx, pool)
	repo := repository.NewRepository(pool)
	owner := sc.user("candidate")
	jd := sc.jd()
	iv := sc.interview(sc.roleProfile(owner, jd), "practice", nil,
		sc.question(jd, "Explain SQL indexes.", &scSystem201), sc.question(jd, "Legacy question?", nil), sc.question(jd, "Never asked?", &scSystem201))
	sc.turn(iv, 1, "core", "Explain SQL indexes.", scStr("A B-tree index avoids scanning the whole table."))
	sc.turn(iv, 2, "core", "Legacy question?", scStr("Some answer."))
	scoring := scNewScoring(t, repo, &scGrader{points: scWorkedExample})
	if err := scoring.Evaluate(ctx, iv); err != nil {
		t.Fatal(err)
	}
	report, err := scoring.Report(ctx, owner, "candidate", iv)
	if err != nil {
		t.Fatal(err)
	}
	if report.OverallScore != nil || report.Scoring == nil || *report.Scoring.Outcome != "withheld" || report.Scoring.AssignedQuestions != 3 || report.Scoring.GradedQuestions != 1 {
		t.Fatalf("report = %+v, want withheld with 1 of 3 graded", report)
	}
	statuses := []string{report.GradedQuestions[0].Status, report.GradedQuestions[1].Status, report.GradedQuestions[2].Status}
	if statuses[0] != "graded" || statuses[1] != "not_gradable" || statuses[2] != "not_answered" {
		t.Fatalf("statuses = %v, want graded, not_gradable, not_answered", statuses)
	}
	if report.GradedQuestions[0].Criteria[0].Name == "" {
		t.Fatal("a system rubric's criterion name must be visible to the practice Candidate")
	}
	for _, viewer := range []struct {
		id   uuid.UUID
		role string
	}{{sc.user("candidate"), "candidate"}, {sc.user("recruiter"), "recruiter"}, {owner, "admin"}} {
		_, err := scoring.Report(ctx, viewer.id, viewer.role, iv)
		scAssertCode(t, err, apperror.CodeInterviewNotFound)
	}
}
