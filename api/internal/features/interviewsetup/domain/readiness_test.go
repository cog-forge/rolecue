package domain

import (
	"reflect"
	"testing"

	"github.com/google/uuid"
)

func ref(name string, usable bool) *Choice {
	return &Choice{ID: uuid.New(), Name: name, Usable: usable}
}

func readyPractice() PracticeFacts {
	env := "office-3d"
	return PracticeFacts{
		Title: "Backend Engineer", Confirmed: true, BankReady: true,
		Avatar: ref("Minh", true), Voice: ref("Lan", true), Environment: &env, Saved: true,
	}
}

func codes(blockers []Blocker) []BlockerCode {
	out := make([]BlockerCode, 0, len(blockers))
	for _, b := range blockers {
		out = append(out, b.Code)
	}
	return out
}

func TestPracticeBlockers(t *testing.T) {
	bad := "tropical-beach"
	tests := []struct {
		name   string
		mutate func(*PracticeFacts)
		want   []BlockerCode
	}{
		{"everything chosen", func(*PracticeFacts) {}, []BlockerCode{}},
		{"unsaved draft needs all three", func(f *PracticeFacts) { f.Avatar, f.Voice, f.Environment, f.Saved = nil, nil, nil, false },
			[]BlockerCode{BlockerAvatarRequired, BlockerVoiceRequired, BlockerEnvironmentRequired}},
		{"unconfirmed profile hides the bank", func(f *PracticeFacts) { f.Confirmed, f.BankReady = false, false },
			[]BlockerCode{BlockerRoleProfileNotConfirmed}},
		{"confirmed without a bank", func(f *PracticeFacts) { f.BankReady = false },
			[]BlockerCode{BlockerQuestionBankNotReady}},
		{"stored avatar no longer usable", func(f *PracticeFacts) { f.Avatar.Usable = false },
			[]BlockerCode{BlockerAvatarUnavailable}},
		{"stored voice deactivated", func(f *PracticeFacts) { f.Voice.Usable = false },
			[]BlockerCode{BlockerVoiceUnavailable}},
		{"environment left the catalog", func(f *PracticeFacts) { f.Environment = &bad },
			[]BlockerCode{BlockerEnvironmentUnsupported}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			facts := readyPractice()
			tc.mutate(&facts)
			got := codes(PracticeBlockers(facts))
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("blockers = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestPracticeBlockersNamesTheField(t *testing.T) {
	facts := readyPractice()
	facts.Avatar, facts.Voice, facts.Environment = nil, nil, nil
	fields := map[BlockerCode]string{}
	for _, b := range PracticeBlockers(facts) {
		fields[b.Code] = b.Field
	}
	want := map[BlockerCode]string{BlockerAvatarRequired: "avatar", BlockerVoiceRequired: "voice", BlockerEnvironmentRequired: "environment"}
	if !reflect.DeepEqual(fields, want) {
		t.Fatalf("fields = %v, want %v", fields, want)
	}
}

func readyRecruitment() RecruitmentFacts {
	env := "meeting-3d"
	return RecruitmentFacts{
		Title: "Posting", Status: ApplicationInterviewEligible, WithinDeadline: true, PostingAccepting: true,
		Avatar: ref("Recruiter avatar", true), Voice: ref("Recruiter voice", true), Environment: &env,
	}
}

func TestRecruitmentBlockers(t *testing.T) {
	bad := "tropical-beach"
	tests := []struct {
		name   string
		mutate func(*RecruitmentFacts)
		want   []BlockerCode
	}{
		{"ready", func(*RecruitmentFacts) {}, []BlockerCode{}},
		{"environment not chosen yet", func(f *RecruitmentFacts) { f.Environment = nil },
			[]BlockerCode{BlockerEnvironmentRequired}},
		{"environment outside the catalog", func(f *RecruitmentFacts) { f.Environment = &bad },
			[]BlockerCode{BlockerEnvironmentUnsupported}},
		{"cv not approved", func(f *RecruitmentFacts) { f.Status = "pending"; f.WithinDeadline = false },
			[]BlockerCode{BlockerApplicationNotEligible}},
		{"posting closed or unfunded", func(f *RecruitmentFacts) { f.PostingAccepting = false },
			[]BlockerCode{BlockerApplicationNotEligible}},
		{"deadline passed", func(f *RecruitmentFacts) { f.WithinDeadline = false },
			[]BlockerCode{BlockerDeadlinePassed}},
		{"interview already started", func(f *RecruitmentFacts) { f.Started = true },
			[]BlockerCode{BlockerInterviewStarted}},
		{"locked avatar removed", func(f *RecruitmentFacts) { f.Avatar = nil },
			[]BlockerCode{BlockerLockedAvatarUnavailable}},
		{"locked avatar no longer the recruiter's", func(f *RecruitmentFacts) { f.Avatar.Usable = false },
			[]BlockerCode{BlockerLockedAvatarUnavailable}},
		{"locked voice inactive", func(f *RecruitmentFacts) { f.Voice.Usable = false },
			[]BlockerCode{BlockerLockedVoiceUnavailable}},
		{"locked voice never set", func(f *RecruitmentFacts) { f.Voice = nil },
			[]BlockerCode{BlockerLockedVoiceUnavailable}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			facts := readyRecruitment()
			tc.mutate(&facts)
			got := codes(RecruitmentBlockers(facts))
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("blockers = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestRecruitmentSaveGate(t *testing.T) {
	// The environment is the only thing a Candidate saves, so the gate is about the Application alone.
	if err := CheckRecruitmentSavable(readyRecruitment()); err != nil {
		t.Fatalf("ready application refused: %v", err)
	}
	for name, mutate := range map[string]func(*RecruitmentFacts){
		"not eligible":     func(f *RecruitmentFacts) { f.Status = "pending" },
		"deadline passed":  func(f *RecruitmentFacts) { f.WithinDeadline = false },
		"posting closed":   func(f *RecruitmentFacts) { f.PostingAccepting = false },
		"already started":  func(f *RecruitmentFacts) { f.Started = true },
		"after the window": func(f *RecruitmentFacts) { f.WithinDeadline = false; f.Status = "rejected" },
	} {
		facts := readyRecruitment()
		mutate(&facts)
		if err := CheckRecruitmentSavable(facts); err != ErrApplicationNotEligible {
			t.Fatalf("%s: err = %v, want ErrApplicationNotEligible", name, err)
		}
	}
}

func TestPracticeSaveGate(t *testing.T) {
	facts := readyPractice()
	if err := CheckPracticeSavable(facts); err != nil {
		t.Fatalf("ready profile refused: %v", err)
	}
	facts.Confirmed = false
	if err := CheckPracticeSavable(facts); err != ErrRoleProfileRequired {
		t.Fatalf("unconfirmed: err = %v", err)
	}
	facts = readyPractice()
	facts.BankReady = false
	if err := CheckPracticeSavable(facts); err != ErrQuestionBankNotReady {
		t.Fatalf("no bank: err = %v", err)
	}
}

func TestEnvironmentCatalog(t *testing.T) {
	for _, id := range Environments {
		if !ValidEnvironment(id) {
			t.Fatalf("%q is in the catalog but not valid", id)
		}
	}
	for _, id := range []string{"", " office-3d", "OFFICE-3D", "tropical-beach"} {
		if ValidEnvironment(id) {
			t.Fatalf("%q must not be valid", id)
		}
	}
}

func TestSetupReadyIsNoBlockers(t *testing.T) {
	setup := Setup{Mode: ModePractice}.WithBlockers(nil)
	if !setup.Ready || setup.Blockers == nil || len(setup.Blockers) != 0 {
		t.Fatalf("no blockers must be ready with an empty (non-nil) list: %+v", setup)
	}
	setup = Setup{Mode: ModePractice}.WithBlockers([]Blocker{{Code: BlockerAvatarRequired, Field: "avatar"}})
	if setup.Ready || len(setup.Blockers) != 1 {
		t.Fatal("a blocker must make the setup not ready")
	}
}
