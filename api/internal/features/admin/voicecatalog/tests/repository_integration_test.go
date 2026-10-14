//go:build integration

package tests

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/cog-forge/rolecue/api/internal/features/admin/voicecatalog/domain"
	"github.com/cog-forge/rolecue/api/internal/features/admin/voicecatalog/repository"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// fixture seeds rows tagged with a random suffix. Cleanup is registered before any write and
// deletes children first, so a failing assertion never leaves rows behind.
type fixture struct {
	t    *testing.T
	ctx  context.Context
	pool *pgxpool.Pool
	tag  string
	undo []string
	args [][]any
}

func open(t *testing.T) *fixture {
	t.Helper()
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL is not set (load ../.env)")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := pool.Ping(ctx); err != nil {
		t.Skipf("database not reachable: %v", err)
	}
	f := &fixture{t: t, ctx: ctx, pool: pool, tag: "vc-" + uuid.NewString()[:8]}
	t.Cleanup(func() {
		for i := len(f.undo) - 1; i >= 0; i-- {
			if _, err := pool.Exec(context.Background(), f.undo[i], f.args[i]...); err != nil {
				t.Errorf("cleanup %q: %v", f.undo[i], err)
			}
		}
	})
	return f
}

func (f *fixture) exec(sql string, args ...any) {
	f.t.Helper()
	if _, err := f.pool.Exec(f.ctx, sql, args...); err != nil {
		f.t.Fatalf("seed %q: %v", sql, err)
	}
}

func (f *fixture) later(sql string, args ...any) {
	f.undo = append(f.undo, sql)
	f.args = append(f.args, args)
}

func (f *fixture) voice(name string, active bool) uuid.UUID {
	id := uuid.New()
	f.later(`DELETE FROM public.voices WHERE id = $1`, id)
	f.exec(`INSERT INTO public.voices (id, name, is_active) VALUES ($1, $2, $3)`, id, f.tag+" "+name, active)
	return id
}

// posting makes a Job Posting that requires the voice.
func (f *fixture) posting(voiceID uuid.UUID) {
	level, jd, recruiter, posting := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	f.later(`DELETE FROM public.levels WHERE id = $1`, level)
	f.later(`DELETE FROM public.users WHERE id = $1`, recruiter)
	f.later(`DELETE FROM public.job_descriptions WHERE id = $1`, jd)
	f.later(`DELETE FROM public.job_postings WHERE id = $1`, posting)
	f.exec(`INSERT INTO public.levels (id, name) VALUES ($1, $2)`, level, f.tag)
	f.exec(`INSERT INTO public.users (id, name, email, role) VALUES ($1, 'rec', $2, 'recruiter')`, recruiter, recruiter.String()+"@vc.test")
	f.exec(`INSERT INTO public.job_descriptions (id, content_path, level_id) VALUES ($1, 'x', $2)`, jd, level)
	f.exec(`INSERT INTO public.job_postings (id, jd_id, recruiter_id, title, required_voice_id) VALUES ($1, $2, $3, 'p', $4)`, posting, jd, recruiter, voiceID)
}

// interview makes a practice interview that used the voice.
func (f *fixture) interview(voiceID uuid.UUID) {
	level, jd, candidate, profile, interview := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	f.later(`DELETE FROM public.levels WHERE id = $1`, level)
	f.later(`DELETE FROM public.users WHERE id = $1`, candidate)
	f.later(`DELETE FROM public.job_descriptions WHERE id = $1`, jd)
	f.later(`DELETE FROM public.role_profiles WHERE id = $1`, profile)
	f.later(`DELETE FROM public.interviews WHERE id = $1`, interview)
	f.exec(`INSERT INTO public.levels (id, name) VALUES ($1, $2)`, level, f.tag)
	f.exec(`INSERT INTO public.users (id, name, email) VALUES ($1, 'cand', $2)`, candidate, candidate.String()+"@vc.test")
	f.exec(`INSERT INTO public.job_descriptions (id, content_path, level_id) VALUES ($1, 'x', $2)`, jd, level)
	f.exec(`INSERT INTO public.role_profiles (id, jd_id, candidate_id, title) VALUES ($1, $2, $3, 't')`, profile, jd, candidate)
	f.exec(`INSERT INTO public.interviews (id, role_profile_id, voice_id, type) VALUES ($1, $2, $3, 'practice')`, interview, profile, voiceID)
}

func wantCode(t *testing.T, err error, want apperror.Code) {
	t.Helper()
	var appErr *apperror.AppError
	if !errors.As(err, &appErr) || appErr.Code != want {
		t.Fatalf("error = %v, want code %q", err, want)
	}
}

func TestListFiltersAndCounts(t *testing.T) {
	f := open(t)
	repo := repository.NewRepository(f.pool)
	used := f.voice("Ava 50% warm", true)
	f.voice("Ben calm", true)
	f.voice("Cara_retired", false)
	f.posting(used)
	f.interview(used)

	all, total, err := repo.List(f.ctx, domain.VoiceListFilter{Status: domain.VoiceStatusAll, Query: f.tag, Limit: 10})
	if err != nil || total != 3 || len(all) != 3 || all[0].Name != f.tag+" Ava 50% warm" {
		t.Fatalf("all = %+v, %d, %v (sorted by name)", all, total, err)
	}
	if !all[0].InUse || all[0].InterviewCount != 1 || all[0].JobPostingCount != 1 || all[1].InUse {
		t.Fatalf("reference counts = %+v", all)
	}
	inactive, total, _ := repo.List(f.ctx, domain.VoiceListFilter{Status: domain.VoiceStatusInactive, Query: f.tag, Limit: 10})
	if total != 1 || inactive[0].IsActive {
		t.Fatalf("inactive = %+v", inactive)
	}
	// Wildcards in the search are literal. "0%w" and "n_c" would match Ava and Ben if % and _ were
	// wildcards; as literals they match nothing. "50%" and "a_r" appear literally in Ava and Cara.
	for query, want := range map[string]int64{f.tag + " ava 50%": 1, f.tag + " cara_": 1, "0%w": 0, "n_c": 0} {
		if _, total, err := repo.List(f.ctx, domain.VoiceListFilter{Status: domain.VoiceStatusAll, Query: query, Limit: 10}); err != nil || total != want {
			t.Fatalf("q=%q: total %d, want %d (%v)", query, total, want, err)
		}
	}
	page, total, _ := repo.List(f.ctx, domain.VoiceListFilter{Status: domain.VoiceStatusAll, Query: f.tag, Limit: 1, Offset: 1})
	if total != 3 || len(page) != 1 || page[0].Name != f.tag+" Ben calm" {
		t.Fatalf("page = %+v, %d", page, total)
	}
	_, err = repo.Get(f.ctx, uuid.New())
	wantCode(t, err, apperror.CodeVoiceNotFound)
}

func TestRemoveKeepsReferencedVoices(t *testing.T) {
	f := open(t)
	repo := repository.NewRepository(f.pool)
	byPosting, byInterview, unused := f.voice("posting", true), f.voice("interview", true), f.voice("unused", true)
	f.posting(byPosting)
	f.interview(byInterview)

	for _, id := range []uuid.UUID{byPosting, byInterview} {
		for call := 1; call <= 2; call++ {
			outcome, err := repo.Remove(f.ctx, id)
			if err != nil || outcome != domain.VoiceDeactivated {
				t.Fatalf("referenced voice, call %d: %q, %v", call, outcome, err)
			}
		}
		got, err := repo.Get(f.ctx, id)
		if err != nil || got.IsActive || !got.InUse {
			t.Fatalf("referenced voice must stay, inactive: %+v, %v", got, err)
		}
	}

	outcome, err := repo.Remove(f.ctx, unused)
	if err != nil || outcome != domain.VoiceDeleted {
		t.Fatalf("unused voice: %q, %v", outcome, err)
	}
	_, err = repo.Get(f.ctx, unused)
	wantCode(t, err, apperror.CodeVoiceNotFound)
	_, err = repo.Remove(f.ctx, unused)
	wantCode(t, err, apperror.CodeVoiceNotFound)
}

func TestActivateUndoesDeactivation(t *testing.T) {
	f := open(t)
	repo := repository.NewRepository(f.pool)
	used, unused := f.voice("reactivate", true), f.voice("gone", true)
	f.posting(used)
	if outcome, err := repo.Remove(f.ctx, used); err != nil || outcome != domain.VoiceDeactivated {
		t.Fatalf("deactivate: %q, %v", outcome, err)
	}
	for call := 1; call <= 2; call++ {
		if err := repo.Activate(f.ctx, used); err != nil {
			t.Fatalf("activate call %d: %v", call, err)
		}
	}
	got, err := repo.Get(f.ctx, used)
	if err != nil || !got.IsActive || got.JobPostingCount != 1 {
		t.Fatalf("reactivated voice: %+v, %v", got, err)
	}

	if _, err := repo.Remove(f.ctx, unused); err != nil {
		t.Fatal(err)
	}
	wantCode(t, repo.Activate(f.ctx, unused), apperror.CodeVoiceNotFound)
}
