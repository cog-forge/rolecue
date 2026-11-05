//go:build integration

// Package tests: account erasure and data export against a real Postgres. The repository and the
// service run unmodified; only the object store is a recording fake. Every test seeds its own users and
// removes them afterwards through the same erase path.
package tests

import (
	"context"
	"fmt"
	"os"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/cog-forge/rolecue/api/internal/features/account/repository"
	"github.com/cog-forge/rolecue/api/internal/features/account/service"
	authdomain "github.com/cog-forge/rolecue/api/internal/features/auth/domain"
	"github.com/cog-forge/rolecue/api/internal/pkg/storage"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func openPool(t *testing.T) (context.Context, *pgxpool.Pool) {
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

// recordingStore records deleted keys and signs links without a network.
type recordingStore struct {
	mu      sync.Mutex
	deleted []string
	ttls    []time.Duration
}

func (s *recordingStore) Delete(_ context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.deleted = append(s.deleted, key)
	return nil
}

func (s *recordingStore) PresignGetWithOverrides(_ context.Context, key string, ttl time.Duration, _ storage.GetOverrides) (storage.SignedRequest, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ttls = append(s.ttls, ttl)
	return storage.SignedRequest{URL: "https://store.test/" + key, Method: "GET", ExpiresAt: time.Now().Add(ttl)}, nil
}

type fixture struct {
	t     *testing.T
	ctx   context.Context
	pool  *pgxpool.Pool
	repo  *repository.Repository
	store *recordingStore
	svc   *service.Service
	users []uuid.UUID
}

func newFixture(t *testing.T) *fixture {
	ctx, pool := openPool(t)
	f := &fixture{t: t, ctx: ctx, pool: pool, repo: repository.NewRepository(pool), store: &recordingStore{}}
	f.svc = service.NewService(f.repo, f.store)
	t.Cleanup(f.cleanup)
	return f
}

func (f *fixture) cleanup() {
	ctx := context.Background()
	for _, id := range f.users {
		_, _ = f.pool.Exec(ctx, `UPDATE public.job_postings SET status = 'closed' WHERE recruiter_id = $1`, id)
		_, _ = f.pool.Exec(ctx, `UPDATE public.applications SET status = 'rejected'
 WHERE job_posting_id IN (SELECT id FROM public.job_postings WHERE recruiter_id = $1)`, id)
		if err := f.repo.Erase(ctx, id); err != nil {
			f.t.Errorf("cleanup erase %s: %v", id, err)
		}
	}
}

func (f *fixture) exec(sql string, args ...any) {
	f.t.Helper()
	if _, err := f.pool.Exec(f.ctx, sql, args...); err != nil {
		f.t.Fatalf("seed %q: %v", sql, err)
	}
}

func (f *fixture) count(sql string, args ...any) int {
	f.t.Helper()
	var n int
	if err := f.pool.QueryRow(f.ctx, sql, args...).Scan(&n); err != nil {
		f.t.Fatal(err)
	}
	return n
}

func (f *fixture) user(role string) authdomain.User {
	id := uuid.New()
	email := fmt.Sprintf("account-%s@example.test", id)
	f.exec(`INSERT INTO public.users (id, name, email, role, email_verified) VALUES ($1, 'account test', $2, $3, true)`, id, email, role)
	f.users = append(f.users, id)
	return authdomain.User{ID: id, Email: email, Role: role, EmailVerified: true}
}

// posting seeds a Job Posting of the recruiter with a JD whose PDF key is jdKey.
func (f *fixture) posting(recruiter uuid.UUID, status, jdKey string) uuid.UUID {
	jd, posting := uuid.New(), uuid.New()
	f.exec(`INSERT INTO public.job_descriptions (id, content_path) VALUES ($1, $2)`, jd, jdKey)
	f.exec(`INSERT INTO public.job_postings (id, status, jd_id, recruiter_id, title) VALUES ($1, $2::public.job_posting_status, $3, $4, 'Go developer')`, posting, status, jd, recruiter)
	return posting
}

// application seeds an Application of the candidate and its recruitment interview with one answered turn.
func (f *fixture) application(candidate, posting uuid.UUID, status, cvKey, recordKey string) (app, interview uuid.UUID) {
	app, interview = uuid.New(), uuid.New()
	f.exec(`INSERT INTO public.applications (id, job_posting_id, candidate_id, status, cv_path) VALUES ($1, $2, $3, $4::public.application_status, $5)`, app, posting, candidate, status, cvKey)
	f.exec(`INSERT INTO public.interviews (id, type, application_id, record_path, record_status, score, feedback)
 VALUES ($1, 'recruitment', $2, $3, 'ready', 77, 'recruiter-only feedback')`, interview, app, recordKey)
	f.exec(`INSERT INTO public.conversation_turns (interview_id, position, question, answer, feedback) VALUES ($1, 1, 'Tell me about Go', 'secret answer', 'turn feedback')`, interview)
	return app, interview
}

// practice seeds a Role Profile of the candidate, a practice interview on it with a turn and a score.
func (f *fixture) practice(candidate uuid.UUID, jdKey string) uuid.UUID {
	jd, rp, interview := uuid.New(), uuid.New(), uuid.New()
	f.exec(`INSERT INTO public.job_descriptions (id, content_path) VALUES ($1, $2)`, jd, jdKey)
	f.exec(`INSERT INTO public.role_profiles (id, jd_id, candidate_id, title) VALUES ($1, $2, $3, 'Practice role')`, rp, jd, candidate)
	f.exec(`INSERT INTO public.interviews (id, type, role_profile_id, score, feedback) VALUES ($1, 'practice', $2, 81, 'practice feedback')`, interview, rp)
	f.exec(`INSERT INTO public.conversation_turns (interview_id, position, question, answer, feedback) VALUES ($1, 1, 'Q', 'practice answer', 'F')`, interview)
	f.exec(`INSERT INTO public.score_details (interview_id, metric_id, score, evidence) SELECT $1, id, 80, 'quote' FROM public.metrics LIMIT 1`, interview)
	return interview
}

// belongings gives the candidate an avatar, a wallet and an issued upload grant, returning the object keys.
func (f *fixture) belongings(user uuid.UUID, modelKey string) (grantKey string) {
	inv := uuid.New()
	f.exec(`INSERT INTO public.inventories (id, user_id, slot) VALUES ($1, $2, 1)`, inv, user)
	f.exec(`INSERT INTO public.avatars (inventory_id, name, model_path, personality) VALUES ($1, 'Avatar', $2, 'calm')`, inv, modelKey)
	f.exec(`INSERT INTO public.wallets (user_id, balance) VALUES ($1, 10)`, user)
	grant := uuid.New()
	f.exec(`INSERT INTO public.upload_grants (key, owner_id, purpose) VALUES ($1, $2, 'role_profile_pdf')`, grant, user)
	return grant.String()
}

type owned struct{ apps, interviews, turns, scores, profiles, avatars, wallets, grants, users int }

func (f *fixture) owned(user uuid.UUID) owned {
	return owned{
		apps:       f.count(`SELECT count(*) FROM public.applications WHERE candidate_id = $1`, user),
		interviews: f.count(`SELECT count(*) FROM public.interviews i LEFT JOIN public.applications a ON a.id = i.application_id LEFT JOIN public.role_profiles rp ON rp.id = i.role_profile_id WHERE a.candidate_id = $1 OR rp.candidate_id = $1`, user),
		turns:      f.count(`SELECT count(*) FROM public.conversation_turns t JOIN public.interviews i ON i.id = t.interview_id LEFT JOIN public.applications a ON a.id = i.application_id LEFT JOIN public.role_profiles rp ON rp.id = i.role_profile_id WHERE a.candidate_id = $1 OR rp.candidate_id = $1`, user),
		scores:     f.count(`SELECT count(*) FROM public.score_details s JOIN public.interviews i ON i.id = s.interview_id LEFT JOIN public.role_profiles rp ON rp.id = i.role_profile_id WHERE rp.candidate_id = $1`, user),
		profiles:   f.count(`SELECT count(*) FROM public.role_profiles WHERE candidate_id = $1`, user),
		avatars:    f.count(`SELECT count(*) FROM public.avatars av JOIN public.inventories inv ON inv.id = av.inventory_id WHERE inv.user_id = $1`, user),
		wallets:    f.count(`SELECT count(*) FROM public.wallets WHERE user_id = $1`, user),
		grants:     f.count(`SELECT count(*) FROM public.upload_grants WHERE owner_id = $1`, user),
		users:      f.count(`SELECT count(*) FROM public.users WHERE id = $1`, user),
	}
}

func sorted(in []string) []string {
	out := append([]string(nil), in...)
	sort.Strings(out)
	return out
}

func TestEraseCandidateCascadesAndLeavesOthersUntouched(t *testing.T) {
	f := newFixture(t)
	recruiter, alice, bob := f.user("recruiter"), f.user("candidate"), f.user("candidate")
	posting := f.posting(recruiter.ID, "open", uuid.NewString())

	aliceCV, aliceRec, aliceJD, aliceModel := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	f.application(alice.ID, posting, "interviewed", aliceCV, aliceRec)
	f.practice(alice.ID, aliceJD)
	grant := f.belongings(alice.ID, aliceModel)

	f.application(bob.ID, posting, "pending", uuid.NewString(), uuid.NewString())
	f.practice(bob.ID, uuid.NewString())
	f.belongings(bob.ID, uuid.NewString())
	bobBefore := f.owned(bob.ID)
	if bobBefore.apps != 1 || bobBefore.interviews != 2 || bobBefore.turns != 2 || bobBefore.scores != 1 {
		t.Fatalf("seed is wrong: %+v", bobBefore)
	}

	if err := f.svc.Erase(f.ctx, alice, "DELETE"); err != nil {
		t.Fatal(err)
	}

	if got := f.owned(alice.ID); got != (owned{}) {
		t.Fatalf("alice still owns rows: %+v", got)
	}
	if got := f.owned(bob.ID); got != bobBefore {
		t.Fatalf("bob changed: before %+v after %+v", bobBefore, got)
	}
	if f.count(`SELECT count(*) FROM public.job_postings WHERE id = $1`, posting) != 1 {
		t.Fatal("the recruiter's posting was deleted")
	}
	want := sorted([]string{aliceCV, aliceRec, aliceJD, aliceModel, grant})
	if got := sorted(f.store.deleted); fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("deleted keys = %v, want exactly alice's %v", got, want)
	}
	// Erasing an account that is already gone succeeds, so a retried request is safe.
	if err := f.repo.Erase(f.ctx, alice.ID); err != nil {
		t.Fatalf("second erase: %v", err)
	}
}

func TestEraseRecruiterRefusedWhileWorkIsActive(t *testing.T) {
	f := newFixture(t)
	recruiter, carol := f.user("recruiter"), f.user("candidate")
	jdKey, cvKey := uuid.NewString(), uuid.NewString()
	posting := f.posting(recruiter.ID, "open", jdKey)
	app, _ := f.application(carol.ID, posting, "pending", cvKey, uuid.NewString())

	err := f.svc.Erase(f.ctx, recruiter, "DELETE")
	if !apperror.IsCode(err, apperror.CodeAccountHasActiveWork) {
		t.Fatalf("erase = %v, want ACCOUNT_HAS_ACTIVE_WORK", err)
	}
	if len(f.store.deleted) != 0 || f.count(`SELECT count(*) FROM public.users WHERE id = $1`, recruiter.ID) != 1 {
		t.Fatalf("a refused recruiter lost data: deleted %v", f.store.deleted)
	}

	// Once the posting is closed and the application decided nothing is in flight.
	f.exec(`UPDATE public.job_postings SET status = 'closed' WHERE id = $1`, posting)
	f.exec(`UPDATE public.applications SET status = 'rejected' WHERE id = $1`, app)
	if err := f.svc.Erase(f.ctx, recruiter, "DELETE"); err != nil {
		t.Fatal(err)
	}
	if f.count(`SELECT count(*) FROM public.job_postings WHERE id = $1`, posting) != 0 {
		t.Fatal("the posting survived")
	}
	if f.count(`SELECT count(*) FROM public.users WHERE id = $1`, carol.ID) != 1 {
		t.Fatal("the other candidate's account was deleted")
	}
	if got := sorted(f.store.deleted); fmt.Sprint(got) != fmt.Sprint(sorted([]string{jdKey, cvKey, f.recordKeyOf(app)})) {
		t.Fatalf("deleted keys = %v", got)
	}
}

func (f *fixture) recordKeyOf(app uuid.UUID) string {
	var key string
	if err := f.pool.QueryRow(f.ctx, `SELECT record_path FROM public.interviews WHERE application_id = $1`, app).Scan(&key); err != nil {
		f.t.Fatal(err)
	}
	return key
}

func TestEraseRefusesAdmin(t *testing.T) {
	f := newFixture(t)
	admin := f.user("admin")
	if err := f.svc.Erase(f.ctx, admin, "DELETE"); !apperror.IsCode(err, apperror.CodeAdminCannotSelfDelete) {
		t.Fatalf("erase = %v, want ADMIN_CANNOT_SELF_DELETE", err)
	}
	if f.count(`SELECT count(*) FROM public.users WHERE id = $1`, admin.ID) != 1 {
		t.Fatal("admin was deleted")
	}
}

func TestExportIsOwnerScopedAndHidesRecruitmentScores(t *testing.T) {
	f := newFixture(t)
	recruiter, alice, bob := f.user("recruiter"), f.user("candidate"), f.user("candidate")
	posting := f.posting(recruiter.ID, "open", uuid.NewString())
	aliceApp, aliceInterview := f.application(alice.ID, posting, "interviewed", "alice-cv", "alice-rec")
	practice := f.practice(alice.ID, uuid.NewString())
	bobApp, bobInterview := f.application(bob.ID, posting, "pending", "bob-cv", "bob-rec")

	out, err := f.svc.Export(f.ctx, alice)
	if err != nil {
		t.Fatal(err)
	}
	if out.Profile.ID != alice.ID || out.Profile.Email != alice.Email {
		t.Fatalf("profile = %+v", out.Profile)
	}
	if len(out.Applications) != 1 || out.Applications[0].ID != aliceApp || out.Applications[0].CV == nil || out.Applications[0].CV.URL != "https://store.test/alice-cv" {
		t.Fatalf("applications = %+v", out.Applications)
	}
	if len(out.Interviews) != 2 {
		t.Fatalf("interviews = %+v", out.Interviews)
	}
	for _, i := range out.Interviews {
		if i.ID == bobInterview {
			t.Fatal("bob's interview leaked")
		}
		if len(i.Turns) != 1 {
			t.Fatalf("turns = %+v", i.Turns)
		}
		switch i.ID {
		case aliceInterview:
			if i.Score != nil || i.Feedback != nil || len(i.Scores) != 0 || i.Turns[0].Feedback != nil {
				t.Fatalf("recruitment interview exposes recruiter-side results: %+v", i)
			}
			if i.Turns[0].Answer == nil || *i.Turns[0].Answer != "secret answer" || i.Recording == nil {
				t.Fatalf("recruitment interview = %+v", i)
			}
		case practice:
			if i.Score == nil || *i.Score != 81 || len(i.Scores) != 1 || i.Turns[0].Feedback == nil || i.Recording != nil {
				t.Fatalf("practice interview = %+v", i)
			}
		default:
			t.Fatalf("unexpected interview %s", i.ID)
		}
	}
	for _, ttl := range f.store.ttls {
		if ttl != 15*time.Minute {
			t.Fatalf("link ttl = %v", ttl)
		}
	}
	if bobApp == aliceApp {
		t.Fatal("seed collision")
	}

	// A recruiter exports profile and postings only.
	rout, err := f.svc.Export(f.ctx, recruiter)
	if err != nil {
		t.Fatal(err)
	}
	if len(rout.Postings) != 1 || len(rout.Applications) != 0 || len(rout.Interviews) != 0 {
		t.Fatalf("recruiter export = %+v", rout)
	}
}
