//go:build integration

package tests

import (
	"testing"
	"time"

	"github.com/cog-forge/rolecue/api/internal/features/interviewsetup/domain"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

func TestRecruitmentSetupLocksAvatarAndVoice(t *testing.T) {
	s := newSeed(t)
	f := s.recruitment()

	setup, err := s.svc.Recruitment(s.ctx, f.candidate, "candidate", f.application)
	if err != nil {
		t.Fatal(err)
	}
	if setup.Mode != domain.ModeRecruitment || setup.TargetTitle != "Setup posting" || setup.InterviewDeadline == nil || setup.Saved {
		t.Fatalf("setup = %+v", setup)
	}
	if setup.Avatar == nil || setup.Avatar.ID != f.avatar || setup.Avatar.Name != "Company interviewer" || setup.Voice == nil || setup.Voice.ID != f.voice {
		t.Fatalf("the Recruiter's avatar and voice must be returned: %+v", setup)
	}
	if len(setup.LockedFields) != 2 || setup.LockedFields[0] != "avatar" || setup.LockedFields[1] != "voice" {
		t.Fatalf("locked fields = %v", setup.LockedFields)
	}
	if got := blockerCodes(setup); setup.Ready || len(got) != 1 || got[0] != domain.BlockerEnvironmentRequired {
		t.Fatalf("blockers = %v", got)
	}

	saved, err := s.svc.SaveRecruitment(s.ctx, f.candidate, "candidate", f.application, str("meeting-3d"))
	if err != nil {
		t.Fatal(err)
	}
	if !saved.Ready || !saved.Saved || *saved.Environment != "meeting-3d" || saved.Avatar.ID != f.avatar {
		t.Fatalf("saved = %+v", saved)
	}
	// The row keeps only the Candidate's part: the locked choices are never copied into it.
	if n := s.count(`SELECT count(*) FROM public.interview_setups WHERE application_id = $1 AND avatar_id IS NULL AND voice_id IS NULL AND type = 'recruitment'`, f.application); n != 1 {
		t.Fatalf("recruitment setup row count = %d", n)
	}
	// A repeat changes nothing; null puts the choice back to not chosen.
	if _, err := s.svc.SaveRecruitment(s.ctx, f.candidate, "candidate", f.application, str("meeting-3d")); err != nil {
		t.Fatal(err)
	}
	cleared, err := s.svc.SaveRecruitment(s.ctx, f.candidate, "candidate", f.application, nil)
	if err != nil || cleared.Ready || cleared.Environment != nil {
		t.Fatalf("cleared = %+v, err = %v", cleared, err)
	}
}

func TestRecruitmentSaveRefusals(t *testing.T) {
	s := newSeed(t)
	f := s.recruitment()
	past := time.Now().Add(-time.Hour)
	pending := s.application(s.posting(f.recruiter, f.avatar, f.voice), f.candidate, "pending", nil)
	expired := s.application(s.posting(f.recruiter, f.avatar, f.voice), f.candidate, "interview_eligible", &past)
	started := s.eligibleApplication(s.posting(f.recruiter, f.avatar, f.voice), f.candidate)
	s.exec(`INSERT INTO public.interviews (type, application_id) VALUES ('recruitment', $1)`, started)
	closedPosting := s.posting(f.recruiter, f.avatar, f.voice)
	closed := s.eligibleApplication(closedPosting, f.candidate)
	s.exec(`UPDATE public.job_postings SET status = 'closed' WHERE id = $1`, closedPosting)
	stranger := s.user("candidate")

	tests := []struct {
		name        string
		user        uuid.UUID
		role        string
		application uuid.UUID
		env         *string
		want        apperror.Code
	}{
		{"foreign application", stranger, "candidate", f.application, str("cafe"), apperror.CodeApplicationNotFound},
		{"missing application", f.candidate, "candidate", uuid.New(), str("cafe"), apperror.CodeApplicationNotFound},
		{"cv not approved", f.candidate, "candidate", pending, str("cafe"), apperror.CodeApplicationNotEligible},
		{"deadline passed", f.candidate, "candidate", expired, str("cafe"), apperror.CodeApplicationNotEligible},
		{"interview already started", f.candidate, "candidate", started, str("cafe"), apperror.CodeApplicationNotEligible},
		{"posting no longer accepting", f.candidate, "candidate", closed, str("cafe"), apperror.CodeApplicationNotEligible},
		{"environment outside the catalog", f.candidate, "candidate", f.application, str("moon"), apperror.CodeValidation},
		{"recruiter", f.recruiter, "recruiter", f.application, str("cafe"), apperror.CodeForbidden},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := s.svc.SaveRecruitment(s.ctx, tc.user, tc.role, tc.application, tc.env)
			assertCode(t, err, tc.want)
		})
	}
	if n := s.count(`SELECT count(*) FROM public.interview_setups WHERE application_id = ANY($1::uuid[])`, []uuid.UUID{f.application, pending, expired, started, closed}); n != 0 {
		t.Fatalf("refused saves wrote %d rows", n)
	}
	if _, err := s.svc.Recruitment(s.ctx, stranger, "candidate", f.application); codeOf(err) != apperror.CodeApplicationNotFound {
		t.Fatalf("a foreign read must not find the application: %v", err)
	}
}

// Reads report why an application cannot start instead of refusing, and stay live: they follow the recruiter's
// posting and the clock.
func TestRecruitmentReadReportsBlockersLive(t *testing.T) {
	s := newSeed(t)
	f := s.recruitment()
	if _, err := s.svc.SaveRecruitment(s.ctx, f.candidate, "candidate", f.application, str("office")); err != nil {
		t.Fatal(err)
	}
	read := func() domain.Setup {
		t.Helper()
		setup, err := s.svc.Recruitment(s.ctx, f.candidate, "candidate", f.application)
		if err != nil {
			t.Fatal(err)
		}
		return setup
	}
	only := func(code domain.BlockerCode) {
		t.Helper()
		setup := read()
		if got := blockerCodes(setup); setup.Ready || len(got) != 1 || got[0] != code {
			t.Fatalf("blockers = %v, want only %s", got, code)
		}
	}
	if !read().Ready {
		t.Fatalf("setup not ready: %v", blockerCodes(read()))
	}

	s.exec(`UPDATE public.voices SET is_active = false WHERE id = $1`, f.voice)
	only(domain.BlockerLockedVoiceUnavailable)
	s.exec(`UPDATE public.voices SET is_active = true WHERE id = $1`, f.voice)

	s.exec(`UPDATE public.job_postings SET required_avatar_id = NULL WHERE id = $1`, f.posting)
	only(domain.BlockerLockedAvatarUnavailable)
	s.exec(`UPDATE public.job_postings SET required_avatar_id = $2 WHERE id = $1`, f.posting, f.avatar)

	// The avatar left the Recruiter's inventory: it is no longer theirs to lock.
	stranger := s.user("candidate")
	s.exec(`INSERT INTO public.inventories (user_id, slot) VALUES ($1, 1) ON CONFLICT (user_id) DO NOTHING`, stranger)
	s.exec(`UPDATE public.avatars SET inventory_id = (SELECT id FROM public.inventories WHERE user_id = $2) WHERE id = $1`, f.avatar, stranger)
	only(domain.BlockerLockedAvatarUnavailable)
	s.exec(`UPDATE public.avatars SET inventory_id = (SELECT id FROM public.inventories WHERE user_id = $2) WHERE id = $1`, f.avatar, f.recruiter)

	s.exec(`UPDATE public.applications SET interview_deadline = now() - interval '1 hour' WHERE id = $1`, f.application)
	only(domain.BlockerDeadlinePassed)
	s.exec(`UPDATE public.applications SET interview_deadline = now() + interval '1 day' WHERE id = $1`, f.application)

	s.exec(`UPDATE public.applications SET status = 'rejected' WHERE id = $1`, f.application)
	only(domain.BlockerApplicationNotEligible)
	s.exec(`UPDATE public.applications SET status = 'interview_eligible' WHERE id = $1`, f.application)

	s.exec(`INSERT INTO public.interviews (type, application_id) VALUES ('recruitment', $1)`, f.application)
	only(domain.BlockerInterviewStarted)
}
