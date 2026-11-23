//go:build integration

package tests

import (
	"sync"
	"testing"

	"github.com/cog-forge/rolecue/api/internal/features/interviewsetup/domain"
	"github.com/cog-forge/rolecue/api/internal/features/interviewsetup/repository"
	"github.com/cog-forge/rolecue/api/internal/features/interviewsetup/service"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

func TestPracticeSetupSavesReloadsAndReplaces(t *testing.T) {
	s := newSeed(t)
	candidate := s.user("candidate")
	profile := s.confirmedProfile(candidate)
	avatar, voice := s.avatarOf(candidate, "Minh"), s.voice(true)

	// Nothing saved yet: a draft with every choice missing, not an error.
	setup, err := s.svc.Practice(s.ctx, candidate, "candidate", profile)
	if err != nil {
		t.Fatal(err)
	}
	if setup.Saved || setup.Ready || setup.Mode != domain.ModePractice || setup.TargetTitle != "Setup role" {
		t.Fatalf("unsaved setup = %+v", setup)
	}
	want := []domain.BlockerCode{domain.BlockerAvatarRequired, domain.BlockerVoiceRequired, domain.BlockerEnvironmentRequired}
	if got := blockerCodes(setup); len(got) != 3 || got[0] != want[0] || got[1] != want[1] || got[2] != want[2] {
		t.Fatalf("blockers = %v", got)
	}

	saved, err := s.svc.SavePractice(s.ctx, candidate, "candidate", profile, domain.PracticeInput{AvatarID: &avatar, VoiceID: &voice, Environment: str("office-3d")})
	if err != nil {
		t.Fatal(err)
	}
	if !saved.Ready || !saved.Saved || saved.UpdatedAt == nil || saved.Avatar == nil || saved.Avatar.Name != "Minh" || saved.Voice.ID != voice {
		t.Fatalf("saved setup = %+v", saved)
	}

	// A new service over a new pool connection reads the same thing back: it is stored, not remembered.
	_, pool := openPool(t)
	reloaded, err := service.New(repository.NewRepository(pool)).Practice(s.ctx, candidate, "candidate", profile)
	if err != nil {
		t.Fatal(err)
	}
	if !reloaded.Ready || *reloaded.Environment != "office-3d" || reloaded.Avatar.ID != avatar {
		t.Fatalf("reloaded setup = %+v", reloaded)
	}

	// Saving is a full replacement: null means not chosen yet, and the draft is blocked again.
	draft, err := s.svc.SavePractice(s.ctx, candidate, "candidate", profile, domain.PracticeInput{Environment: str("cafe")})
	if err != nil {
		t.Fatal(err)
	}
	if draft.Ready || draft.Avatar != nil || draft.Voice != nil || *draft.Environment != "cafe" || len(draft.Blockers) != 2 {
		t.Fatalf("replaced setup = %+v", draft)
	}
	if s.count(`SELECT count(*) FROM public.interview_setups WHERE role_profile_id = $1`, profile) != 1 {
		t.Fatal("a profile has exactly one setup")
	}
}

func TestRepeatedSaveIsInvisible(t *testing.T) {
	s := newSeed(t)
	candidate := s.user("candidate")
	profile := s.confirmedProfile(candidate)
	avatar, voice := s.avatarOf(candidate, "Minh"), s.voice(true)
	in := domain.PracticeInput{AvatarID: &avatar, VoiceID: &voice, Environment: str("meeting-3d")}
	if _, err := s.svc.SavePractice(s.ctx, candidate, "candidate", profile, in); err != nil {
		t.Fatal(err)
	}
	before := s.stamp(profile)
	for i := 0; i < 3; i++ {
		if _, err := s.svc.SavePractice(s.ctx, candidate, "candidate", profile, in); err != nil {
			t.Fatalf("retry %d: %v", i, err)
		}
	}
	if after := s.stamp(profile); after != before {
		t.Fatalf("a retried save changed the row: %+v -> %+v", before, after)
	}
}

func TestConcurrentSavesLeaveOneRow(t *testing.T) {
	s := newSeed(t)
	candidate := s.user("candidate")
	profile := s.confirmedProfile(candidate)
	var wg sync.WaitGroup
	errs := make(chan error, 12)
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			env := domain.Environments[i%len(domain.Environments)]
			_, err := s.svc.SavePractice(s.ctx, candidate, "candidate", profile, domain.PracticeInput{Environment: &env})
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("a concurrent save failed: %v", err)
		}
	}
	if s.stamp(profile).rows != 1 {
		t.Fatal("concurrent saves must leave exactly one row")
	}
}

func TestPracticeSaveRefusals(t *testing.T) {
	s := newSeed(t)
	candidate, other := s.user("candidate"), s.user("candidate")
	profile := s.confirmedProfile(candidate)
	avatar := s.avatarOf(candidate, "Mine")
	foreignAvatar, inactive := s.avatarOf(other, "Theirs"), s.voice(false)
	unconfirmed := s.profile(candidate, "review", 3)
	noBank := s.profile(candidate, "confirmed", 0)
	missing := uuid.New()

	tests := []struct {
		name    string
		role    string
		user    uuid.UUID
		profile uuid.UUID
		in      domain.PracticeInput
		want    apperror.Code
	}{
		{"foreign profile", "candidate", other, profile, domain.PracticeInput{}, apperror.CodeJDNotFound},
		{"missing profile", "candidate", candidate, uuid.New(), domain.PracticeInput{}, apperror.CodeJDNotFound},
		{"unconfirmed profile", "candidate", candidate, unconfirmed, domain.PracticeInput{}, apperror.CodeRoleProfileRequired},
		{"bank not ready", "candidate", candidate, noBank, domain.PracticeInput{}, apperror.CodeQuestionBankNotReady},
		{"foreign avatar", "candidate", candidate, profile, domain.PracticeInput{AvatarID: &foreignAvatar}, apperror.CodeAvatarNotFound},
		{"unknown avatar", "candidate", candidate, profile, domain.PracticeInput{AvatarID: &missing}, apperror.CodeAvatarNotFound},
		{"unknown voice", "candidate", candidate, profile, domain.PracticeInput{VoiceID: &missing}, apperror.CodeVoiceNotFound},
		{"inactive voice", "candidate", candidate, profile, domain.PracticeInput{VoiceID: &inactive}, apperror.CodeVoiceNotFound},
		{"environment outside the catalog", "candidate", candidate, profile, domain.PracticeInput{Environment: str("moon")}, apperror.CodeValidation},
		{"recruiter", "recruiter", candidate, profile, domain.PracticeInput{}, apperror.CodeForbidden},
		// The avatar is fine, the voice is not: the whole save is refused, the avatar is not half-saved.
		{"one bad choice refuses all", "candidate", candidate, profile, domain.PracticeInput{AvatarID: &avatar, VoiceID: &inactive, Environment: str("cafe")}, apperror.CodeVoiceNotFound},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := s.svc.SavePractice(s.ctx, tc.user, tc.role, tc.profile, tc.in)
			assertCode(t, err, tc.want)
		})
	}
	if n := s.count(`SELECT count(*) FROM public.interview_setups WHERE role_profile_id IN ($1, $2, $3)`, profile, unconfirmed, noBank); n != 0 {
		t.Fatalf("refused saves wrote %d rows", n)
	}

	// A foreign profile is not found on a read either, for the same reason as the interview start.
	if _, err := s.svc.Practice(s.ctx, other, "candidate", profile); codeOf(err) != apperror.CodeJDNotFound {
		t.Fatalf("foreign read: %v", err)
	}
}

// A setup is revalidated against the live data on every read: stale choices are reported and kept, nothing is
// cleared or written, and the first service call after a fix is ready again.
func TestReadRevalidatesStaleChoicesWithoutWriting(t *testing.T) {
	s := newSeed(t)
	candidate := s.user("candidate")
	profile := s.confirmedProfile(candidate)
	avatar, voice := s.avatarOf(candidate, "Minh"), s.voice(true)
	if _, err := s.svc.SavePractice(s.ctx, candidate, "candidate", profile, domain.PracticeInput{AvatarID: &avatar, VoiceID: &voice, Environment: str("balcony")}); err != nil {
		t.Fatal(err)
	}
	before := s.stamp(profile)
	read := func() domain.Setup {
		t.Helper()
		setup, err := s.svc.Practice(s.ctx, candidate, "candidate", profile)
		if err != nil {
			t.Fatal(err)
		}
		return setup
	}
	only := func(setup domain.Setup, code domain.BlockerCode) {
		t.Helper()
		if got := blockerCodes(setup); setup.Ready || len(got) != 1 || got[0] != code {
			t.Fatalf("blockers = %v, want only %s", got, code)
		}
	}

	// The voice is switched off by an admin: reported, still shown, never cleared.
	s.exec(`UPDATE public.voices SET is_active = false WHERE id = $1`, voice)
	setup := read()
	only(setup, domain.BlockerVoiceUnavailable)
	if setup.Voice == nil || setup.Voice.ID != voice {
		t.Fatal("a deactivated voice stays in the setup")
	}
	s.exec(`UPDATE public.voices SET is_active = true WHERE id = $1`, voice)
	if !read().Ready {
		t.Fatal("the setup is ready again as soon as the voice is")
	}

	// The avatar's model is blocked by moderation.
	s.exec(`UPDATE public.avatar_assets SET status = 'blocked' WHERE id = (SELECT asset_id FROM public.avatars WHERE id = $1)`, avatar)
	only(read(), domain.BlockerAvatarUnavailable)
	s.exec(`UPDATE public.avatar_assets SET status = 'ready' WHERE id = (SELECT asset_id FROM public.avatars WHERE id = $1)`, avatar)

	// The avatar is no longer in the Candidate's inventory.
	other := s.user("candidate")
	s.exec(`INSERT INTO public.inventories (user_id, slot) VALUES ($1, 1) ON CONFLICT (user_id) DO NOTHING`, other)
	s.exec(`UPDATE public.avatars SET inventory_id = (SELECT id FROM public.inventories WHERE user_id = $2) WHERE id = $1`, avatar, other)
	only(read(), domain.BlockerAvatarUnavailable)
	s.exec(`UPDATE public.avatars SET inventory_id = (SELECT id FROM public.inventories WHERE user_id = $2) WHERE id = $1`, avatar, candidate)

	// The bank is emptied, the profile is no longer confirmed.
	s.exec(`UPDATE public.role_profiles SET status = 'review' WHERE id = $1`, profile)
	only(read(), domain.BlockerRoleProfileNotConfirmed)
	s.exec(`UPDATE public.role_profiles SET status = 'confirmed' WHERE id = $1`, profile)
	s.exec(`DELETE FROM public.core_questions WHERE jd_id = (SELECT jd_id FROM public.role_profiles WHERE id = $1)`, profile)
	only(read(), domain.BlockerQuestionBankNotReady)

	if after := s.stamp(profile); after != before {
		t.Fatalf("reads changed the stored setup: %+v -> %+v", before, after)
	}
}

// Deleting an avatar or removing a voice keeps working with a saved setup pointing at it: the database clears
// the choice and the setup reports it as required.
func TestDeletedAvatarAndVoiceBecomeRequired(t *testing.T) {
	s := newSeed(t)
	candidate := s.user("candidate")
	profile := s.confirmedProfile(candidate)
	avatar, voice := s.avatarOf(candidate, "Minh"), s.voice(true)
	if _, err := s.svc.SavePractice(s.ctx, candidate, "candidate", profile, domain.PracticeInput{AvatarID: &avatar, VoiceID: &voice, Environment: str("cafe-3d")}); err != nil {
		t.Fatal(err)
	}
	s.exec(`DELETE FROM public.avatars WHERE id = $1`, avatar)
	s.exec(`DELETE FROM public.voices WHERE id = $1`, voice)
	setup, err := s.svc.Practice(s.ctx, candidate, "candidate", profile)
	if err != nil {
		t.Fatal(err)
	}
	if setup.Ready || setup.Avatar != nil || setup.Voice != nil || !setup.Saved || *setup.Environment != "cafe-3d" {
		t.Fatalf("setup = %+v", setup)
	}
	got := blockerCodes(setup)
	if len(got) != 2 || got[0] != domain.BlockerAvatarRequired || got[1] != domain.BlockerVoiceRequired {
		t.Fatalf("blockers = %v", got)
	}
}
