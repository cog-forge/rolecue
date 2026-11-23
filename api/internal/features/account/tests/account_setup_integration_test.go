//go:build integration

// Saved interview setups (KAN-231) in the account data export. The fixture helpers come from
// account_integration_test.go.
package tests

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

func TestExportIncludesOnlyTheOwnersInterviewSetups(t *testing.T) {
	f := newFixture(t)
	recruiter, alice, bob := f.user("recruiter"), f.user("candidate"), f.user("candidate")
	voice := uuid.New()
	f.exec(`INSERT INTO public.voices (id, name, is_active) VALUES ($1, 'Export voice', true)`, voice)
	t.Cleanup(func() { _, _ = f.pool.Exec(context.Background(), `DELETE FROM public.voices WHERE id = $1`, voice) })

	aliceProfile, _, _, _ := f.roadmapOf(alice.ID)
	bobProfile, _, _, _ := f.roadmapOf(bob.ID)
	posting := f.posting(recruiter.ID, "open", uuid.NewString())
	aliceApp, _ := f.application(alice.ID, posting, "interviewed", "alice-cv", "alice-rec")
	bobApp, _ := f.application(bob.ID, posting, "interviewed", "bob-cv", "bob-rec")
	f.exec(`INSERT INTO public.interview_setups (type, role_profile_id, voice_id, environment) VALUES ('practice', $1, $2, 'cafe-3d')`, aliceProfile, voice)
	f.exec(`INSERT INTO public.interview_setups (type, application_id, environment) VALUES ('recruitment', $1, 'office')`, aliceApp)
	f.exec(`INSERT INTO public.interview_setups (type, role_profile_id, environment) VALUES ('practice', $1, 'balcony')`, bobProfile)
	f.exec(`INSERT INTO public.interview_setups (type, application_id, environment) VALUES ('recruitment', $1, 'meeting-3d')`, bobApp)

	out, err := f.svc.Export(f.ctx, alice)
	if err != nil {
		t.Fatal(err)
	}
	if len(out.InterviewSetups) != 2 {
		t.Fatalf("alice exports %d setups, want her two: %+v", len(out.InterviewSetups), out.InterviewSetups)
	}
	practice, recruitment := out.InterviewSetups[0], out.InterviewSetups[1]
	if practice.Type != "practice" || practice.RoleProfileID == nil || *practice.RoleProfileID != aliceProfile || practice.ApplicationID != nil ||
		practice.VoiceID == nil || *practice.VoiceID != voice || practice.AvatarID != nil ||
		practice.Environment == nil || *practice.Environment != "cafe-3d" || practice.CreatedAt.IsZero() || practice.UpdatedAt.IsZero() {
		t.Fatalf("practice setup = %+v", practice)
	}
	if recruitment.Type != "recruitment" || recruitment.ApplicationID == nil || *recruitment.ApplicationID != aliceApp || recruitment.RoleProfileID != nil ||
		recruitment.AvatarID != nil || recruitment.VoiceID != nil || recruitment.Environment == nil || *recruitment.Environment != "office" {
		t.Fatalf("recruitment setup = %+v", recruitment)
	}
	for _, s := range out.InterviewSetups {
		if (s.RoleProfileID != nil && *s.RoleProfileID == bobProfile) || (s.ApplicationID != nil && *s.ApplicationID == bobApp) {
			t.Fatalf("bob's setup leaked into alice's export: %+v", s)
		}
	}

	bobOut, err := f.svc.Export(f.ctx, bob)
	if err != nil || len(bobOut.InterviewSetups) != 2 {
		t.Fatalf("bob export = %+v, err %v", bobOut.InterviewSetups, err)
	}
	// The posting's Recruiter owns neither Candidate's setup.
	recruiterOut, err := f.svc.Export(f.ctx, recruiter)
	if err != nil || len(recruiterOut.InterviewSetups) != 0 {
		t.Fatalf("recruiter export = %+v, err %v", recruiterOut.InterviewSetups, err)
	}
}

func TestExportWithoutSetupsHasNone(t *testing.T) {
	f := newFixture(t)
	alice := f.user("candidate")
	f.roadmapOf(alice.ID)
	out, err := f.svc.Export(f.ctx, alice)
	if err != nil || len(out.InterviewSetups) != 0 {
		t.Fatalf("export = %+v, err %v", out.InterviewSetups, err)
	}
}
