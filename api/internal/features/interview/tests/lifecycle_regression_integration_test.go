//go:build integration

// API-level regression for KAN-75: the practice and the recruitment interview lifecycle, end to end
// through the same service methods the HTTP handlers call, against a real PostgreSQL database.
//
// It is NOT a substitute for a real realtime run: no HTTP request, no avatar or voice, no LLM and no
// object storage is involved. The scorer is a deterministic fake and the recording store is a stub. What it
// proves is the business chain: create and charge or consume a slot, answer turns, complete, evaluate, read
// the report and transcript, decide after the gate, and refund once at the terminal close, with resume and
// retry never duplicating a charge or a slot, and the Candidate never seeing the transcript or the recording.
//
// It needs a migrated, disposable database:
//
//	cd api && set -a && . ../.env && set +a
//	go test -tags=integration ./internal/features/interview/tests -count=1
package tests

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	appdomain "github.com/cog-forge/rolecue/api/internal/features/application/domain"
	appRepo "github.com/cog-forge/rolecue/api/internal/features/application/repository"
	appService "github.com/cog-forge/rolecue/api/internal/features/application/service"
	evaldomain "github.com/cog-forge/rolecue/api/internal/features/evaluation/domain"
	evalRepo "github.com/cog-forge/rolecue/api/internal/features/evaluation/repository"
	evalService "github.com/cog-forge/rolecue/api/internal/features/evaluation/service"
	"github.com/cog-forge/rolecue/api/internal/features/interview/domain"
	"github.com/cog-forge/rolecue/api/internal/features/interview/repository"
	"github.com/cog-forge/rolecue/api/internal/features/interview/service"
	"github.com/cog-forge/rolecue/api/internal/pkg/storage"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// regAnswerMarker is a string only the Candidate's own answers contain. It must never appear in a
// Candidate-facing response of a recruitment interview.
const regAnswerMarker = "regression-answer-marker"

// regPracticeCoins and regQuestionLimit are the rules of the regression run.
const (
	regPracticeCoins = domain.PracticeInterviewCoins
	regQuestionLimit = 2
)

func regOpenPool(t *testing.T) (context.Context, *pgxpool.Pool) {
	t.Helper()
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL is not set (load ../.env)")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
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

// regScorer grades every metric with a fixed score and answers with a fixed summary.
type regScorer struct {
	output evaldomain.ScoreOutput
	calls  int
}

func (s *regScorer) Score(_ context.Context, _ evaldomain.ScoreInput) (evaldomain.ScoreOutput, error) {
	s.calls++
	return s.output, nil
}

// regCVLinker and regStore stand in for object storage. The regression never reads an object.
type regCVLinker struct{}

func (regCVLinker) PresignGet(_ context.Context, _ string, ttl time.Duration) (storage.SignedRequest, error) {
	return storage.SignedRequest{URL: "https://storage.example.test/signed?sig=regression", ExpiresAt: time.Now().Add(ttl)}, nil
}

type regStore struct{}

func (regStore) PresignPut(context.Context, string, int64, time.Duration) (storage.UploadGrant, error) {
	return storage.UploadGrant{}, fmt.Errorf("regression run does not upload")
}

func (regStore) PresignGet(_ context.Context, _ string, ttl time.Duration) (storage.SignedRequest, error) {
	return regCVLinker{}.PresignGet(context.Background(), "", ttl)
}

func (regStore) Download(context.Context, string) (storage.Object, error) {
	return storage.Object{Body: io.NopCloser(strings.NewReader(""))}, fmt.Errorf("regression run does not download")
}

// regSeed inserts the rows the lifecycle needs and records their ids. The cleanup deletes them in
// foreign-key order: score_details, conversation_turns, interview_questions, interviews, applications,
// metrics_percentage, job_postings, core_questions, role_profiles, job_descriptions, transactions,
// wallets, users.
type regSeed struct {
	t        *testing.T
	ctx      context.Context
	pool     *pgxpool.Pool
	users    []string
	jds      []string
	postings []string
}

func newRegSeed(t *testing.T, ctx context.Context, pool *pgxpool.Pool) *regSeed {
	s := &regSeed{t: t, ctx: ctx, pool: pool}
	t.Cleanup(s.cleanup)
	return s
}

func (s *regSeed) exec(sql string, args ...any) {
	s.t.Helper()
	if _, err := s.pool.Exec(s.ctx, sql, args...); err != nil {
		s.t.Fatalf("seed %q: %v", sql, err)
	}
}

func (s *regSeed) user(role string) uuid.UUID {
	id := uuid.New()
	var company *string
	if role == "recruiter" {
		acme := "Acme"
		company = &acme
	}
	s.exec(`INSERT INTO public.users (id, name, email, role, email_verified, company_name)
		VALUES ($1, $2, $3, $4, true, $5)`, id, "reg "+role, fmt.Sprintf("reg-%s@example.test", id), role, company)
	s.users = append(s.users, id.String())
	return id
}

// wallet gives a user a coin balance; the user has no wallet otherwise.
func (s *regSeed) wallet(userID uuid.UUID, balance int32) {
	s.exec(`INSERT INTO public.wallets (user_id, balance) VALUES ($1, $2)`, userID, balance)
}

func (s *regSeed) balance(userID uuid.UUID) int32 {
	s.t.Helper()
	var balance int32
	if err := s.pool.QueryRow(s.ctx, `SELECT balance FROM public.wallets WHERE user_id = $1`, userID).Scan(&balance); err != nil {
		s.t.Fatalf("wallet of %s: %v", userID, err)
	}
	return balance
}

func (s *regSeed) walletRows(userID uuid.UUID) int {
	s.t.Helper()
	return s.count(`SELECT count(*) FROM public.wallets WHERE user_id = $1`, userID)
}

func (s *regSeed) count(sql string, args ...any) int {
	s.t.Helper()
	var n int
	if err := s.pool.QueryRow(s.ctx, sql, args...).Scan(&n); err != nil {
		s.t.Fatal(err)
	}
	return n
}

func (s *regSeed) ledgerRows(userID uuid.UUID) int {
	return s.count(`SELECT count(*) FROM public.transactions WHERE "from"::text = $1 OR "to"::text = $1`, userID.String())
}

// jd adds a Job Description with n core questions, the question bank a session asks from.
func (s *regSeed) jd(questions int) uuid.UUID {
	s.t.Helper()
	var levelID uuid.UUID
	if err := s.pool.QueryRow(s.ctx, `SELECT id FROM public.levels ORDER BY id LIMIT 1`).Scan(&levelID); err != nil {
		s.t.Fatalf("no seniority level seeded: %v", err)
	}
	id := uuid.New()
	s.exec(`INSERT INTO public.job_descriptions (id, content_path, level_id) VALUES ($1, $2, $3)`,
		id, "job-postings/"+id.String()+".pdf", levelID)
	s.jds = append(s.jds, id.String())
	for i := 1; i <= questions; i++ {
		s.exec(`INSERT INTO public.core_questions (jd_id, content) VALUES ($1, $2)`, id, fmt.Sprintf("Regression question %d", i))
	}
	return id
}

// roleProfile adds a confirmed Role Profile of the Candidate over its own Job Description.
func (s *regSeed) roleProfile(candidateID uuid.UUID, questions int) uuid.UUID {
	id := uuid.New()
	s.exec(`INSERT INTO public.role_profiles (id, jd_id, candidate_id, title, status)
		VALUES ($1, $2, $3, 'regression profile', 'confirmed')`, id, s.jd(questions), candidateID)
	return id
}

// posting adds an open posting with no interview capacity yet; FundCapacity sets it.
func (s *regSeed) posting(recruiterID, jdID uuid.UUID) uuid.UUID {
	id := uuid.New()
	s.exec(`INSERT INTO public.job_postings (id, status, jd_id, recruiter_id, title)
		VALUES ($1, 'open'::public.job_posting_status, $2, $3, 'regression posting')`, id, jdID, recruiterID)
	s.postings = append(s.postings, id.String())
	return id
}

// application adds a pending Application with a stored CV, the state a candidate submission creates.
func (s *regSeed) application(postingID, candidateID uuid.UUID) uuid.UUID {
	id := uuid.New()
	s.exec(`INSERT INTO public.applications (id, job_posting_id, candidate_id, cv_path)
		VALUES ($1, $2, $3, 'cvs/regression.pdf')`, id, postingID, candidateID)
	return id
}

// slotConsumed counts the interviews that used a slot of the posting.
func (s *regSeed) slotConsumed(postingID uuid.UUID) int {
	return s.count(`SELECT count(*) FROM public.interviews i
		JOIN public.applications a ON a.id = i.application_id WHERE a.job_posting_id = $1`, postingID)
}

func (s *regSeed) applicationStatus(applicationID uuid.UUID) string {
	s.t.Helper()
	var status string
	if err := s.pool.QueryRow(s.ctx, `SELECT status::text FROM public.applications WHERE id = $1`, applicationID).Scan(&status); err != nil {
		s.t.Fatal(err)
	}
	return status
}

func (s *regSeed) postingStatus(postingID uuid.UUID) string {
	s.t.Helper()
	var status string
	if err := s.pool.QueryRow(s.ctx, `SELECT status::text FROM public.job_postings WHERE id = $1`, postingID).Scan(&status); err != nil {
		s.t.Fatal(err)
	}
	return status
}

// setDeadline moves the interview deadline of an Application. The test clock is the only reason for it:
// the 24-hour window of CV approval cannot elapse inside a test run.
func (s *regSeed) setDeadline(applicationID uuid.UUID, at time.Time) {
	s.exec(`UPDATE public.applications SET interview_deadline = $2 WHERE id = $1`, applicationID, at)
}

func (s *regSeed) cleanup() {
	ctx := context.Background()
	byCandidate := `(SELECT i.id FROM public.interviews i
		LEFT JOIN public.role_profiles rp ON rp.id = i.role_profile_id
		LEFT JOIN public.applications a ON a.id = i.application_id
		WHERE rp.candidate_id::text = ANY($1::text[]) OR a.candidate_id::text = ANY($1::text[]))`
	steps := []struct {
		sql  string
		args []any
	}{
		{`DELETE FROM public.score_details WHERE interview_id IN ` + byCandidate, []any{s.users}},
		{`DELETE FROM public.conversation_turns WHERE interview_id IN ` + byCandidate, []any{s.users}},
		{`DELETE FROM public.interview_questions WHERE interview_id IN ` + byCandidate, []any{s.users}},
		{`DELETE FROM public.interviews WHERE id IN ` + byCandidate, []any{s.users}},
		{`DELETE FROM public.applications WHERE candidate_id::text = ANY($1::text[])`, []any{s.users}},
		{`DELETE FROM public.metrics_percentage WHERE job_posting_id::text = ANY($1::text[])`, []any{s.postings}},
		{`DELETE FROM public.job_postings WHERE id::text = ANY($1::text[])`, []any{s.postings}},
		{`DELETE FROM public.core_questions WHERE jd_id::text = ANY($1::text[])`, []any{s.jds}},
		{`DELETE FROM public.role_profiles WHERE candidate_id::text = ANY($1::text[])`, []any{s.users}},
		{`DELETE FROM public.job_descriptions WHERE id::text = ANY($1::text[])`, []any{s.jds}},
		{`DELETE FROM public.transactions WHERE "from"::text = ANY($1::text[]) OR "to"::text = ANY($1::text[])`, []any{s.users}},
		{`DELETE FROM public.wallets WHERE user_id::text = ANY($1::text[])`, []any{s.users}},
		{`DELETE FROM public.users WHERE id::text = ANY($1::text[])`, []any{s.users}},
	}
	for _, step := range steps {
		if _, err := s.pool.Exec(ctx, step.sql, step.args...); err != nil {
			s.t.Errorf("cleanup %q: %v", step.sql, err)
		}
	}
}

// regServices are the feature services the lifecycle calls, built over the real repositories.
type regServices struct {
	interview *service.Service
	recording *service.RecordingService
	scoring   *evalService.Scoring
	scorer    *regScorer
	apps      *appService.Service
	metrics   []evaldomain.Metric
	pool      *pgxpool.Pool
}

func regBuild(t *testing.T, ctx context.Context, pool *pgxpool.Pool) regServices {
	t.Helper()
	interviewRepo := repository.NewRepository(pool)
	interviewSvc, err := service.New(interviewRepo, service.Settings{QuestionLimit: regQuestionLimit})
	if err != nil {
		t.Fatal(err)
	}
	recording, err := service.NewRecordingService(interviewRepo, regStore{})
	if err != nil {
		t.Fatal(err)
	}
	evalRepository := evalRepo.NewRepository(pool)
	metrics, err := evalRepository.ListMetrics(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(metrics) < 2 {
		t.Fatalf("metrics catalog has %d entries, want at least 2 (migration 000011)", len(metrics))
	}
	scorer := &regScorer{output: evaldomain.ScoreOutput{
		Scores:  []evaldomain.MetricScore{{MetricID: metrics[0].ID, Score: 90}, {MetricID: metrics[1].ID, Score: 50}},
		Summary: "regression summary",
		Questions: []evaldomain.QuestionFeedback{
			{Position: 1, Feedback: "regression feedback 1"},
			{Position: 2, Feedback: "regression feedback 2"},
		},
	}}
	scoring, err := evalService.NewScoring(evalRepository, scorer)
	if err != nil {
		t.Fatal(err)
	}
	apps, err := appService.New(appRepo.NewRepository(pool), regCVLinker{})
	if err != nil {
		t.Fatal(err)
	}
	return regServices{
		interview: interviewSvc, recording: recording, scoring: scoring, scorer: scorer,
		apps: apps, metrics: metrics, pool: pool,
	}
}

// regAnswer returns an answer the Candidate gave, carrying the marker.
func regAnswer(n int) string {
	return fmt.Sprintf("%s answer %d", regAnswerMarker, n)
}

// regJSON is the serialized body a client would receive, for leak checks.
func regJSON(t *testing.T, v any) string {
	t.Helper()
	body, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}

func regAssertCode(t *testing.T, err error, want apperror.Code) {
	t.Helper()
	var appErr *apperror.AppError
	if !errors.As(err, &appErr) || appErr.Code != want {
		t.Fatalf("error = %v, want code %s", err, want)
	}
}

// TestKAN75PracticeLifecycleAPIRegression runs one practice session: role profile, create and charge,
// same-session resume, answers and a retry, complete, evaluate, the candidate report and the history.
func TestKAN75PracticeLifecycleAPIRegression(t *testing.T) {
	ctx, pool := regOpenPool(t)
	s := newRegSeed(t, ctx, pool)
	svc := regBuild(t, ctx, pool)
	candidate := s.user("candidate")
	s.wallet(candidate, 10)
	roleID := s.roleProfile(candidate, regQuestionLimit)

	created, wasCreated, err := svc.interview.Create(ctx, candidate, "candidate", domain.CreateInput{
		Type: domain.TypePractice, RoleProfileID: &roleID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !wasCreated || created.QuestionCount != regQuestionLimit || created.CurrentQuestion == nil || created.CurrentQuestion.Position != 1 {
		t.Fatalf("Create() = %+v created=%v, want a new session with the first question", created, wasCreated)
	}
	if got := s.balance(candidate); got != 10-regPracticeCoins {
		t.Fatalf("balance after create = %d, want %d", got, 10-regPracticeCoins)
	}
	ledgerAfterCreate := s.ledgerRows(candidate)

	// Resume before any answer is the same session, with no charge.
	resumed, err := svc.interview.Resume(ctx, candidate, "candidate", created.ID)
	if err != nil || resumed.ID != created.ID {
		t.Fatalf("Resume() = %+v, %v, want the same session", resumed, err)
	}

	if _, err := svc.interview.AppendTurn(ctx, candidate, "candidate", created.ID, 1, regAnswer(1)); err != nil {
		t.Fatal(err)
	}
	// Retrying the same answer changes nothing.
	retried, err := svc.interview.AppendTurn(ctx, candidate, "candidate", created.ID, 1, regAnswer(1))
	if err != nil || retried.AnsweredCount != 1 {
		t.Fatalf("retried AppendTurn() = %+v, %v, want the session unchanged with one answer", retried, err)
	}
	if _, err := svc.interview.AppendTurn(ctx, candidate, "candidate", created.ID, 2, regAnswer(2)); err != nil {
		t.Fatal(err)
	}
	// Resume after the answers: still the same session, and no second charge.
	resumed, err = svc.interview.Resume(ctx, candidate, "candidate", created.ID)
	if err != nil || resumed.ID != created.ID || resumed.AnsweredCount != regQuestionLimit {
		t.Fatalf("Resume() after answers = %+v, %v, want the same session with %d answers", resumed, err, regQuestionLimit)
	}

	completed, err := svc.interview.Complete(ctx, candidate, "candidate", created.ID)
	if err != nil || completed.CompletedAt == nil {
		t.Fatalf("Complete() = %+v, %v, want completed", completed, err)
	}
	// Practice has no posting weights: every catalog metric counts equally, so a uniform 70 is the overall 70.
	scores := make([]evaldomain.MetricScore, 0, len(svc.metrics))
	for _, metric := range svc.metrics {
		scores = append(scores, evaldomain.MetricScore{MetricID: metric.ID, Score: 70})
	}
	svc.scorer.output.Scores = scores
	if err := svc.scoring.Evaluate(ctx, created.ID); err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}
	if svc.scorer.calls != 1 {
		t.Fatalf("scorer calls = %d, want 1", svc.scorer.calls)
	}

	report, err := svc.scoring.Report(ctx, candidate, "candidate", created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if report.Status != "completed" || report.OverallScore == nil || *report.OverallScore != 70 ||
		report.Summary == nil || *report.Summary != "regression summary" || len(report.Breakdown) != len(svc.metrics) ||
		len(report.Questions) != regQuestionLimit {
		t.Fatalf("practice report = %+v, want the full evaluation with score 70 over every metric", report)
	}

	history, err := svc.interview.ListHistory(ctx, candidate, "candidate", domain.HistoryFilter{Type: domain.TypePractice, Page: 1, PageSize: 20})
	if err != nil {
		t.Fatal(err)
	}
	if history.Total != 1 || len(history.Items) != 1 || history.Items[0].ID != created.ID ||
		history.Items[0].Status != "completed" || history.Items[0].Score == nil || *history.Items[0].Score != 70 {
		t.Fatalf("history = %+v, want the one completed practice session with score 70", history)
	}

	if _, err := svc.interview.Resume(ctx, candidate, "candidate", created.ID); err == nil {
		t.Fatal("Resume() after completion succeeded, want INTERVIEW_COMPLETED")
	} else {
		regAssertCode(t, err, apperror.CodeInterviewCompleted)
	}
	if got := s.balance(candidate); got != 10-regPracticeCoins {
		t.Fatalf("balance = %d after resume and answers, want the single charge kept", got)
	}
	if n := s.ledgerRows(candidate); n != ledgerAfterCreate {
		t.Fatalf("ledger rows = %d after resume and answers, want %d (no second charge)", n, ledgerAfterCreate)
	}
}

// TestKAN75RecruitmentLifecycleAPIRegression runs one recruitment interview from the funded posting to the
// terminal close: CV approval, the consumed slot, same-session resume with no second slot, the candidate's
// limited view, completion, evaluation, the recruiter's report and transcript, the decision after the gate
// and the single refund of unused capacity.
func TestKAN75RecruitmentLifecycleAPIRegression(t *testing.T) {
	ctx, pool := regOpenPool(t)
	s := newRegSeed(t, ctx, pool)
	svc := regBuild(t, ctx, pool)
	recruiter := s.user("recruiter")
	s.wallet(recruiter, 10)
	candidate := s.user("candidate")
	// A confirmed Role Profile is needed for the recruitment start; its own Job Description has no questions.
	s.roleProfile(candidate, 0)
	posting := s.posting(recruiter, s.jd(regQuestionLimit))
	weights := []struct {
		metric uuid.UUID
		pct    float64
	}{{svc.metrics[0].ID, 60}, {svc.metrics[1].ID, 40}}
	for _, w := range weights {
		s.exec(`INSERT INTO public.metrics_percentage (metric_id, percentage, job_posting_id) VALUES ($1, $2, $3)`,
			w.metric, w.pct, posting)
	}

	// Funded with two slots, then one Application is CV-approved; the posting is closed for intake.
	if _, err := svc.apps.FundCapacity(ctx, recruiter, "recruiter", posting, 2); err != nil {
		t.Fatalf("FundCapacity() error = %v", err)
	}
	if got := s.balance(recruiter); got != 8 {
		t.Fatalf("recruiter balance after funding = %d, want 8", got)
	}
	app := s.application(posting, candidate)
	if _, err := svc.apps.ApproveCV(ctx, recruiter, "recruiter", posting, app); err != nil {
		t.Fatalf("ApproveCV() error = %v", err)
	}
	if _, err := svc.apps.CloseIntake(ctx, recruiter, "recruiter", posting); err != nil {
		t.Fatal(err)
	}

	// Create consumes one slot; a repeat create and a resume do not.
	created, wasCreated, err := svc.interview.Create(ctx, candidate, "candidate", domain.CreateInput{
		Type: domain.TypeRecruitment, ApplicationID: &app,
	})
	if err != nil || !wasCreated || created.ApplicationID == nil || *created.ApplicationID != app {
		t.Fatalf("Create() = %+v created=%v err=%v, want a new recruitment session", created, wasCreated, err)
	}
	if n := s.slotConsumed(posting); n != 1 {
		t.Fatalf("consumed slots = %d after create, want 1", n)
	}
	again, wasCreated, err := svc.interview.Create(ctx, candidate, "candidate", domain.CreateInput{
		Type: domain.TypeRecruitment, ApplicationID: &app,
	})
	if err != nil || wasCreated || again.ID != created.ID {
		t.Fatalf("repeat Create() = %+v created=%v err=%v, want the same session with no side effect", again, wasCreated, err)
	}
	resumed, err := svc.interview.Resume(ctx, candidate, "candidate", created.ID)
	if err != nil || resumed.ID != created.ID {
		t.Fatalf("Resume() = %+v, %v, want the same session", resumed, err)
	}
	if n := s.slotConsumed(posting); n != 1 {
		t.Fatalf("consumed slots = %d after repeat create and resume, want still 1", n)
	}
	if n := s.walletRows(candidate); n != 0 || s.ledgerRows(candidate) != 0 {
		t.Fatal("a recruitment interview charged the candidate, want no charge")
	}

	// The candidate answers both questions; the session never shows them back as a transcript.
	if _, err := svc.interview.AppendTurn(ctx, candidate, "candidate", created.ID, 1, regAnswer(1)); err != nil {
		t.Fatal(err)
	}
	mid, err := svc.interview.Get(ctx, candidate, "candidate", created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if mid.Turns != nil || mid.CurrentQuestion == nil || mid.CurrentQuestion.Position != 2 {
		t.Fatalf("candidate session mid-way = %+v, want only the current question 2 and no turns", mid)
	}
	if strings.Contains(regJSON(t, mid), regAnswerMarker) {
		t.Fatal("candidate session leaks the candidate's stored answer")
	}
	if _, err := svc.interview.AppendTurn(ctx, candidate, "candidate", created.ID, 2, regAnswer(2)); err != nil {
		t.Fatal(err)
	}
	done, err := svc.interview.Complete(ctx, candidate, "candidate", created.ID)
	if err != nil || done.CompletedAt == nil {
		t.Fatalf("Complete() = %+v, %v, want completed", done, err)
	}
	if got := s.applicationStatus(app); got != "interviewed" {
		t.Fatalf("application status = %s after completion, want interviewed", got)
	}

	if err := svc.scoring.Evaluate(ctx, created.ID); err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}
	// (60*90 + 40*50) / 100 = 74.
	recruiterReport, err := svc.scoring.Report(ctx, recruiter, "recruiter", created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if recruiterReport.Status != "completed" || recruiterReport.OverallScore == nil || *recruiterReport.OverallScore != 74 ||
		recruiterReport.Summary == nil || len(recruiterReport.Breakdown) != 2 || len(recruiterReport.Questions) != regQuestionLimit {
		t.Fatalf("recruiter report = %+v, want the evaluation with score 74", recruiterReport)
	}
	candidateReport, err := svc.scoring.Report(ctx, candidate, "candidate", created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if candidateReport.OverallScore == nil || *candidateReport.OverallScore != 74 ||
		candidateReport.Summary != nil || candidateReport.Breakdown != nil || candidateReport.Questions != nil {
		t.Fatalf("candidate report = %+v, want the score only", candidateReport)
	}
	if strings.Contains(regJSON(t, candidateReport), regAnswerMarker) {
		t.Fatal("candidate report leaks the transcript")
	}

	transcript, err := svc.recording.Transcript(ctx, recruiter, "recruiter", posting, app)
	if err != nil {
		t.Fatal(err)
	}
	if len(transcript.Turns) != regQuestionLimit || transcript.Turns[0].Answer == nil || !strings.Contains(*transcript.Turns[0].Answer, regAnswerMarker) {
		t.Fatalf("recruiter transcript = %+v, want the two answered turns", transcript)
	}
	if _, err := svc.recording.Transcript(ctx, candidate, "candidate", posting, app); err == nil {
		t.Fatal("the candidate read the transcript, want it hidden")
	}
	recording, err := svc.recording.Recording(ctx, recruiter, "recruiter", posting, app)
	if err != nil || recording.RecordStatus != "none" || recording.URL != nil {
		t.Fatalf("Recording() = %+v, %v, want status none with no link", recording, err)
	}
	mine, err := svc.apps.GetMine(ctx, candidate, "candidate", app)
	if err != nil {
		t.Fatal(err)
	}
	if mine.Interview == nil || mine.Interview.Score == nil || *mine.Interview.Score != 74 || strings.Contains(regJSON(t, mine), regAnswerMarker) {
		t.Fatalf("candidate application = %+v, want the interview score and no transcript", mine)
	}

	// The decision waits for the deadline of the posting: 24 hours after the approval, so it is locked now.
	if _, err := svc.apps.Decide(ctx, recruiter, "recruiter", posting, app, appdomain.DecisionApprove, nil); err == nil {
		t.Fatal("Decide() succeeded before the interview deadline, want FINAL_REVIEW_LOCKED")
	} else {
		regAssertCode(t, err, apperror.CodeFinalReviewLocked)
	}
	// The window closes (the test moves the deadline into the past).
	s.setDeadline(app, time.Now().Add(-time.Minute))
	decided, err := svc.apps.Decide(ctx, recruiter, "recruiter", posting, app, appdomain.DecisionApprove, nil)
	if err != nil {
		t.Fatalf("Decide() after the gate error = %v", err)
	}
	if decided.Status != appdomain.StatusApproved {
		t.Fatalf("status = %s after the decision, want approved", decided.Status)
	}

	// The last live Application is terminal: the posting closes and one unused slot is refunded (2 - 1 consumed).
	if status := s.postingStatus(posting); status != "closed" {
		t.Fatalf("posting status = %s after the last decision, want closed", status)
	}
	if got := s.balance(recruiter); got != 9 {
		t.Fatalf("recruiter balance = %d after the refund, want 9", got)
	}
	if _, err := svc.apps.Decide(ctx, recruiter, "recruiter", posting, app, appdomain.DecisionReject, nil); err == nil {
		t.Fatal("a repeated decision succeeded, want APPLICATION_NOT_INTERVIEWED")
	}
	if got := s.balance(recruiter); got != 9 {
		t.Fatalf("recruiter balance = %d after a repeated decision, want still 9 (refund once)", got)
	}
}
