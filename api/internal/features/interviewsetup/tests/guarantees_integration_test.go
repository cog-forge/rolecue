//go:build integration

package tests

import (
	"testing"

	"github.com/cog-forge/rolecue/api/internal/features/account/repository"
	"github.com/cog-forge/rolecue/api/internal/features/interviewsetup/domain"
)

// world is everything a start would change and a setup must not: coins, interviews and recruitment slots.
type world struct {
	balance      int
	transactions int
	interviews   int
	slots        int
	consumed     int
}

func (s *seed) world(candidate, posting any) world {
	s.t.Helper()
	var w world
	if err := s.pool.QueryRow(s.ctx, `SELECT balance FROM public.wallets WHERE user_id = $1`, candidate).Scan(&w.balance); err != nil {
		s.t.Fatal(err)
	}
	w.transactions = s.count(`SELECT count(*) FROM public.transactions WHERE "from" = $1::text OR "to" = $1::text`, candidate)
	w.interviews = s.count(`SELECT count(*) FROM public.interviews i
		LEFT JOIN public.role_profiles rp ON rp.id = i.role_profile_id
		LEFT JOIN public.applications a ON a.id = i.application_id
		WHERE rp.candidate_id = $1 OR a.candidate_id = $1`, candidate)
	if posting != nil {
		if err := s.pool.QueryRow(s.ctx, `SELECT COALESCE(interview_slot, 0), erased_interviews FROM public.job_postings WHERE id = $1`, posting).Scan(&w.slots, &w.consumed); err != nil {
			s.t.Fatal(err)
		}
	}
	return w
}

// Setup and preflight never debit coins, never use a recruitment slot and never create an interview, whatever
// the outcome of the call.
func TestSetupNeverDebitsCoinsUsesSlotsOrCreatesInterviews(t *testing.T) {
	s := newSeed(t)
	f := s.recruitment()
	profile := s.confirmedProfile(f.candidate)
	avatar, voice := s.avatarOf(f.candidate, "Minh"), s.voice(true)
	before := s.world(f.candidate, f.posting)

	calls := []func(){
		func() {
			_, _ = s.svc.SavePractice(s.ctx, f.candidate, "candidate", profile, domain.PracticeInput{AvatarID: &avatar, VoiceID: &voice, Environment: str("cafe")})
		},
		func() {
			_, _ = s.svc.SavePractice(s.ctx, f.candidate, "candidate", profile, domain.PracticeInput{Environment: str("moon")})
		},
		func() { _, _ = s.svc.Practice(s.ctx, f.candidate, "candidate", profile) },
		func() { _, _ = s.svc.SaveRecruitment(s.ctx, f.candidate, "candidate", f.application, str("office")) },
		func() { _, _ = s.svc.Recruitment(s.ctx, f.candidate, "candidate", f.application) },
	}
	for _, call := range calls {
		call()
	}
	if after := s.world(f.candidate, f.posting); after != before {
		t.Fatalf("a setup changed coins, interviews or slots: %+v -> %+v", before, after)
	}
	if before.balance != 500 || before.interviews != 0 {
		t.Fatalf("fixture drifted: %+v", before)
	}
}

// The table's own rules: a row is either a practice setup of a profile or a recruitment setup of an
// application, a recruitment row never holds the Recruiter's avatar or voice, and each target has one row.
func TestSchemaRejectsMalformedSetups(t *testing.T) {
	s := newSeed(t)
	f := s.recruitment()
	profile := s.confirmedProfile(f.candidate)
	avatar, voice := s.avatarOf(f.candidate, "Minh"), s.voice(true)

	bad := []struct {
		name, sql string
		args      []any
	}{
		{"practice without a profile", `INSERT INTO public.interview_setups (type) VALUES ('practice')`, nil},
		{"practice with an application", `INSERT INTO public.interview_setups (type, role_profile_id, application_id) VALUES ('practice', $1, $2)`, []any{profile, f.application}},
		{"recruitment without an application", `INSERT INTO public.interview_setups (type, role_profile_id) VALUES ('recruitment', $1)`, []any{profile}},
		{"recruitment with an avatar", `INSERT INTO public.interview_setups (type, application_id, avatar_id) VALUES ('recruitment', $1, $2)`, []any{f.application, avatar}},
		{"recruitment with a voice", `INSERT INTO public.interview_setups (type, application_id, voice_id) VALUES ('recruitment', $1, $2)`, []any{f.application, voice}},
		{"blank environment", `INSERT INTO public.interview_setups (type, role_profile_id, environment) VALUES ('practice', $1, '  ')`, []any{profile}},
	}
	for _, tc := range bad {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := s.pool.Exec(s.ctx, tc.sql, tc.args...); err == nil {
				t.Fatal("the schema accepted a malformed setup")
			}
		})
	}
	s.exec(`INSERT INTO public.interview_setups (type, role_profile_id) VALUES ('practice', $1)`, profile)
	if _, err := s.pool.Exec(s.ctx, `INSERT INTO public.interview_setups (type, role_profile_id) VALUES ('practice', $1)`, profile); err == nil {
		t.Fatal("a profile must have one setup")
	}
}

// Erasing a Candidate takes their setups with them: the Role Profile's and the Application's, and the avatar the
// setup pointed at goes without a foreign key error.
func TestAccountErasureLeavesNoSetups(t *testing.T) {
	s := newSeed(t)
	f := s.recruitment()
	profile := s.confirmedProfile(f.candidate)
	avatar, voice := s.avatarOf(f.candidate, "Minh"), s.voice(true)
	if _, err := s.svc.SavePractice(s.ctx, f.candidate, "candidate", profile, domain.PracticeInput{AvatarID: &avatar, VoiceID: &voice, Environment: str("cafe")}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.svc.SaveRecruitment(s.ctx, f.candidate, "candidate", f.application, str("office")); err != nil {
		t.Fatal(err)
	}
	// Another Candidate's setup on the same voice must survive.
	bystander := s.user("candidate")
	bystanderProfile := s.confirmedProfile(bystander)
	if _, err := s.svc.SavePractice(s.ctx, bystander, "candidate", bystanderProfile, domain.PracticeInput{VoiceID: &voice, Environment: str("cafe")}); err != nil {
		t.Fatal(err)
	}

	owned := `SELECT count(*) FROM public.interview_setups WHERE role_profile_id IN (SELECT id FROM public.role_profiles WHERE candidate_id = $1)
		OR application_id IN (SELECT id FROM public.applications WHERE candidate_id = $1)`
	if n := s.count(owned, f.candidate); n != 2 {
		t.Fatalf("setups before = %d", n)
	}
	if _, err := repository.NewRepository(s.pool).Erase(s.ctx, f.candidate); err != nil {
		t.Fatalf("erasure: %v", err)
	}
	if n := s.count(`SELECT count(*) FROM public.interview_setups WHERE role_profile_id = $1 OR application_id = $2`, profile, f.application); n != 0 {
		t.Fatalf("%d setups left behind after the account was erased", n)
	}
	if n := s.count(`SELECT count(*) FROM public.users WHERE id = $1`, f.candidate); n != 0 {
		t.Fatal("the account is still there")
	}
	left, err := s.svc.Practice(s.ctx, bystander, "candidate", bystanderProfile)
	if err != nil || !left.Saved || left.Voice == nil || left.Voice.ID != voice {
		t.Fatalf("another Candidate's setup must survive: %+v, %v", left, err)
	}
}
