// Package domain holds the rules of the persisted pre-interview setup (KAN-231): the Candidate's chosen
// interviewer avatar, voice and room (environment) for a practice or a recruitment interview, and the
// readiness blockers computed from it. Nothing here starts a session, debits coins or uses a recruitment slot.
package domain

import (
	"slices"
	"time"

	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

// Mode is the kind of interview a setup belongs to. The values are the interview_type enum.
type Mode string

const (
	ModePractice    Mode = "practice"
	ModeRecruitment Mode = "recruitment"
)

// ApplicationInterviewEligible is the Application status a recruitment interview needs (CV approved). It
// mirrors the application feature's value, as the interview feature does.
const ApplicationInterviewEligible = "interview_eligible"

// RoleProfileConfirmed is the only Role Profile status a practice interview starts from.
const RoleProfileConfirmed = "confirmed"

// Environments are the interview rooms a Candidate can choose. The ids are the picker's room background ids,
// and the frontend file src/features/interview/components/room-backgrounds.ts stays their single source: a
// test reads that file and fails when this list drifts from it. No SQL constraint repeats the list.
var Environments = []string{"cafe-3d", "office-3d", "meeting-3d", "office", "balcony", "cafe"}

// ValidEnvironment reports whether id is in the catalog (exact match).
func ValidEnvironment(id string) bool { return slices.Contains(Environments, id) }

// BlockerCode names one reason a setup is not ready to start. The codes are stable API values.
type BlockerCode string

const (
	// Practice.
	BlockerRoleProfileNotConfirmed BlockerCode = "role_profile_not_confirmed"
	BlockerQuestionBankNotReady    BlockerCode = "question_bank_not_ready"
	BlockerAvatarRequired          BlockerCode = "avatar_required"
	BlockerAvatarUnavailable       BlockerCode = "avatar_unavailable"
	BlockerVoiceRequired           BlockerCode = "voice_required"
	BlockerVoiceUnavailable        BlockerCode = "voice_unavailable"
	// Both modes.
	BlockerEnvironmentRequired    BlockerCode = "environment_required"
	BlockerEnvironmentUnsupported BlockerCode = "environment_unsupported"
	// Recruitment.
	BlockerApplicationNotEligible  BlockerCode = "application_not_eligible"
	BlockerDeadlinePassed          BlockerCode = "interview_deadline_passed"
	BlockerInterviewStarted        BlockerCode = "interview_already_started"
	BlockerLockedAvatarUnavailable BlockerCode = "locked_avatar_unavailable"
	BlockerLockedVoiceUnavailable  BlockerCode = "locked_voice_unavailable"
)

// Blocker is one reason a setup is not ready. Field is the setup field it is about, when there is one.
type Blocker struct {
	Code  BlockerCode
	Field string
}

// Choice is an avatar or a voice a setup points at, with whether it can be used right now.
type Choice struct {
	ID     uuid.UUID
	Name   string
	Usable bool
}

// PracticeFacts is everything the readiness of a practice setup depends on, read in one snapshot. Avatar,
// Voice and Environment are the stored choices (nil: not chosen yet).
type PracticeFacts struct {
	Title       string
	Confirmed   bool
	BankReady   bool
	Avatar      *Choice
	Voice       *Choice
	Environment *string
	Saved       bool
	UpdatedAt   *time.Time
}

// RecruitmentFacts is the same for a recruitment setup. Avatar and Voice are the ones the Recruiter locked on
// the posting: read-only for the Candidate. Environment is the Candidate's stored choice.
type RecruitmentFacts struct {
	Title            string
	Status           string
	WithinDeadline   bool
	Deadline         *time.Time
	PostingAccepting bool
	Started          bool
	Avatar           *Choice
	Voice            *Choice
	Environment      *string
	Saved            bool
	UpdatedAt        *time.Time
}

// PracticeBlockers lists why a practice setup cannot start, in a stable order. Stored choices that went stale
// (a voice deactivated, an avatar no longer usable) are reported, never cleared. A deleted avatar or voice is
// already empty in storage (ON DELETE SET NULL), so it shows as required.
func PracticeBlockers(f PracticeFacts) []Blocker {
	blockers := []Blocker{}
	if !f.Confirmed {
		blockers = append(blockers, Blocker{Code: BlockerRoleProfileNotConfirmed})
	} else if !f.BankReady {
		blockers = append(blockers, Blocker{Code: BlockerQuestionBankNotReady})
	}
	blockers = appendChoice(blockers, f.Avatar, "avatar", BlockerAvatarRequired, BlockerAvatarUnavailable)
	blockers = appendChoice(blockers, f.Voice, "voice", BlockerVoiceRequired, BlockerVoiceUnavailable)
	return appendEnvironment(blockers, f.Environment)
}

// RecruitmentBlockers lists why a recruitment setup cannot start. Capacity is not one: the slot is consumed
// by the first start, which checks it.
func RecruitmentBlockers(f RecruitmentFacts) []Blocker {
	blockers := []Blocker{}
	switch {
	case f.Status != ApplicationInterviewEligible || !f.PostingAccepting:
		blockers = append(blockers, Blocker{Code: BlockerApplicationNotEligible})
	case !f.WithinDeadline:
		blockers = append(blockers, Blocker{Code: BlockerDeadlinePassed})
	}
	if f.Started {
		blockers = append(blockers, Blocker{Code: BlockerInterviewStarted})
	}
	if f.Avatar == nil || !f.Avatar.Usable {
		blockers = append(blockers, Blocker{Code: BlockerLockedAvatarUnavailable, Field: "avatar"})
	}
	if f.Voice == nil || !f.Voice.Usable {
		blockers = append(blockers, Blocker{Code: BlockerLockedVoiceUnavailable, Field: "voice"})
	}
	return appendEnvironment(blockers, f.Environment)
}

func appendChoice(blockers []Blocker, choice *Choice, field string, required, unavailable BlockerCode) []Blocker {
	switch {
	case choice == nil:
		return append(blockers, Blocker{Code: required, Field: field})
	case !choice.Usable:
		return append(blockers, Blocker{Code: unavailable, Field: field})
	}
	return blockers
}

func appendEnvironment(blockers []Blocker, environment *string) []Blocker {
	switch {
	case environment == nil:
		return append(blockers, Blocker{Code: BlockerEnvironmentRequired, Field: "environment"})
	case !ValidEnvironment(*environment):
		return append(blockers, Blocker{Code: BlockerEnvironmentUnsupported, Field: "environment"})
	}
	return blockers
}

var (
	ErrRoleProfileNotFound    = apperror.New(apperror.CodeJDNotFound, "Role Profile not found.")
	ErrRoleProfileRequired    = apperror.New(apperror.CodeRoleProfileRequired, "A confirmed Role Profile is required.")
	ErrQuestionBankNotReady   = apperror.New(apperror.CodeQuestionBankNotReady, "The question bank is not ready.")
	ErrApplicationNotFound    = apperror.New(apperror.CodeApplicationNotFound, "Application not found.")
	ErrApplicationNotEligible = apperror.New(apperror.CodeApplicationNotEligible,
		"The application is not eligible for a recruitment interview setup.")
	ErrAvatarNotFound = apperror.New(apperror.CodeAvatarNotFound, "Avatar not found.")
	ErrVoiceNotFound  = apperror.New(apperror.CodeVoiceNotFound, "Voice not found.")
)

// CheckPracticeSavable is the rule a practice setup is saved under: a confirmed Role Profile with a ready
// bank. The save never reads the bank itself, only whether it exists.
func CheckPracticeSavable(f PracticeFacts) error {
	if !f.Confirmed {
		return ErrRoleProfileRequired
	}
	if !f.BankReady {
		return ErrQuestionBankNotReady
	}
	return nil
}

// CheckRecruitmentSavable is the rule a recruitment setup is saved under: a CV-approved Application of an
// accepting posting, inside its deadline, with no interview started yet.
func CheckRecruitmentSavable(f RecruitmentFacts) error {
	if f.Status != ApplicationInterviewEligible || !f.PostingAccepting || !f.WithinDeadline || f.Started {
		return ErrApplicationNotEligible
	}
	return nil
}

// Setup is what the API returns: the chosen (or locked) interviewer and room, and the live readiness. It
// never carries core questions, bank size or evaluation weights.
type Setup struct {
	Mode              Mode
	RoleProfileID     *uuid.UUID
	ApplicationID     *uuid.UUID
	TargetTitle       string
	Avatar            *Choice
	Voice             *Choice
	Environment       *string
	LockedFields      []string
	InterviewDeadline *time.Time
	Saved             bool
	UpdatedAt         *time.Time
	Ready             bool
	Blockers          []Blocker
}

// WithBlockers sets the blockers and the readiness they decide: ready means no blockers. The list is never nil.
func (s Setup) WithBlockers(blockers []Blocker) Setup {
	if blockers == nil {
		blockers = []Blocker{}
	}
	s.Blockers = blockers
	s.Ready = len(blockers) == 0
	return s
}

// PracticeSetup builds the practice view of the facts.
func PracticeSetup(profileID uuid.UUID, f PracticeFacts) Setup {
	return Setup{
		Mode: ModePractice, RoleProfileID: &profileID, TargetTitle: f.Title,
		Avatar: f.Avatar, Voice: f.Voice, Environment: f.Environment, LockedFields: []string{},
		Saved: f.Saved, UpdatedAt: f.UpdatedAt,
	}.WithBlockers(PracticeBlockers(f))
}

// RecruitmentSetup builds the recruitment view of the facts: avatar and voice are the Recruiter's, locked.
func RecruitmentSetup(applicationID uuid.UUID, f RecruitmentFacts) Setup {
	return Setup{
		Mode: ModeRecruitment, ApplicationID: &applicationID, TargetTitle: f.Title,
		Avatar: f.Avatar, Voice: f.Voice, Environment: f.Environment, LockedFields: []string{"avatar", "voice"},
		InterviewDeadline: f.Deadline, Saved: f.Saved, UpdatedAt: f.UpdatedAt,
	}.WithBlockers(RecruitmentBlockers(f))
}

// PracticeInput is a practice save: a full replacement, where nil means not chosen yet.
type PracticeInput struct {
	AvatarID    *uuid.UUID
	VoiceID     *uuid.UUID
	Environment *string
}

// CheckEnvironment rejects a chosen environment that is not in the catalog. Nil (not chosen) is allowed.
func CheckEnvironment(environment *string) error {
	if environment != nil && !ValidEnvironment(*environment) {
		return apperror.New(apperror.CodeValidation, "environment is not a supported interview room")
	}
	return nil
}
