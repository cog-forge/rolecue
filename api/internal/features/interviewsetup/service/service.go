// Package service holds the pre-interview setup use cases (KAN-231): save, read and revalidate the avatar,
// voice and environment a Candidate chose for a practice or a recruitment interview.
//
// Nothing here starts a session. Starting still goes through POST /interviews, which keeps the coin debit, the
// recruitment slot and the interviews row for itself; a setup only remembers choices and says whether they are
// ready. POST /interviews calls Practice or Recruitment before a new session starts and refuses one that is not
// ready.
package service

import (
	"context"

	"github.com/cog-forge/rolecue/api/internal/features/interviewsetup/domain"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

const candidateRole = "candidate"

// Repository is what the use cases need from storage.
type Repository interface {
	Practice(ctx context.Context, candidateID, profileID uuid.UUID) (domain.PracticeFacts, error)
	Recruitment(ctx context.Context, candidateID, applicationID uuid.UUID) (domain.RecruitmentFacts, error)
	SavePractice(ctx context.Context, candidateID, profileID uuid.UUID, in domain.PracticeInput) error
	SaveRecruitment(ctx context.Context, applicationID uuid.UUID, environment *string) error
}

type Service struct {
	repository Repository
}

func New(repository Repository) *Service {
	return &Service{repository: repository}
}

func requireCandidate(role string) error {
	if role != candidateRole {
		return apperror.New(apperror.CodeForbidden, "Interview setup is available to candidates.")
	}
	return nil
}

// Practice reads the practice setup of a Role Profile the Candidate owns and revalidates it against the data
// as it is now: every call recomputes the blockers, writes nothing and never clears a stored choice that went
// stale. It is also the check POST /interviews calls before it starts a new practice session (ready == no
// blockers).
func (s *Service) Practice(ctx context.Context, userID uuid.UUID, role string, profileID uuid.UUID) (domain.Setup, error) {
	if err := requireCandidate(role); err != nil {
		return domain.Setup{}, err
	}
	facts, err := s.repository.Practice(ctx, userID, profileID)
	if err != nil {
		return domain.Setup{}, err
	}
	return domain.PracticeSetup(profileID, facts), nil
}

// SavePractice replaces the practice setup (nil: not chosen yet) and returns it revalidated. A foreign or
// missing Role Profile, an unconfirmed one, a profile without a ready bank, an avatar outside the Candidate's
// inventory, an unknown or inactive voice and an environment outside the catalog are refused and nothing is
// saved. Saving the same values again changes nothing.
func (s *Service) SavePractice(ctx context.Context, userID uuid.UUID, role string, profileID uuid.UUID, in domain.PracticeInput) (domain.Setup, error) {
	if err := requireCandidate(role); err != nil {
		return domain.Setup{}, err
	}
	facts, err := s.repository.Practice(ctx, userID, profileID)
	if err != nil {
		return domain.Setup{}, err
	}
	if err := domain.CheckPracticeSavable(facts); err != nil {
		return domain.Setup{}, err
	}
	if err := domain.CheckEnvironment(in.Environment); err != nil {
		return domain.Setup{}, err
	}
	if err := s.repository.SavePractice(ctx, userID, profileID, in); err != nil {
		return domain.Setup{}, err
	}
	return s.Practice(ctx, userID, role, profileID)
}

// Recruitment reads the recruitment setup of an Application the Candidate owns: the avatar and voice the
// Recruiter locked (read-only), the Candidate's environment and the live blockers. It writes nothing and is
// the check a start flow calls before a recruitment start.
func (s *Service) Recruitment(ctx context.Context, userID uuid.UUID, role string, applicationID uuid.UUID) (domain.Setup, error) {
	if err := requireCandidate(role); err != nil {
		return domain.Setup{}, err
	}
	facts, err := s.repository.Recruitment(ctx, userID, applicationID)
	if err != nil {
		return domain.Setup{}, err
	}
	return domain.RecruitmentSetup(applicationID, facts), nil
}

// SaveRecruitment replaces the Candidate's environment of a recruitment setup and returns it revalidated. Only
// a CV-approved Application of an accepting posting, inside its deadline and with no interview started, can
// be saved; the locked avatar and voice are not part of the request.
func (s *Service) SaveRecruitment(ctx context.Context, userID uuid.UUID, role string, applicationID uuid.UUID, environment *string) (domain.Setup, error) {
	if err := requireCandidate(role); err != nil {
		return domain.Setup{}, err
	}
	facts, err := s.repository.Recruitment(ctx, userID, applicationID)
	if err != nil {
		return domain.Setup{}, err
	}
	if err := domain.CheckRecruitmentSavable(facts); err != nil {
		return domain.Setup{}, err
	}
	if err := domain.CheckEnvironment(environment); err != nil {
		return domain.Setup{}, err
	}
	if err := s.repository.SaveRecruitment(ctx, applicationID, environment); err != nil {
		return domain.Setup{}, err
	}
	return s.Recruitment(ctx, userID, role, applicationID)
}
