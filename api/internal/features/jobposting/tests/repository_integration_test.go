//go:build integration

// Package tests holds the slow, opt-in tests of the Job Posting feature. They need a migrated
// PostgreSQL database:
//
//	cd api && set -a && . ../.env && set +a
//	go test -tags=integration ./internal/features/jobposting/tests -count=1
package tests

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/cog-forge/rolecue/api/internal/features/jobposting/domain"
	"github.com/cog-forge/rolecue/api/internal/features/jobposting/repository"
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

// seed inserts tagged rows and deletes exactly those rows afterwards, in reverse order.
type seed struct {
	t    *testing.T
	ctx  context.Context
	pool *pgxpool.Pool
	tag  string
	undo []func()
}

func newSeed(t *testing.T, ctx context.Context, pool *pgxpool.Pool) *seed {
	s := &seed{t: t, ctx: ctx, pool: pool, tag: uuid.NewString()[:8]}
	t.Cleanup(func() {
		for i := len(s.undo) - 1; i >= 0; i-- {
			s.undo[i]()
		}
	})
	return s
}

func (s *seed) exec(sql string, args ...any) {
	s.t.Helper()
	if _, err := s.pool.Exec(s.ctx, sql, args...); err != nil {
		s.t.Fatalf("seed %q: %v", sql, err)
	}
}

func (s *seed) later(sql string, args ...any) {
	s.undo = append(s.undo, func() {
		if _, err := s.pool.Exec(context.Background(), sql, args...); err != nil {
			s.t.Errorf("cleanup %q: %v", sql, err)
		}
	})
}

// levels adds any seniority level the database is missing (a DB not yet migrated past 000006
// has none) and removes only the rows it added.
func (s *seed) levels() {
	for _, name := range []string{"intern", "junior", "mid", "senior", "lead"} {
		id := uuid.New()
		tag, err := s.pool.Exec(s.ctx, `INSERT INTO public.levels (id, name) SELECT $1, $2::text
			WHERE NOT EXISTS (SELECT 1 FROM public.levels WHERE lower(name) = $2::text)`, id, name)
		if err != nil {
			s.t.Fatal(err)
		}
		if tag.RowsAffected() == 1 {
			s.later(`DELETE FROM public.levels WHERE id = $1`, id)
		}
	}
}

func (s *seed) user(role string) uuid.UUID {
	id := uuid.New()
	s.exec(`INSERT INTO public.users (id, name, email, role, email_verified) VALUES ($1, $2, $3, $4, true)`,
		id, "jp "+role, fmt.Sprintf("jp-%s-%s@example.test", s.tag, id), role)
	s.later(`DELETE FROM public.users WHERE id = $1`, id)
	return id
}

func (s *seed) avatar(ownerID uuid.UUID) uuid.UUID {
	inventory, avatar := uuid.New(), uuid.New()
	s.exec(`INSERT INTO public.inventories (id, user_id, slot) VALUES ($1, $2, 3)`, inventory, ownerID)
	s.later(`DELETE FROM public.inventories WHERE id = $1`, inventory)
	s.exec(`INSERT INTO public.avatars (id, inventory_id, name, model_path, personality) VALUES ($1, $2, 'Ava', 'm.vrm', 'calm')`, avatar, inventory)
	s.later(`DELETE FROM public.avatars WHERE id = $1`, avatar)
	return avatar
}

func (s *seed) voice(active bool) uuid.UUID {
	id := uuid.New()
	s.exec(`INSERT INTO public.voices (id, name, is_active) VALUES ($1, 'Voice '||$2::text, $3)`, id, s.tag, active)
	s.later(`DELETE FROM public.voices WHERE id = $1`, id)
	return id
}

// track removes a posting created through the repository, with its JD rows.
func (s *seed) track(postingID uuid.UUID) {
	var jdID uuid.UUID
	if err := s.pool.QueryRow(s.ctx, `SELECT jd_id FROM public.job_postings WHERE id = $1`, postingID).Scan(&jdID); err != nil {
		s.t.Fatal(err)
	}
	s.later(`DELETE FROM public.job_descriptions WHERE id = $1`, jdID)
	s.later(`DELETE FROM public.requirements WHERE jd_id = $1`, jdID)
	s.later(`DELETE FROM public.core_questions WHERE jd_id = $1`, jdID)
	s.later(`DELETE FROM public.job_postings WHERE id = $1`, postingID)
}

func codeOf(err error) apperror.Code {
	var appErr *apperror.AppError
	if errors.As(err, &appErr) {
		return appErr.Code
	}
	return ""
}

func ptr[T any](v T) *T { return &v }

type fixture struct {
	repo      *repository.Repository
	seed      *seed
	recruiter uuid.UUID
	avatar    uuid.UUID
	voice     uuid.UUID
}

func newFixture(t *testing.T) (context.Context, fixture) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	s.levels()
	recruiter := s.user("recruiter")
	return ctx, fixture{repo: repository.NewRepository(pool), seed: s, recruiter: recruiter, avatar: s.avatar(recruiter), voice: s.voice(true)}
}

func (f fixture) create(t *testing.T, ctx context.Context, title string) uuid.UUID {
	t.Helper()
	record, err := f.repo.Create(ctx, f.recruiter, domain.NewPosting{ContentPath: "jp/" + f.seed.tag, CreateInput: domain.CreateInput{
		Title: title, SeniorityLevel: domain.Senior, Location: ptr("Ho Chi Minh City"), SalaryMin: ptr(int32(1000)),
		SalaryMax: ptr(int32(2000)), InterviewDuration: ptr(int32(30)), RequiredAvatarID: f.avatar, RequiredVoiceID: f.voice,
		Requirements: []domain.PostingRequirement{
			{Name: "Go", Target: "programming_language", Type: domain.Required},
			{Name: "Kafka", Target: "technologies", Type: domain.Preferred},
		},
	}})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	f.seed.track(record.ID)
	return record.ID
}

func TestCreateAndGet(t *testing.T) {
	ctx, f := newFixture(t)
	id := f.create(t, ctx, "Backend Engineer")

	got, err := f.repo.Get(ctx, f.recruiter, id)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != domain.StatusPending || got.SeniorityLevel != domain.Senior || got.ContentPath != "jp/"+f.seed.tag {
		t.Fatalf("status=%s level=%s path=%s", got.Status, got.SeniorityLevel, got.ContentPath)
	}
	if len(got.Requirements) != 2 || got.Requirements[0].Name != "Go" || got.Requirements[1].Type != domain.Preferred {
		t.Fatalf("requirements = %+v", got.Requirements)
	}
	if got.RequiredAvatar == nil || got.RequiredAvatar.ID != f.avatar || got.RequiredVoice == nil || got.RequiredVoice.ID != f.voice {
		t.Fatalf("refs avatar=%+v voice=%+v", got.RequiredAvatar, got.RequiredVoice)
	}
	if got.InterviewSlot != nil || got.CoreQuestionCount != 0 || *got.SalaryMax != 2000 {
		t.Fatalf("slot=%v questions=%d salary_max=%v", got.InterviewSlot, got.CoreQuestionCount, got.SalaryMax)
	}
}

func TestCreateRejectsForeignAvatarAndInactiveVoice(t *testing.T) {
	ctx, f := newFixture(t)
	other := f.seed.user("recruiter")
	for name, input := range map[string]domain.CreateInput{
		"foreign avatar": {RequiredAvatarID: f.seed.avatar(other), RequiredVoiceID: f.voice},
		"missing avatar": {RequiredAvatarID: uuid.New(), RequiredVoiceID: f.voice},
		"inactive voice": {RequiredAvatarID: f.avatar, RequiredVoiceID: f.seed.voice(false)},
		"missing voice":  {RequiredAvatarID: f.avatar, RequiredVoiceID: uuid.New()},
	} {
		t.Run(name, func(t *testing.T) {
			input.Title, input.SeniorityLevel = "Rejected", domain.Mid
			input.Requirements = []domain.PostingRequirement{{Name: "Go", Target: "tool", Type: domain.Required}}
			_, err := f.repo.Create(ctx, f.recruiter, domain.NewPosting{CreateInput: input, ContentPath: "x"})
			if codeOf(err) != apperror.CodeValidation {
				t.Fatalf("error = %v, want VALIDATION_ERROR", err)
			}
		})
	}
	var count int
	if err := f.seed.pool.QueryRow(ctx, `SELECT count(*) FROM public.job_postings WHERE recruiter_id = $1`, f.recruiter).Scan(&count); err != nil || count != 0 {
		t.Fatalf("rejected creates left %d postings (err=%v)", count, err)
	}
}

func TestOtherRecruiterSeesNothing(t *testing.T) {
	ctx, f := newFixture(t)
	id := f.create(t, ctx, "Private posting")
	other := f.seed.user("recruiter")

	if _, err := f.repo.Get(ctx, other, id); codeOf(err) != apperror.CodeJobPostingNotFound {
		t.Fatalf("get by another recruiter: %v", err)
	}
	err := f.repo.Update(ctx, other, id, domain.Edit{Title: domain.StringChange{Present: true, Value: ptr("Hijacked")}}, nil)
	if codeOf(err) != apperror.CodeJobPostingNotFound {
		t.Fatalf("update by another recruiter: %v", err)
	}
	items, total, err := f.repo.List(ctx, other, domain.ListFilter{Limit: 20})
	if err != nil || total != 0 || len(items) != 0 {
		t.Fatalf("list by another recruiter: total=%d err=%v", total, err)
	}
}

func TestListFiltersAndSearch(t *testing.T) {
	ctx, f := newFixture(t)
	f.create(t, ctx, "Go Engineer 100% remote")
	second := f.create(t, ctx, "Data Engineer")
	f.seed.exec(`UPDATE public.job_postings SET status = 'open' WHERE id = $1`, second)

	for _, tt := range []struct {
		name   string
		filter domain.ListFilter
		want   int64
	}{
		{"all", domain.ListFilter{Limit: 20}, 2},
		{"open only", domain.ListFilter{Status: ptr(domain.StatusOpen), Limit: 20}, 1},
		{"case-insensitive search", domain.ListFilter{Search: ptr("data"), Limit: 20}, 1},
		// The service escapes wildcards; here the escaped form must match the literal percent sign.
		{"escaped percent", domain.ListFilter{Search: ptr(`100\%`), Limit: 20}, 1},
		{"escaped underscore", domain.ListFilter{Search: ptr(`\_`), Limit: 20}, 0},
		{"page size", domain.ListFilter{Limit: 1}, 2},
	} {
		t.Run(tt.name, func(t *testing.T) {
			items, total, err := f.repo.List(ctx, f.recruiter, tt.filter)
			if err != nil || total != tt.want {
				t.Fatalf("total=%d err=%v", total, err)
			}
			if int64(len(items)) != min(tt.want, int64(tt.filter.Limit)) {
				t.Fatalf("items=%d", len(items))
			}
		})
	}
}

func TestUpdate(t *testing.T) {
	ctx, f := newFixture(t)
	// Seeded before the posting, so cleanup removes the posting first.
	newVoice := f.seed.voice(true)
	id := f.create(t, ctx, "Backend Engineer")

	err := f.repo.Update(ctx, f.recruiter, id, domain.Edit{
		Title:             domain.StringChange{Present: true, Value: ptr("Senior Backend Engineer")},
		Location:          domain.StringChange{Present: true},
		SalaryMax:         domain.Int32Change{Present: true, Value: ptr(int32(3000))},
		SeniorityLevel:    domain.StringChange{Present: true, Value: ptr("lead")},
		RequiredVoiceID:   domain.UUIDChange{Present: true, Value: &newVoice},
		InterviewDuration: domain.Int32Change{Present: true, Value: ptr(int32(45))},
		Requirements: domain.RequirementsChange{Present: true, Value: &[]domain.PostingRequirement{
			{Name: "Rust", Target: "programming_language", Type: domain.Required},
		}},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	got, err := f.repo.Get(ctx, f.recruiter, id)
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "Senior Backend Engineer" || got.Location != nil || *got.SalaryMax != 3000 || *got.SalaryMin != 1000 {
		t.Fatalf("title=%q location=%v salary=%v-%v", got.Title, got.Location, *got.SalaryMin, *got.SalaryMax)
	}
	if got.SeniorityLevel != domain.Lead || got.RequiredVoice.ID != newVoice || got.RequiredAvatar.ID != f.avatar || *got.InterviewDuration != 45 {
		t.Fatalf("level=%s voice=%v avatar=%v duration=%v", got.SeniorityLevel, got.RequiredVoice.ID, got.RequiredAvatar.ID, *got.InterviewDuration)
	}
	if len(got.Requirements) != 1 || got.Requirements[0].Name != "Rust" {
		t.Fatalf("requirements = %+v", got.Requirements)
	}

	// An empty edit changes nothing and succeeds.
	if err := f.repo.Update(ctx, f.recruiter, id, domain.Edit{}, nil); err != nil {
		t.Fatalf("empty edit: %v", err)
	}
}

func TestUpdateGuards(t *testing.T) {
	ctx, f := newFixture(t)
	other := f.seed.user("recruiter")

	t.Run("salary range after merge", func(t *testing.T) {
		id := f.create(t, ctx, "Salary")
		err := f.repo.Update(ctx, f.recruiter, id, domain.Edit{SalaryMin: domain.Int32Change{Present: true, Value: ptr(int32(5000))}}, nil)
		if !errors.Is(err, domain.ErrSalaryRange) {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("foreign avatar", func(t *testing.T) {
		id, foreign := f.create(t, ctx, "Avatar"), f.seed.avatar(other)
		err := f.repo.Update(ctx, f.recruiter, id, domain.Edit{RequiredAvatarID: domain.UUIDChange{Present: true, Value: &foreign}}, nil)
		if !errors.Is(err, domain.ErrAvatarNotOwned) {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("requirements locked by question bank", func(t *testing.T) {
		id := f.create(t, ctx, "Locked")
		f.seed.exec(`INSERT INTO public.core_questions (jd_id, content) SELECT jd_id, 'Explain goroutines.' FROM public.job_postings WHERE id = $1`, id)
		err := f.repo.Update(ctx, f.recruiter, id, domain.Edit{SeniorityLevel: domain.StringChange{Present: true, Value: ptr("junior")}}, nil)
		if !errors.Is(err, domain.ErrRequirementsLocked) {
			t.Fatalf("seniority edit: %v", err)
		}
		// Resending the stored values, as a whole-form submit does, is not a change.
		same := domain.Edit{
			SeniorityLevel: domain.StringChange{Present: true, Value: ptr("senior")},
			Requirements: domain.RequirementsChange{Present: true, Value: &[]domain.PostingRequirement{
				{Name: "Kafka", Target: "technologies", Type: domain.Preferred},
				{Name: "Go", Target: "programming_language", Type: domain.Required},
			}},
		}
		if err := f.repo.Update(ctx, f.recruiter, id, same, nil); err != nil {
			t.Fatalf("unchanged resend: %v", err)
		}
		changed := same
		changed.Requirements = domain.RequirementsChange{Present: true, Value: &[]domain.PostingRequirement{
			{Name: "Go", Target: "programming_language", Type: domain.Preferred},
			{Name: "Kafka", Target: "technologies", Type: domain.Preferred},
		}}
		if err := f.repo.Update(ctx, f.recruiter, id, changed, nil); !errors.Is(err, domain.ErrRequirementsLocked) {
			t.Fatalf("changed requirement type: %v", err)
		}
		// Other fields stay editable while the bank exists.
		if err := f.repo.Update(ctx, f.recruiter, id, domain.Edit{Title: domain.StringChange{Present: true, Value: ptr("Still pending")}}, nil); err != nil {
			t.Fatalf("title edit: %v", err)
		}
	})
	t.Run("not pending", func(t *testing.T) {
		id := f.create(t, ctx, "Approved")
		f.seed.exec(`UPDATE public.job_postings SET status = 'open' WHERE id = $1`, id)
		err := f.repo.Update(ctx, f.recruiter, id, domain.Edit{Title: domain.StringChange{Present: true, Value: ptr("Too late")}}, nil)
		if codeOf(err) != apperror.CodeJobPostingNotEditable {
			t.Fatalf("error = %v", err)
		}
		got, _ := f.repo.Get(ctx, f.recruiter, id)
		if got.Title != "Approved" {
			t.Fatal("an open posting was edited")
		}
	})
}

func TestCreateReturnsStoredPosting(t *testing.T) {
	ctx, f := newFixture(t)
	record, err := f.repo.Create(ctx, f.recruiter, domain.NewPosting{ContentPath: "jp/x", CreateInput: domain.CreateInput{
		Title: "Returned", SeniorityLevel: domain.Junior, RequiredAvatarID: f.avatar, RequiredVoiceID: f.voice,
		Requirements: []domain.PostingRequirement{{Name: "Go", Target: "tool", Type: domain.Required}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	f.seed.track(record.ID)
	if record.Status != domain.StatusPending || record.CreatedAt.IsZero() || record.RequiredAvatar.Name == nil ||
		*record.RequiredAvatar.Name != "Ava" || record.RequiredVoice.Name == nil {
		t.Fatalf("record = %+v", record.JobPosting)
	}
}

// A question-bank save that holds the posting lock and commits a question while an edit waits for
// the lock must make that edit fail: the count is read after the lock, not in the locking query.
func TestEditWaitingOnBankSaveSeesNewQuestions(t *testing.T) {
	ctx, f := newFixture(t)
	id := f.create(t, ctx, "Racing")

	tx, err := f.seed.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	var jdID uuid.UUID
	if err := tx.QueryRow(ctx, `SELECT jd_id FROM public.job_postings WHERE id = $1 FOR UPDATE`, id).Scan(&jdID); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO public.core_questions (jd_id, content) VALUES ($1, 'Explain channels.')`, jdID); err != nil {
		t.Fatal(err)
	}

	done := make(chan error, 1)
	go func() {
		done <- f.repo.Update(ctx, f.recruiter, id, domain.Edit{SeniorityLevel: domain.StringChange{Present: true, Value: ptr("intern")}}, nil)
	}()
	select {
	case err := <-done:
		t.Fatalf("the edit did not wait for the lock: %v", err)
	case <-time.After(300 * time.Millisecond):
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err := <-done; !errors.Is(err, domain.ErrRequirementsLocked) {
		t.Fatalf("edit after the bank was saved: %v", err)
	}
}

// An Admin deactivating a voice locks it FOR UPDATE. A posting that selects the voice meanwhile
// must wait for the Admin and then see it inactive, for both create and edit.
func TestVoiceSelectionWaitsForAdminDeactivation(t *testing.T) {
	ctx, f := newFixture(t)
	// Seeded before any posting, so cleanup removes the postings first.
	forCreate, forEdit := f.seed.voice(true), f.seed.voice(true)
	id := f.create(t, ctx, "Voice race")

	for _, tc := range []struct {
		name  string
		voice uuid.UUID
		write func() error
	}{
		{"create", forCreate, func() error {
			record, err := f.repo.Create(ctx, f.recruiter, domain.NewPosting{ContentPath: "jp/race", CreateInput: domain.CreateInput{
				Title: "Raced", SeniorityLevel: domain.Mid, RequiredAvatarID: f.avatar, RequiredVoiceID: forCreate,
				Requirements: []domain.PostingRequirement{{Name: "Go", Target: "tool", Type: domain.Required}},
			}})
			if err == nil {
				f.seed.track(record.ID)
			}
			return err
		}},
		{"edit", forEdit, func() error {
			return f.repo.Update(ctx, f.recruiter, id, domain.Edit{RequiredVoiceID: domain.UUIDChange{Present: true, Value: &forEdit}}, nil)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			admin, err := f.seed.pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = admin.Rollback(context.Background()) }()
			if _, err := admin.Exec(ctx, `SELECT id FROM public.voices WHERE id = $1 FOR UPDATE`, tc.voice); err != nil {
				t.Fatal(err)
			}
			if _, err := admin.Exec(ctx, `UPDATE public.voices SET is_active = false WHERE id = $1`, tc.voice); err != nil {
				t.Fatal(err)
			}
			done := make(chan error, 1)
			go func() { done <- tc.write() }()
			select {
			case err := <-done:
				t.Fatalf("the voice check did not wait for the Admin: %v", err)
			case <-time.After(300 * time.Millisecond):
			}
			if err := admin.Commit(ctx); err != nil {
				t.Fatal(err)
			}
			if err := <-done; !errors.Is(err, domain.ErrVoiceNotActive) {
				t.Fatalf("after deactivation: %v", err)
			}
		})
	}
}

// Deactivation blocks only new selections: resending the voice a posting already has still works.
func TestUnchangedVoiceSurvivesDeactivation(t *testing.T) {
	ctx, f := newFixture(t)
	inactive := f.seed.voice(false)
	id := f.create(t, ctx, "Keeps its voice")
	f.seed.exec(`UPDATE public.voices SET is_active = false WHERE id = $1`, f.voice)

	err := f.repo.Update(ctx, f.recruiter, id, domain.Edit{
		Title:           domain.StringChange{Present: true, Value: ptr("Renamed")},
		RequiredVoiceID: domain.UUIDChange{Present: true, Value: &f.voice},
	}, nil)
	if err != nil {
		t.Fatalf("unchanged voice: %v", err)
	}
	got, _ := f.repo.Get(ctx, f.recruiter, id)
	if got.Title != "Renamed" || got.RequiredVoice.ID != f.voice {
		t.Fatalf("title=%q voice=%v", got.Title, got.RequiredVoice.ID)
	}
	err = f.repo.Update(ctx, f.recruiter, id, domain.Edit{RequiredVoiceID: domain.UUIDChange{Present: true, Value: &inactive}}, nil)
	if !errors.Is(err, domain.ErrVoiceNotActive) {
		t.Fatalf("new inactive voice: %v", err)
	}
}

func TestRejectedPostingCorrectionAndResubmit(t *testing.T) {
	ctx, f := newFixture(t)
	other := f.seed.user("recruiter")
	id := f.create(t, ctx, "Rejected")
	f.seed.exec(`UPDATE public.job_postings SET status = 'rejected', reject_reason = 'Too vague' WHERE id = $1`, id)
	question := func() {
		f.seed.exec(`INSERT INTO public.core_questions (jd_id, content) SELECT jd_id, 'Explain goroutines.' FROM public.job_postings WHERE id = $1`, id)
	}
	get := func() domain.Record {
		got, err := f.repo.Get(ctx, f.recruiter, id)
		if err != nil {
			t.Fatal(err)
		}
		return got
	}
	question()

	// Unrelated metadata keeps the bank, and saving neither resubmits nor drops the feedback.
	if err := f.repo.Update(ctx, f.recruiter, id, domain.Edit{Title: domain.StringChange{Present: true, Value: ptr("Clearer title")}}, nil); err != nil {
		t.Fatal(err)
	}
	if got := get(); got.Status != domain.StatusRejected || got.RejectReason == nil || got.CoreQuestionCount != 1 || got.Title != "Clearer title" {
		t.Fatalf("after metadata edit: status=%s reason=%v bank=%d", got.Status, got.RejectReason, got.CoreQuestionCount)
	}

	// A real seniority change clears the outdated bank.
	if err := f.repo.Update(ctx, f.recruiter, id, domain.Edit{SeniorityLevel: domain.StringChange{Present: true, Value: ptr("junior")}}, nil); err != nil {
		t.Fatal(err)
	}
	if got := get(); got.SeniorityLevel != domain.Junior || got.CoreQuestionCount != 0 || got.Status != domain.StatusRejected {
		t.Fatalf("after seniority edit: level=%s bank=%d status=%s", got.SeniorityLevel, got.CoreQuestionCount, got.Status)
	}

	// Corrected text points the JD at the new object and clears the bank too.
	question()
	if err := f.repo.Update(ctx, f.recruiter, id, domain.Edit{}, ptr("jp/corrected")); err != nil {
		t.Fatal(err)
	}
	if got := get(); got.ContentPath != "jp/corrected" || got.CoreQuestionCount != 0 {
		t.Fatalf("after text edit: path=%q bank=%d", got.ContentPath, got.CoreQuestionCount)
	}

	// Only the owner can resubmit.
	if err := f.repo.Resubmit(ctx, other, id); codeOf(err) != apperror.CodeJobPostingNotFound {
		t.Fatalf("other recruiter: %v", err)
	}
	for call := 1; call <= 2; call++ {
		if err := f.repo.Resubmit(ctx, f.recruiter, id); err != nil {
			t.Fatalf("resubmit call %d: %v", call, err)
		}
		if got := get(); got.Status != domain.StatusPending || got.RejectReason != nil {
			t.Fatalf("resubmit call %d: status=%s reason=%v", call, got.Status, got.RejectReason)
		}
	}

	// Back in pending, the text is fixed again and the bank lock applies.
	if err := f.repo.Update(ctx, f.recruiter, id, domain.Edit{}, ptr("jp/late")); !errors.Is(err, domain.ErrTextLocked) {
		t.Fatalf("text edit while pending: %v", err)
	}
	question()
	err := f.repo.Update(ctx, f.recruiter, id, domain.Edit{SeniorityLevel: domain.StringChange{Present: true, Value: ptr("lead")}}, nil)
	if !errors.Is(err, domain.ErrRequirementsLocked) {
		t.Fatalf("seniority edit while pending with a bank: %v", err)
	}
}

func TestResubmitOnlyFromRejected(t *testing.T) {
	ctx, f := newFixture(t)
	id := f.create(t, ctx, "Open")
	f.seed.exec(`UPDATE public.job_postings SET status = 'open' WHERE id = $1`, id)
	if err := f.repo.Resubmit(ctx, f.recruiter, id); codeOf(err) != apperror.CodeJobPostingNotRejected {
		t.Fatalf("open posting: %v", err)
	}
	if err := f.repo.Resubmit(ctx, f.recruiter, uuid.New()); codeOf(err) != apperror.CodeJobPostingNotFound {
		t.Fatalf("missing posting: %v", err)
	}
}
