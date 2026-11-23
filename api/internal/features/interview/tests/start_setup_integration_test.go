//go:build integration

// POST /interviews re-checks the saved interview setup before a new session starts (KAN-231 follow-up).
package tests

import (
	"errors"
	"testing"

	"github.com/cog-forge/rolecue/api/internal/features/interview/domain"
	"github.com/cog-forge/rolecue/api/internal/features/interview/repository"
	"github.com/cog-forge/rolecue/api/internal/features/interview/service"
	setupdomain "github.com/cog-forge/rolecue/api/internal/features/interviewsetup/domain"
	setuprepository "github.com/cog-forge/rolecue/api/internal/features/interviewsetup/repository"
	setupservice "github.com/cog-forge/rolecue/api/internal/features/interviewsetup/service"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// serviceWithSetup is the interview service of production: the real repository with the real setup service.
func serviceWithSetup(t *testing.T, pool *pgxpool.Pool) (*service.Service, *setupservice.Service) {
	t.Helper()
	svc, err := service.New(repository.NewRepository(pool), service.Settings{QuestionLimit: 3})
	if err != nil {
		t.Fatal(err)
	}
	setup := setupservice.New(setuprepository.NewRepository(pool))
	svc.SetStartSetup(setup)
	return svc, setup
}

func blockersOf(t *testing.T, err error) []service.SetupBlocker {
	t.Helper()
	var appErr *apperror.AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("err = %v, want an AppError", err)
	}
	details, ok := appErr.Details.(service.SetupNotReadyDetails)
	if !ok {
		t.Fatalf("details = %#v", appErr.Details)
	}
	return details.Blockers
}

func TestPracticeStartNeedsASavedReadySetupAndChargesNothingWithout(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc, setup := serviceWithSetup(t, pool)
	candidate := s.user("candidate")
	s.wallet(candidate, 20)
	profile, _ := s.confirmedProfile(candidate, 4)

	// Nothing saved: every choice is missing, and the refusal leaves coins, rows and slots alone.
	_, _, err := svc.Create(ctx, candidate, "candidate", practiceInput(profile))
	assertCode(t, err, apperror.CodeSetupNotReady)
	got := map[string]bool{}
	for _, b := range blockersOf(t, err) {
		got[b.Code] = true
	}
	for _, want := range []string{"avatar_required", "voice_required", "environment_required"} {
		if !got[want] {
			t.Errorf("blockers %v lack %s", got, want)
		}
	}
	if s.balance(candidate) != 20 || len(s.coinRows(candidate)) != 0 || s.interviewRows(profile) != 0 {
		t.Fatalf("a refused start changed something: balance %d, coin rows %d, interviews %d",
			s.balance(candidate), len(s.coinRows(candidate)), s.interviewRows(profile))
	}

	// A partly saved setup is still refused, with the remaining blockers only.
	avatar, voice := s.avatarOf(candidate), s.voice(true)
	if _, err = setup.SavePractice(ctx, candidate, "candidate", profile, setupdomain.PracticeInput{AvatarID: &avatar}); err != nil {
		t.Fatal(err)
	}
	_, _, err = svc.Create(ctx, candidate, "candidate", practiceInput(profile))
	assertCode(t, err, apperror.CodeSetupNotReady)
	if len(blockersOf(t, err)) != 2 || s.balance(candidate) != 20 {
		t.Fatalf("blockers = %v, balance %d", blockersOf(t, err), s.balance(candidate))
	}

	// Ready: the session starts with the saved avatar and voice and the usual debit.
	room := "cafe-3d"
	if _, err = setup.SavePractice(ctx, candidate, "candidate", profile, setupdomain.PracticeInput{AvatarID: &avatar, VoiceID: &voice, Environment: &room}); err != nil {
		t.Fatal(err)
	}
	session, created, err := svc.Create(ctx, candidate, "candidate", practiceInput(profile))
	if err != nil || !created {
		t.Fatalf("created=%v err=%v", created, err)
	}
	if session.AvatarID == nil || *session.AvatarID != avatar || session.VoiceID == nil || *session.VoiceID != voice {
		t.Fatalf("session avatar/voice = %v/%v, want the saved %s/%s", session.AvatarID, session.VoiceID, avatar, voice)
	}
	if s.balance(candidate) != 15 || len(s.coinRows(candidate)) != 1 {
		t.Fatalf("balance %d, coin rows %d, want one debit of 5", s.balance(candidate), len(s.coinRows(candidate)))
	}
}

func TestPracticeStartRefusesAvatarOrVoiceThatDiffersFromTheSavedSetup(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc, setup := serviceWithSetup(t, pool)
	candidate := s.user("candidate")
	s.wallet(candidate, 20)
	profile, _ := s.confirmedProfile(candidate, 4)
	avatar, voice, room := s.avatarOf(candidate), s.voice(true), "office"
	if _, err := setup.SavePractice(ctx, candidate, "candidate", profile, setupdomain.PracticeInput{AvatarID: &avatar, VoiceID: &voice, Environment: &room}); err != nil {
		t.Fatal(err)
	}
	otherVoice := s.voice(true)
	input := practiceInput(profile)
	input.VoiceID = &otherVoice
	_, _, err := svc.Create(ctx, candidate, "candidate", input)
	assertCode(t, err, apperror.CodeValidation)
	input = practiceInput(profile)
	other := uuid.New()
	input.AvatarID = &other
	_, _, err = svc.Create(ctx, candidate, "candidate", input)
	assertCode(t, err, apperror.CodeValidation)
	if s.balance(candidate) != 20 || s.interviewRows(profile) != 0 {
		t.Fatalf("a refused start changed something: balance %d interviews %d", s.balance(candidate), s.interviewRows(profile))
	}

	// The body may repeat the saved choices (what the current client sends).
	input = practiceInput(profile)
	input.AvatarID, input.VoiceID = &avatar, &voice
	if _, created, err := svc.Create(ctx, candidate, "candidate", input); err != nil || !created {
		t.Fatalf("created=%v err=%v", created, err)
	}
}

func TestResumeAndRetryAreNotBlockedWhenTheSetupIsNoLongerReady(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc, setup := serviceWithSetup(t, pool)
	candidate := s.user("candidate")
	s.wallet(candidate, 20)
	profile, _ := s.confirmedProfile(candidate, 4)
	avatar, voice, room := s.avatarOf(candidate), s.voice(true), "office"
	if _, err := setup.SavePractice(ctx, candidate, "candidate", profile, setupdomain.PracticeInput{AvatarID: &avatar, VoiceID: &voice, Environment: &room}); err != nil {
		t.Fatal(err)
	}
	input := practiceInput(profile)
	input.IdempotencyKey = "start-1"
	first, created, err := svc.Create(ctx, candidate, "candidate", input)
	if err != nil || !created {
		t.Fatalf("created=%v err=%v", created, err)
	}
	// The voice goes away and the setup is no longer ready.
	s.exec(`UPDATE public.voices SET is_active = false WHERE id = $1`, voice)
	if ready, err := setup.Practice(ctx, candidate, "candidate", profile); err != nil || ready.Ready {
		t.Fatalf("setup ready = %v err = %v, want not ready", ready.Ready, err)
	}

	// The open practice session of the profile comes back, whatever the setup says.
	again, created, err := svc.Create(ctx, candidate, "candidate", practiceInput(profile))
	if err != nil || created || again.ID != first.ID {
		t.Fatalf("resume: id=%s created=%v err=%v", again.ID, created, err)
	}
	// So does a retry with the same key, and the session can be resumed directly.
	retry, created, err := svc.Create(ctx, candidate, "candidate", input)
	if err != nil || created || retry.ID != first.ID {
		t.Fatalf("retry: id=%s created=%v err=%v", retry.ID, created, err)
	}
	if _, err := svc.Resume(ctx, candidate, "candidate", first.ID); err != nil {
		t.Fatal(err)
	}
	if s.balance(candidate) != 15 || s.interviewRows(profile) != 1 {
		t.Fatalf("balance %d interviews %d, want one session and one debit", s.balance(candidate), s.interviewRows(profile))
	}
}

func TestRecruitmentStartNeedsASavedReadySetupAndExistingSessionsAreNotBlocked(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc, setup := serviceWithSetup(t, pool)
	candidate := s.user("candidate")
	slot := int32(2)
	recruiter, posting, _ := s.openPosting(&slot, 4)
	avatar, voice := s.avatarOf(recruiter), s.voice(true)
	s.exec(`UPDATE public.job_postings SET required_avatar_id = $2, required_voice_id = $3 WHERE id = $1`, posting, avatar, voice)
	// Runs before the seed cleanup, which deletes the avatar and the voice the posting points at.
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `UPDATE public.job_postings SET required_avatar_id = NULL, required_voice_id = NULL WHERE id = $1`, posting)
	})
	application := s.eligibleApplication(posting, candidate)
	input := domain.CreateInput{Type: domain.TypeRecruitment, ApplicationID: &application}

	_, _, err := svc.Create(ctx, candidate, "candidate", input)
	assertCode(t, err, apperror.CodeSetupNotReady)
	if blockers := blockersOf(t, err); len(blockers) != 1 || blockers[0] != (service.SetupBlocker{Code: "environment_required", Field: "environment"}) {
		t.Fatalf("blockers = %v", blockers)
	}
	if s.count(`SELECT count(*) FROM public.interviews WHERE application_id = $1`, application) != 0 {
		t.Fatal("a refused recruitment start created a session")
	}

	room := "meeting-3d"
	if _, err = setup.SaveRecruitment(ctx, candidate, "candidate", application, &room); err != nil {
		t.Fatal(err)
	}
	first, created, err := svc.Create(ctx, candidate, "candidate", input)
	if err != nil || !created {
		t.Fatalf("created=%v err=%v", created, err)
	}
	if first.AvatarID == nil || *first.AvatarID != avatar || first.VoiceID == nil || *first.VoiceID != voice {
		t.Fatalf("recruitment session avatar/voice = %v/%v, want the posting's", first.AvatarID, first.VoiceID)
	}
	// The interview has started, so the setup now reports interview_already_started: asking again returns the
	// same session and is never refused.
	if ready, err := setup.Recruitment(ctx, candidate, "candidate", application); err != nil || ready.Ready {
		t.Fatalf("setup ready = %v err = %v, want not ready once started", ready.Ready, err)
	}
	again, created, err := svc.Create(ctx, candidate, "candidate", input)
	if err != nil || created || again.ID != first.ID {
		t.Fatalf("again: id=%s created=%v err=%v", again.ID, created, err)
	}
}

func TestStructuralRefusalsKeepTheirCodesWithTheSetupCheckOn(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc, _ := serviceWithSetup(t, pool)
	candidate := s.user("candidate")
	s.wallet(candidate, 20)
	unconfirmed, _ := s.roleProfile(candidate, "draft", 4)
	_, _, err := svc.Create(ctx, candidate, "candidate", practiceInput(unconfirmed))
	assertCode(t, err, apperror.CodeRoleProfileRequired)

	slot := int32(2)
	_, posting, _ := s.openPosting(&slot, 4)
	late := s.eligibleApplication(posting, candidate)
	s.setDeadlineInPast(late)
	application := late
	_, _, err = svc.Create(ctx, candidate, "candidate", domain.CreateInput{Type: domain.TypeRecruitment, ApplicationID: &application})
	assertCode(t, err, apperror.CodeApplicationNotEligible)

	// Someone else's Role Profile is reported as missing, like before.
	stranger := s.user("candidate")
	foreign, _ := s.confirmedProfile(stranger, 4)
	_, _, err = svc.Create(ctx, candidate, "candidate", practiceInput(foreign))
	assertCode(t, err, apperror.CodeJDNotFound)
}
