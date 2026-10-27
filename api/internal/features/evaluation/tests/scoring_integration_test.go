//go:build integration

// Interview evaluation (KAN-61, KAN-62) and report visibility, run against a real PostgreSQL database
// that is migrated and disposable. The Scorer is a deterministic fake, so the weighting, the
// idempotency, the failure path and the report rules are proven without any LLM call. Helpers here are
// prefixed sc so they do not collide with the posting weights tests in this package.
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

// scScorer is a deterministic Scorer. It returns output, or err, and records every call.
type scScorer struct {
	mu     sync.Mutex
	calls  int
	inputs []domain.ScoreInput
	output domain.ScoreOutput
	err    error
}

func (f *scScorer) Score(_ context.Context, in domain.ScoreInput) (domain.ScoreOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	f.inputs = append(f.inputs, in)
	if f.err != nil {
		return domain.ScoreOutput{}, f.err
	}
	return f.output, nil
}

func (f *scScorer) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

// scSeed inserts evaluation rows. Every id it creates is recorded, and the cleanup deletes them
// in foreign-key order: score_details, conversation_turns, interview_questions, interviews,
// applications, metrics_percentage, job_postings, core_questions, role_profiles, job_descriptions,
// wallets, users.
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

// roleProfile adds a confirmed Role Profile. Each Role Profile needs its own Job Description.
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

// interview adds a session. A recruitment session needs its Application; completed sets completed_at.
func (sc *scSeed) interview(roleID uuid.UUID, interviewType string, applicationID *uuid.UUID, completed bool) uuid.UUID {
	id := uuid.New()
	var appArg *string
	if applicationID != nil {
		value := applicationID.String()
		appArg = &value
	}
	sc.exec(`INSERT INTO public.interviews (id, role_profile_id, type, application_id, completed_at)
		VALUES ($1, $2, $3::public.interview_type, $4::uuid,
		        CASE WHEN $5::boolean THEN now() ELSE NULL END)`, id, roleID, interviewType, appArg, completed)
	return id
}

// turn stores one question with its answer. A nil answer is an unanswered question.
func (sc *scSeed) turn(interviewID uuid.UUID, position int32, question string, answer *string) {
	sc.exec(`INSERT INTO public.conversation_turns (interview_id, position, question, answer) VALUES ($1, $2, $3, $4)`,
		interviewID, position, question, answer)
}

// scRow is the evaluation state of one interview as stored.
type scRow struct {
	status   string
	score    *int32
	feedback *string
}

func (sc *scSeed) readInterview(id uuid.UUID) scRow {
	sc.t.Helper()
	var row scRow
	if err := sc.pool.QueryRow(sc.ctx,
		`SELECT evaluation_status, score, feedback FROM public.interviews WHERE id = $1`, id).
		Scan(&row.status, &row.score, &row.feedback); err != nil {
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

func (sc *scSeed) feedbackAt(interviewID uuid.UUID, position int32) *string {
	sc.t.Helper()
	var feedback *string
	if err := sc.pool.QueryRow(sc.ctx,
		`SELECT feedback FROM public.conversation_turns WHERE interview_id = $1 AND position = $2`,
		interviewID, position).Scan(&feedback); err != nil {
		sc.t.Fatal(err)
	}
	return feedback
}

func (sc *scSeed) applicationStatus(applicationID uuid.UUID) string {
	sc.t.Helper()
	var status string
	if err := sc.pool.QueryRow(sc.ctx,
		`SELECT status::text FROM public.applications WHERE id = $1`, applicationID).Scan(&status); err != nil {
		sc.t.Fatal(err)
	}
	return status
}

// cleanup deletes every row the test created, children before parents.
func (sc *scSeed) cleanup() {
	ctx := context.Background()
	byCandidate := `(SELECT i.id FROM public.interviews i
		JOIN public.role_profiles rp ON rp.id = i.role_profile_id
		WHERE rp.candidate_id::text = ANY($1::text[]))`
	steps := []struct {
		sql  string
		args []any
	}{
		{`DELETE FROM public.score_details WHERE interview_id IN ` + byCandidate, []any{sc.users}},
		{`DELETE FROM public.conversation_turns WHERE interview_id IN ` + byCandidate, []any{sc.users}},
		{`DELETE FROM public.interview_questions WHERE interview_id IN ` + byCandidate, []any{sc.users}},
		{`DELETE FROM public.interviews WHERE role_profile_id IN
			(SELECT id FROM public.role_profiles WHERE candidate_id::text = ANY($1::text[]))`, []any{sc.users}},
		{`DELETE FROM public.applications WHERE candidate_id::text = ANY($1::text[])`, []any{sc.users}},
		{`DELETE FROM public.metrics_percentage WHERE job_posting_id::text = ANY($1::text[])`, []any{sc.postings}},
		{`DELETE FROM public.job_postings WHERE id::text = ANY($1::text[])`, []any{sc.postings}},
		{`DELETE FROM public.core_questions WHERE jd_id::text = ANY($1::text[])`, []any{sc.jds}},
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

// scCatalog returns the seeded metrics in the order the catalog lists them.
func scCatalog(t *testing.T, ctx context.Context, repo *repository.Repository) []domain.Metric {
	t.Helper()
	catalog, err := repo.ListMetrics(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog) < 2 {
		t.Fatalf("metrics catalog has %d entries, want at least 2 (migration 000011)", len(catalog))
	}
	return catalog
}

// scEach scores every metric of the catalog the same.
func scEach(metrics []domain.Metric, score int32) []domain.MetricScore {
	out := make([]domain.MetricScore, 0, len(metrics))
	for _, metric := range metrics {
		out = append(out, domain.MetricScore{MetricID: metric.ID, Score: score})
	}
	return out
}

func scNewScoring(t *testing.T, repo *repository.Repository, scorer service.Scorer) *service.Scoring {
	t.Helper()
	scoring, err := service.NewScoring(repo, scorer)
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

// --- Evaluation (KAN-61) ---

func TestEvaluatePracticeWeightsEveryMetricEqually(t *testing.T) {
	ctx, pool := scOpenPool(t)
	sc := newScSeed(t, ctx, pool)
	repo := repository.NewRepository(pool)
	catalog := scCatalog(t, ctx, repo)
	role := sc.roleProfile(sc.user("candidate"), sc.jd())
	iv := sc.interview(role, "practice", nil, true)
	sc.turn(iv, 1, "Q1", scStr("A1"))
	sc.turn(iv, 2, "Q2", scStr("A2"))
	sc.turn(iv, 3, "Q3", nil)

	scorer := &scScorer{output: domain.ScoreOutput{
		Scores:    scEach(catalog, 60),
		Summary:   "solid answers",
		Questions: []domain.QuestionFeedback{{Position: 1, Feedback: "fb1"}, {Position: 2, Feedback: "fb2"}},
	}}
	scoring := scNewScoring(t, repo, scorer)
	if err := scoring.Evaluate(ctx, iv); err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}
	if scorer.callCount() != 1 {
		t.Fatalf("scorer calls = %d, want 1", scorer.callCount())
	}
	in := scorer.inputs[0]
	if len(in.Turns) != 2 || len(in.Metrics) != len(catalog) {
		t.Fatalf("scorer input has %d answered turns and %d metrics, want 2 and %d (unanswered Q3 left out)",
			len(in.Turns), len(in.Metrics), len(catalog))
	}
	for _, metric := range in.Metrics {
		if metric.Weight != 1 {
			t.Fatalf("practice metric %s weight = %v, want the equal weight 1", metric.Name, metric.Weight)
		}
	}

	row := sc.readInterview(iv)
	if row.status != "completed" || row.score == nil || *row.score != 60 || row.feedback == nil || *row.feedback != "solid answers" {
		t.Fatalf("interview = %+v, want completed with score 60 and the summary", row)
	}
	if n := sc.count(`SELECT count(*) FROM public.score_details WHERE interview_id = $1 AND score = 60`, iv); n != len(catalog) {
		t.Fatalf("score_details rows = %d, want one per metric (%d)", n, len(catalog))
	}
	if f := sc.feedbackAt(iv, 1); f == nil || *f != "fb1" {
		t.Fatalf("feedback at 1 = %v, want fb1", f)
	}
	if f := sc.feedbackAt(iv, 2); f == nil || *f != "fb2" {
		t.Fatalf("feedback at 2 = %v, want fb2", f)
	}
	if f := sc.feedbackAt(iv, 3); f != nil {
		t.Fatalf("feedback at the unanswered position 3 = %q, want none", *f)
	}

	if err := scoring.Evaluate(ctx, iv); err != nil {
		t.Fatalf("second Evaluate() error = %v, want a no-op", err)
	}
	if scorer.callCount() != 1 {
		t.Fatalf("scorer calls = %d after a second Evaluate, want still 1", scorer.callCount())
	}
}

func TestEvaluateRecruitmentWeightsByThePostingAndKeepsTheApplication(t *testing.T) {
	ctx, pool := scOpenPool(t)
	sc := newScSeed(t, ctx, pool)
	repo := repository.NewRepository(pool)
	catalog := scCatalog(t, ctx, repo)
	accuracy, depth := catalog[0].ID, catalog[1].ID
	candidate := sc.user("candidate")
	posting := sc.posting(sc.user("recruiter"), sc.jd())
	sc.weight(posting, accuracy, 70)
	sc.weight(posting, depth, 30)
	app := sc.application(posting, candidate, "interviewed")
	iv := sc.interview(sc.roleProfile(candidate, sc.jd()), "recruitment", &app, true)
	sc.turn(iv, 1, "Q1", scStr("A1"))

	scorer := &scScorer{output: domain.ScoreOutput{
		Scores:  []domain.MetricScore{{MetricID: accuracy, Score: 100}, {MetricID: depth, Score: 40}},
		Summary: "recruiter summary",
	}}
	if err := scNewScoring(t, repo, scorer).Evaluate(ctx, iv); err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}
	in := scorer.inputs[0]
	weights := map[uuid.UUID]float64{}
	for _, metric := range in.Metrics {
		weights[metric.MetricID] = metric.Weight
	}
	if len(weights) != 2 || weights[accuracy] != 70 || weights[depth] != 30 {
		t.Fatalf("scorer weights = %v, want the posting's 70 and 30", weights)
	}

	// (70*100 + 30*40) / 100 = 82.
	row := sc.readInterview(iv)
	if row.status != "completed" || row.score == nil || *row.score != 82 || row.feedback == nil || *row.feedback != "recruiter summary" {
		t.Fatalf("interview = %+v, want completed with the weighted score 82", row)
	}
	if n := sc.count(`SELECT count(*) FROM public.score_details WHERE interview_id = $1`, iv); n != 2 {
		t.Fatalf("score_details rows = %d, want 2 (one per weighted metric)", n)
	}
	if got := sc.applicationStatus(app); got != "interviewed" {
		t.Fatalf("application status = %s after evaluation, want interviewed (the AI never decides)", got)
	}
}

func TestEvaluateWithoutAnsweredQuestionsFailsAndWritesNothingElse(t *testing.T) {
	ctx, pool := scOpenPool(t)
	sc := newScSeed(t, ctx, pool)
	repo := repository.NewRepository(pool)
	catalog := scCatalog(t, ctx, repo)
	iv := sc.interview(sc.roleProfile(sc.user("candidate"), sc.jd()), "practice", nil, true)
	sc.turn(iv, 1, "Q1", nil)
	sc.turn(iv, 2, "Q2", scStr("   "))

	scorer := &scScorer{output: domain.ScoreOutput{Scores: scEach(catalog, 90), Summary: "should not be stored"}}
	if err := scNewScoring(t, repo, scorer).Evaluate(ctx, iv); err == nil {
		t.Fatal("Evaluate() error = nil for an interview with no answers, want an error")
	}
	if scorer.callCount() != 0 {
		t.Fatalf("scorer calls = %d, want none without answered questions", scorer.callCount())
	}
	row := sc.readInterview(iv)
	if row.status != "failed" || row.score != nil || row.feedback != nil {
		t.Fatalf("interview = %+v, want failed with no score and no summary", row)
	}
	if n := sc.count(`SELECT count(*) FROM public.score_details WHERE interview_id = $1`, iv); n != 0 {
		t.Fatalf("score_details rows = %d, want none", n)
	}
}

func TestEvaluateScorerErrorMarksFailedAndWritesNothingElse(t *testing.T) {
	ctx, pool := scOpenPool(t)
	sc := newScSeed(t, ctx, pool)
	repo := repository.NewRepository(pool)
	iv := sc.interview(sc.roleProfile(sc.user("candidate"), sc.jd()), "practice", nil, true)
	sc.turn(iv, 1, "Q1", scStr("A1"))

	scorer := &scScorer{err: errors.New("model unavailable")}
	if err := scNewScoring(t, repo, scorer).Evaluate(ctx, iv); err == nil {
		t.Fatal("Evaluate() error = nil after a scorer failure, want an error")
	}
	if scorer.callCount() != 1 {
		t.Fatalf("scorer calls = %d, want 1", scorer.callCount())
	}
	row := sc.readInterview(iv)
	if row.status != "failed" || row.score != nil || row.feedback != nil {
		t.Fatalf("interview = %+v, want failed with nothing written", row)
	}
	if n := sc.count(`SELECT count(*) FROM public.score_details WHERE interview_id = $1`, iv); n != 0 {
		t.Fatalf("score_details rows = %d, want none", n)
	}
	if f := sc.feedbackAt(iv, 1); f != nil {
		t.Fatalf("turn feedback = %q, want none", *f)
	}
}

func TestEvaluateRejectsInvalidScorerOutput(t *testing.T) {
	ctx, pool := scOpenPool(t)
	sc := newScSeed(t, ctx, pool)
	repo := repository.NewRepository(pool)
	catalog := scCatalog(t, ctx, repo)
	outOfRange := scEach(catalog, 50)
	outOfRange[0].Score = 150
	cases := map[string][]domain.MetricScore{
		"score above 100": outOfRange,
		"missing metric":  scEach(catalog, 50)[1:],
	}
	for name, scores := range cases {
		t.Run(name, func(t *testing.T) {
			candidate := sc.user("candidate")
			iv := sc.interview(sc.roleProfile(candidate, sc.jd()), "practice", nil, true)
			sc.turn(iv, 1, "Q1", scStr("A1"))
			scorer := &scScorer{output: domain.ScoreOutput{Scores: scores, Summary: "bad output"}}
			if err := scNewScoring(t, repo, scorer).Evaluate(ctx, iv); err == nil {
				t.Fatal("Evaluate() error = nil for an invalid scorer output, want an error")
			}
			row := sc.readInterview(iv)
			if row.status != "failed" || row.score != nil || row.feedback != nil {
				t.Fatalf("interview = %+v, want failed with nothing written", row)
			}
			if n := sc.count(`SELECT count(*) FROM public.score_details WHERE interview_id = $1`, iv); n != 0 {
				t.Fatalf("score_details rows = %d, want none", n)
			}
		})
	}
}

func TestEvaluateInvalidWeightsFailBeforeTheScorerRuns(t *testing.T) {
	ctx, pool := scOpenPool(t)
	sc := newScSeed(t, ctx, pool)
	repo := repository.NewRepository(pool)
	catalog := scCatalog(t, ctx, repo)
	candidate := sc.user("candidate")
	posting := sc.posting(sc.user("recruiter"), sc.jd())
	sc.weight(posting, catalog[0].ID, -10)
	sc.weight(posting, catalog[1].ID, 110)
	app := sc.application(posting, candidate, "interviewed")
	iv := sc.interview(sc.roleProfile(candidate, sc.jd()), "recruitment", &app, true)
	sc.turn(iv, 1, "Q1", scStr("A1"))

	scorer := &scScorer{output: domain.ScoreOutput{Scores: scEach(catalog[:2], 80)}}
	if err := scNewScoring(t, repo, scorer).Evaluate(ctx, iv); err == nil {
		t.Fatal("Evaluate() error = nil with a negative posting weight, want an error")
	}
	if scorer.callCount() != 0 {
		t.Fatalf("scorer calls = %d, want none with invalid weights", scorer.callCount())
	}
	if row := sc.readInterview(iv); row.status != "failed" || row.score != nil {
		t.Fatalf("interview = %+v, want failed with no score", row)
	}
}

func TestEvaluateNeedsACompletedInterview(t *testing.T) {
	ctx, pool := scOpenPool(t)
	sc := newScSeed(t, ctx, pool)
	repo := repository.NewRepository(pool)
	iv := sc.interview(sc.roleProfile(sc.user("candidate"), sc.jd()), "practice", nil, false)
	sc.turn(iv, 1, "Q1", scStr("A1"))

	scorer := &scorerCounter{}
	err := scNewScoring(t, repo, scorer).Evaluate(ctx, iv)
	scAssertCode(t, err, apperror.CodeInterviewNotCompleted)
	if scorer.calls != 0 {
		t.Fatalf("scorer calls = %d for an unfinished interview, want none", scorer.calls)
	}
	if row := sc.readInterview(iv); row.status != "pending" {
		t.Fatalf("evaluation status = %s, want pending", row.status)
	}
}

// scorerCounter is a Scorer that only counts calls and returns an error.
type scorerCounter struct{ calls int }

func (c *scorerCounter) Score(context.Context, domain.ScoreInput) (domain.ScoreOutput, error) {
	c.calls++
	return domain.ScoreOutput{}, errors.New("must not be called")
}

// --- Report visibility (KAN-62) ---

func TestPracticeReportIsFullForItsOwnerOnly(t *testing.T) {
	ctx, pool := scOpenPool(t)
	sc := newScSeed(t, ctx, pool)
	repo := repository.NewRepository(pool)
	catalog := scCatalog(t, ctx, repo)
	owner, stranger, recruiter := sc.user("candidate"), sc.user("candidate"), sc.user("recruiter")
	iv := sc.interview(sc.roleProfile(owner, sc.jd()), "practice", nil, false)
	sc.turn(iv, 1, "Q1", scStr("A1"))
	sc.turn(iv, 2, "Q2", nil)
	scoring := scNewScoring(t, repo, &scScorer{})

	inProgress, err := scoring.Report(ctx, owner, "candidate", iv)
	if err != nil {
		t.Fatal(err)
	}
	if inProgress.Status != "in_progress" || inProgress.CompletedAt != nil || inProgress.Summary != nil ||
		inProgress.Breakdown != nil || len(inProgress.Questions) != 2 {
		t.Fatalf("in-progress report = %+v, want in_progress with the questions and no evaluation", inProgress)
	}

	sc.exec(`UPDATE public.interviews SET completed_at = now(), evaluation_status = 'pending' WHERE id = $1`, iv)
	scorer := &scScorer{output: domain.ScoreOutput{
		Scores:    scEach(catalog, 70),
		Summary:   "practice summary",
		Questions: []domain.QuestionFeedback{{Position: 1, Feedback: "fb1"}},
	}}
	if err := scNewScoring(t, repo, scorer).Evaluate(ctx, iv); err != nil {
		t.Fatal(err)
	}

	full, err := scoring.Report(ctx, owner, "candidate", iv)
	if err != nil {
		t.Fatal(err)
	}
	if full.Status != "completed" || full.OverallScore == nil || *full.OverallScore != 70 ||
		full.Summary == nil || *full.Summary != "practice summary" || len(full.Breakdown) != len(catalog) ||
		len(full.Questions) != 2 || full.Questions[0].Feedback == nil || *full.Questions[0].Feedback != "fb1" ||
		full.Questions[0].Answer == nil || *full.Questions[0].Answer != "A1" || full.Questions[1].Feedback != nil {
		t.Fatalf("practice owner report = %+v, want the full evaluation", full)
	}

	for name, viewer := range map[string]struct {
		id   uuid.UUID
		role string
	}{
		"other candidate": {stranger, "candidate"},
		"recruiter":       {recruiter, "recruiter"},
		"admin":           {owner, "admin"},
	} {
		if _, err := scoring.Report(ctx, viewer.id, viewer.role, iv); err == nil {
			t.Fatalf("%s read a practice report, want INTERVIEW_NOT_FOUND", name)
		} else {
			scAssertCode(t, err, apperror.CodeInterviewNotFound)
		}
	}
}

func TestRecruitmentReportShowsCandidateOnlyScoreAndStatus(t *testing.T) {
	ctx, pool := scOpenPool(t)
	sc := newScSeed(t, ctx, pool)
	repo := repository.NewRepository(pool)
	catalog := scCatalog(t, ctx, repo)
	accuracy, depth := catalog[0].ID, catalog[1].ID
	candidate, otherCandidate := sc.user("candidate"), sc.user("candidate")
	recruiter, otherRecruiter := sc.user("recruiter"), sc.user("recruiter")
	posting := sc.posting(recruiter, sc.jd())
	sc.weight(posting, accuracy, 70)
	sc.weight(posting, depth, 30)
	app := sc.application(posting, candidate, "interviewed")
	iv := sc.interview(sc.roleProfile(candidate, sc.jd()), "recruitment", &app, true)
	sc.turn(iv, 1, "Q1", scStr("A1"))
	scoring := scNewScoring(t, repo, &scScorer{})

	pending, err := scoring.Report(ctx, candidate, "candidate", iv)
	if err != nil {
		t.Fatal(err)
	}
	if pending.Status != "pending" || pending.OverallScore != nil {
		t.Fatalf("report before evaluation = %+v, want status pending with no score", pending)
	}

	scorer := &scScorer{output: domain.ScoreOutput{
		Scores:  []domain.MetricScore{{MetricID: accuracy, Score: 100}, {MetricID: depth, Score: 40}},
		Summary: "the recruiter's evidence",
	}}
	if err := scNewScoring(t, repo, scorer).Evaluate(ctx, iv); err != nil {
		t.Fatal(err)
	}

	limited, err := scoring.Report(ctx, candidate, "candidate", iv)
	if err != nil {
		t.Fatal(err)
	}
	if limited.Status != "completed" || limited.OverallScore == nil || *limited.OverallScore != 82 || limited.CompletedAt == nil {
		t.Fatalf("candidate report = %+v, want completed with the score 82", limited)
	}
	if limited.Summary != nil || limited.Breakdown != nil || limited.Questions != nil {
		t.Fatalf("candidate report = %+v, want summary, breakdown and questions all null", limited)
	}

	full, err := scoring.Report(ctx, recruiter, "recruiter", iv)
	if err != nil {
		t.Fatal(err)
	}
	if full.Summary == nil || *full.Summary != "the recruiter's evidence" || len(full.Breakdown) != 2 ||
		len(full.Questions) != 1 || full.Questions[0].Answer == nil {
		t.Fatalf("owning recruiter report = %+v, want the full evidence", full)
	}

	for name, viewer := range map[string]struct {
		id   uuid.UUID
		role string
	}{
		"other recruiter": {otherRecruiter, "recruiter"},
		"other candidate": {otherCandidate, "candidate"},
		"admin":           {recruiter, "admin"},
	} {
		if _, err := scoring.Report(ctx, viewer.id, viewer.role, iv); err == nil {
			t.Fatalf("%s read a recruitment report, want INTERVIEW_NOT_FOUND", name)
		} else {
			scAssertCode(t, err, apperror.CodeInterviewNotFound)
		}
	}
}
