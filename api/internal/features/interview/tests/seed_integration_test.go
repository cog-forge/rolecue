//go:build integration

// Package tests: interview session integration tests against a real Postgres. The repository, the
// service and the recording and history flows run unmodified; only object storage and the evaluator
// are fakes. Every test seeds its own users, postings and interviews and removes exactly those rows
// afterwards, so the tests stay independent of each other and of the rest of the database.
package tests

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"testing"
	"time"

	"github.com/cog-forge/rolecue/api/internal/features/interview/repository"
	"github.com/cog-forge/rolecue/api/internal/features/interview/service"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// openPool connects to the disposable database named by DATABASE_URL. The test is skipped when the
// variable is unset or the database is unreachable.
func openPool(t *testing.T) (context.Context, *pgxpool.Pool) {
	t.Helper()
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL is not set (load ../.env)")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
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

// seed tracks every row a test creates. The cleanup removes them in foreign-key order, scoped to the
// ids the test made, so nothing outside the test is touched.
type seed struct {
	t        *testing.T
	ctx      context.Context
	pool     *pgxpool.Pool
	users    []string
	jds      []string
	postings []string
	voices   []string
}

func newSeed(t *testing.T, ctx context.Context, pool *pgxpool.Pool) *seed {
	t.Helper()
	s := &seed{t: t, ctx: ctx, pool: pool}
	t.Cleanup(s.cleanup)
	return s
}

// interviewScope selects the interviews of the seeded candidates and postings: a session belongs to
// its candidate through the role profile, and a recruitment session also through its Application.
const interviewScope = `(SELECT i.id FROM public.interviews i
	LEFT JOIN public.role_profiles rp ON rp.id = i.role_profile_id
	LEFT JOIN public.applications a ON a.id = i.application_id
	WHERE rp.candidate_id = ANY($1::text[]::uuid[])
	   OR a.candidate_id = ANY($1::text[]::uuid[])
	   OR a.job_posting_id = ANY($2::text[]::uuid[]))`

// cleanup deletes the seeded rows children first: scores, turns and questions, interviews,
// applications, postings, the question banks, role profiles, job descriptions, coin rows, wallets and
// users. Each step is one statement, so a failure names the table that refused.
func (s *seed) cleanup() {
	ctx := context.Background()
	users, postings, jds := s.users, s.postings, s.jds
	userIDs := append([]string(nil), users...)
	steps := []struct {
		name string
		sql  string
		args []any
	}{
		{"score_details", `DELETE FROM public.score_details WHERE interview_id IN ` + interviewScope,
			[]any{userIDs, postings}},
		{"conversation_turns", `DELETE FROM public.conversation_turns WHERE interview_id IN ` + interviewScope,
			[]any{userIDs, postings}},
		{"interview_questions", `DELETE FROM public.interview_questions WHERE interview_id IN ` + interviewScope,
			[]any{userIDs, postings}},
		{"interviews", `DELETE FROM public.interviews WHERE id IN ` + interviewScope,
			[]any{userIDs, postings}},
		{"avatars", `DELETE FROM public.avatars WHERE inventory_id IN
			(SELECT id FROM public.inventories WHERE user_id = ANY($1::text[]::uuid[]))`, []any{userIDs}},
		{"inventories", `DELETE FROM public.inventories WHERE user_id = ANY($1::text[]::uuid[])`, []any{userIDs}},
		{"voices", `DELETE FROM public.voices WHERE id = ANY($1::text[]::uuid[])`, []any{s.voices}},
		{"applications", `DELETE FROM public.applications WHERE candidate_id = ANY($1::text[]::uuid[])
			OR job_posting_id = ANY($2::text[]::uuid[])`, []any{userIDs, postings}},
		{"job_postings", `DELETE FROM public.job_postings WHERE id = ANY($1::text[]::uuid[])`,
			[]any{postings}},
		{"core_questions", `DELETE FROM public.core_questions WHERE jd_id = ANY($1::text[]::uuid[])`,
			[]any{jds}},
		{"role_profiles", `DELETE FROM public.role_profiles WHERE candidate_id = ANY($1::text[]::uuid[])`,
			[]any{userIDs}},
		{"job_descriptions", `DELETE FROM public.job_descriptions WHERE id = ANY($1::text[]::uuid[])`,
			[]any{jds}},
		{"transactions", `DELETE FROM public.transactions WHERE "from" = ANY($1::text[]) OR "to" = ANY($1::text[])`,
			[]any{userIDs}},
		{"wallets", `DELETE FROM public.wallets WHERE user_id = ANY($1::text[]::uuid[])`, []any{userIDs}},
		{"users", `DELETE FROM public.users WHERE id = ANY($1::text[]::uuid[])`, []any{userIDs}},
	}
	for _, step := range steps {
		if _, err := s.pool.Exec(ctx, step.sql, step.args...); err != nil {
			s.t.Errorf("cleanup %s: %v", step.name, err)
		}
	}
}

func (s *seed) exec(sql string, args ...any) {
	s.t.Helper()
	if _, err := s.pool.Exec(s.ctx, sql, args...); err != nil {
		s.t.Fatalf("seed %q: %v", sql, err)
	}
}

func (s *seed) count(sql string, args ...any) int {
	s.t.Helper()
	var n int
	if err := s.pool.QueryRow(s.ctx, sql, args...).Scan(&n); err != nil {
		s.t.Fatalf("count %q: %v", sql, err)
	}
	return n
}

// user adds a user of the role. Recruiters get a company name, as the account rules expect.
func (s *seed) user(role string) uuid.UUID {
	s.t.Helper()
	id := uuid.New()
	var company *string
	if role == "recruiter" {
		name := "Interview Test Co"
		company = &name
	}
	s.exec(`INSERT INTO public.users (id, name, email, role, email_verified, company_name)
		VALUES ($1, $2, $3, $4, true, $5)`, id, "interview "+role, fmt.Sprintf("interview-%s@example.test", id), role, company)
	s.users = append(s.users, id.String())
	return id
}

// wallet gives the user a wallet with the balance. The row is removed with the user.
func (s *seed) wallet(userID uuid.UUID, balance int32) {
	s.t.Helper()
	s.exec(`INSERT INTO public.wallets (user_id, balance) VALUES ($1, $2)
		ON CONFLICT (user_id) DO UPDATE SET balance = EXCLUDED.balance`, userID, balance)
}

// balance reads the wallet balance, failing when the user has no wallet.
func (s *seed) balance(userID uuid.UUID) int32 {
	s.t.Helper()
	var balance int32
	if err := s.pool.QueryRow(s.ctx, `SELECT balance FROM public.wallets WHERE user_id = $1`, userID).Scan(&balance); err != nil {
		s.t.Fatalf("wallet of %s: %v", userID, err)
	}
	return balance
}

// ledgerRow is one transactions row.
type ledgerRow struct {
	amount      int32
	from, to    string
	currency    string
	status      string
	description string
}

// coinRows returns every transactions row the user takes part in.
func (s *seed) coinRows(userID uuid.UUID) []ledgerRow {
	s.t.Helper()
	rows, err := s.pool.Query(s.ctx,
		`SELECT amount, "from", "to", currency, status::text, COALESCE(description, '')
		 FROM public.transactions WHERE "from" = $1 OR "to" = $1 ORDER BY description`, userID.String())
	if err != nil {
		s.t.Fatal(err)
	}
	defer rows.Close()
	var out []ledgerRow
	for rows.Next() {
		var r ledgerRow
		if err := rows.Scan(&r.amount, &r.from, &r.to, &r.currency, &r.status, &r.description); err != nil {
			s.t.Fatal(err)
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		s.t.Fatal(err)
	}
	return out
}

// jobDescription adds a Job Description and returns its id. The question bank hangs off it.
func (s *seed) jobDescription() uuid.UUID {
	s.t.Helper()
	var levelID uuid.UUID
	if err := s.pool.QueryRow(s.ctx, `SELECT id FROM public.levels ORDER BY id LIMIT 1`).Scan(&levelID); err != nil {
		s.t.Fatalf("no seniority level seeded: %v", err)
	}
	jdID := uuid.New()
	s.exec(`INSERT INTO public.job_descriptions (id, content_path, level_id) VALUES ($1, $2, $3)`,
		jdID, "job-descriptions/"+jdID.String()+".pdf", levelID)
	s.jds = append(s.jds, jdID.String())
	return jdID
}

// coreQuestions adds n bank questions to a Job Description and returns their ids ordered by id, which
// is the order a session reads the bank in. Each question text carries its own id so tests can tell
// questions apart in JSON.
func (s *seed) coreQuestions(jdID uuid.UUID, n int) []uuid.UUID {
	s.t.Helper()
	ids := make([]uuid.UUID, 0, n)
	for i := 0; i < n; i++ {
		id := uuid.New()
		s.exec(`INSERT INTO public.core_questions (id, jd_id, content) VALUES ($1, $2, $3)`,
			id, jdID, fmt.Sprintf("bank question %s", id))
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i].String() < ids[j].String() })
	return ids
}

// contentOf reads the bank text of one core question.
func (s *seed) contentOf(coreQuestionID uuid.UUID) string {
	s.t.Helper()
	var content string
	if err := s.pool.QueryRow(s.ctx, `SELECT content FROM public.core_questions WHERE id = $1`, coreQuestionID).Scan(&content); err != nil {
		s.t.Fatal(err)
	}
	return content
}

// roleProfile gives the candidate a role profile of the status with a bank of n questions. It returns
// the profile id and the bank ids in session order.
func (s *seed) roleProfile(candidateID uuid.UUID, status string, questions int) (uuid.UUID, []uuid.UUID) {
	s.t.Helper()
	jdID := s.jobDescription()
	bank := s.coreQuestions(jdID, questions)
	profileID := uuid.New()
	s.exec(`INSERT INTO public.role_profiles (id, jd_id, candidate_id, title, status)
		VALUES ($1, $2, $3, 'interview test', $4)`, profileID, jdID, candidateID, status)
	return profileID, bank
}

// confirmedProfile is roleProfile with the confirmed status, the only one a session starts from.
func (s *seed) confirmedProfile(candidateID uuid.UUID, questions int) (uuid.UUID, []uuid.UUID) {
	return s.roleProfile(candidateID, "confirmed", questions)
}

// openPosting adds a recruiter with an open posting funded with slot interview places (nil means the
// capacity was never funded) and a bank of questions on the posting's Job Description.
func (s *seed) openPosting(slot *int32, questions int) (uuid.UUID, uuid.UUID, []uuid.UUID) {
	s.t.Helper()
	recruiterID := s.user("recruiter")
	jdID := s.jobDescription()
	bank := s.coreQuestions(jdID, questions)
	postingID := uuid.New()
	s.exec(`INSERT INTO public.job_postings (id, status, jd_id, recruiter_id, title, interview_slot)
		VALUES ($1, 'open', $2, $3, 'interview posting', $4)`, postingID, jdID, recruiterID, slot)
	s.postings = append(s.postings, postingID.String())
	return recruiterID, postingID, bank
}

// application adds an Application of the candidate to the posting in the status with the deadline.
// A nil deadline leaves the column empty.
func (s *seed) application(postingID, candidateID uuid.UUID, status string, deadline *time.Time) uuid.UUID {
	s.t.Helper()
	id := uuid.New()
	s.exec(`INSERT INTO public.applications (id, job_posting_id, candidate_id, cv_path, status, interview_deadline)
		VALUES ($1, $2, $3, 'cvs/interview-test.pdf', $4::public.application_status, $5)`,
		id, postingID, candidateID, status, deadline)
	return id
}

// eligibleApplication adds an Application the Recruiter has moved to interview_eligible, with a
// deadline a day ahead.
func (s *seed) eligibleApplication(postingID, candidateID uuid.UUID) uuid.UUID {
	s.t.Helper()
	deadline := time.Now().Add(24 * time.Hour)
	return s.application(postingID, candidateID, "interview_eligible", &deadline)
}

// setDeadlineInPast moves an Application's interview deadline an hour back, using the database clock.
func (s *seed) setDeadlineInPast(applicationID uuid.UUID) {
	s.t.Helper()
	s.exec(`UPDATE public.applications SET interview_deadline = now() - interval '1 hour' WHERE id = $1`, applicationID)
}

// snapshot returns the core question ids a session was created with, in position order.
func (s *seed) snapshot(interviewID uuid.UUID) []uuid.UUID {
	s.t.Helper()
	rows, err := s.pool.Query(s.ctx,
		`SELECT core_question_id FROM public.interview_questions WHERE interview_id = $1 ORDER BY position`, interviewID)
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
	return out
}

// applicationStatus reads the stored status of an Application.
func (s *seed) applicationStatus(applicationID uuid.UUID) string {
	s.t.Helper()
	var status string
	if err := s.pool.QueryRow(s.ctx, `SELECT status::text FROM public.applications WHERE id = $1`, applicationID).Scan(&status); err != nil {
		s.t.Fatal(err)
	}
	return status
}

// newService builds the interview service over the real repository with the given rules.
func newService(t *testing.T, pool *pgxpool.Pool, questionLimit int32) *service.Service {
	t.Helper()
	svc, err := service.New(repository.NewRepository(pool), service.Settings{
		QuestionLimit: questionLimit,
	})
	if err != nil {
		t.Fatal(err)
	}
	return svc
}

// codeOf returns the apperror code of err, or the empty code when err is not an AppError.
func codeOf(err error) apperror.Code {
	var appErr *apperror.AppError
	if errors.As(err, &appErr) {
		return appErr.Code
	}
	return ""
}

// assertCode fails the test unless err carries the wanted code.
func assertCode(t *testing.T, err error, want apperror.Code) {
	t.Helper()
	if got := codeOf(err); got != want {
		t.Fatalf("error = %v, want code %s", err, want)
	}
}

// countingEvaluator records every evaluation the service starts. done receives each interview id as it
// is evaluated, so a test can wait for the background call without sleeping.
type countingEvaluator struct {
	done chan uuid.UUID
}

func newCountingEvaluator() *countingEvaluator {
	return &countingEvaluator{done: make(chan uuid.UUID, 16)}
}

func (e *countingEvaluator) Evaluate(_ context.Context, interviewID uuid.UUID) error {
	e.done <- interviewID
	return nil
}

// avatarOf gives the user an inventory and one avatar in it, and returns the avatar id.
func (s *seed) avatarOf(userID uuid.UUID) uuid.UUID {
	s.t.Helper()
	id := uuid.New()
	s.exec(`INSERT INTO public.inventories (user_id, slot) VALUES ($1, 1) ON CONFLICT (user_id) DO NOTHING`, userID)
	s.exec(`INSERT INTO public.avatars (id, inventory_id, name, model_path, personality)
		SELECT $2, inv.id, 'Test avatar', 'models/test.vrm', 'calm' FROM public.inventories inv WHERE inv.user_id = $1`, userID, id)
	return id
}

// voice creates a voice, active or not, and returns its id.
func (s *seed) voice(active bool) uuid.UUID {
	s.t.Helper()
	id := uuid.New()
	s.exec(`INSERT INTO public.voices (id, name, is_active) VALUES ($1, 'Test voice', $2)`, id, active)
	s.voices = append(s.voices, id.String())
	return id
}
