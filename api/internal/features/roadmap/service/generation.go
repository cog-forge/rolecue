// Package service generates the content of a Role Profile's Learning Roadmap.
package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	questionbankdomain "github.com/cog-forge/rolecue/api/internal/features/questionbank/domain"
	"github.com/cog-forge/rolecue/api/internal/features/roadmap/domain"
	"github.com/cog-forge/rolecue/api/internal/features/roadmap/repository"
	"github.com/cog-forge/rolecue/api/internal/pkg/logger"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"
)

const (
	// DefaultLease is how long a generation may stay silent before another request may take it over. It is
	// longer than the two sequential model steps of a confirmation can take.
	DefaultLease = 5 * time.Minute

	candidateRole       = "candidate"
	topicsFailedMessage = "Roadmap generation failed."
	failWriteTimeout    = 5 * time.Second
)

// TopicGenerator performs one model attempt and must not retry internally. The input is untrusted data.
type TopicGenerator interface {
	GenerateTopics(ctx context.Context, in domain.TopicInput) (domain.TopicCandidate, error)
}

// Store is the persistence the generation needs; *repository.Repository implements it.
type Store interface {
	EnsureForProfile(ctx context.Context, candidate, profile uuid.UUID) (domain.Roadmap, error)
	ProfileState(ctx context.Context, candidate, profile uuid.UUID) (domain.ProfileState, error)
	Restore(ctx context.Context, roadmap uuid.UUID, fence domain.Fence) error
	GetItem(ctx context.Context, candidate, profile, item uuid.UUID) (domain.Item, error)
	InPracticeLock(ctx context.Context, candidate, profile uuid.UUID, fn func(ctx context.Context, tx pgx.Tx) error) error
	Claim(ctx context.Context, roadmap uuid.UUID, allowedFrom []domain.GenerationStatus, staleBefore time.Time) (domain.Fence, error)
	Fail(ctx context.Context, roadmap uuid.UUID, fence domain.Fence) error
	Publish(ctx context.Context, roadmap uuid.UUID, fence domain.Fence, items []domain.NewItem, hook repository.PublishHook) ([]domain.Item, error)
}

// InputReader reads the confirmed requirements, seniority and refinement of a job description.
type InputReader interface {
	GenerationInput(ctx context.Context, jdID uuid.UUID) (questionbankdomain.GenerationInput, error)
}

// BankDrafter generates, without storing, the practice bank of a profile mapped to the given topics.
type BankDrafter interface {
	DraftPracticeBank(ctx context.Context, candidateID, jdID uuid.UUID, topics []questionbankdomain.RoadmapTopic) ([]questionbankdomain.GeneratedQuestion, error)
	DraftTopicQuestions(ctx context.Context, candidateID, jdID uuid.UUID, topic questionbankdomain.RoadmapTopic, count int) ([]questionbankdomain.GeneratedQuestion, error)
}

// BankStore replaces the practice bank inside the transaction that publishes the topics.
type BankStore interface {
	ReplacePracticeBank(ctx context.Context, tx pgx.Tx, jdID uuid.UUID, questions []questionbankdomain.GeneratedQuestion) error
	AppendTopicQuestions(ctx context.Context, tx pgx.Tx, jdID, topicID uuid.UUID, total int, questions []questionbankdomain.GeneratedQuestion) (bool, error)
}

type Service struct {
	store      Store
	topics     TopicGenerator
	drafter    BankDrafter
	banks      BankStore
	inputs     InputReader
	maxRetries int
	lease      time.Duration
	now        func() time.Time
}

// New builds the generation service. maxRetries is the number of extra topic attempts after an unusable one
// (0 or 1, like the question bank).
func New(store Store, topics TopicGenerator, inputs InputReader, drafter BankDrafter, banks BankStore, maxRetries int) (*Service, error) {
	if store == nil || topics == nil || inputs == nil || drafter == nil || banks == nil {
		return nil, fmt.Errorf("roadmap generation requires a store, a topic generator, an input reader, a bank drafter and a bank store")
	}
	if maxRetries < 0 || maxRetries > 1 {
		return nil, fmt.Errorf("roadmap generation max retries must be 0 or 1")
	}
	return &Service{store: store, topics: topics, drafter: drafter, banks: banks, inputs: inputs, maxRetries: maxRetries, lease: DefaultLease, now: time.Now}, nil
}

// Prepare generates the topics and the practice bank of a profile whose requirements the candidate has just
// reviewed, and publishes both together (KAN-343, KAN-344). The caller has already checked ownership and that the review
// is valid. The model is never called inside a database transaction: the Roadmap is claimed (generating) and
// the claim's fence guards the final write, so a worker that lost its claim cannot overwrite a newer one.
//
//   - ready: nothing to do (a retried confirmation reuses the result of the first one);
//   - generating: another request is working on it, ROADMAP_GENERATING;
//   - pending or failed: claimed, generated, published (ready) or marked failed.
func (s *Service) Prepare(ctx context.Context, candidate, profile, jdID uuid.UUID) error {
	roadmap, err := s.store.EnsureForProfile(ctx, candidate, profile)
	if err != nil {
		return err
	}
	if roadmap.GenerationStatus == domain.StatusReady {
		logger.FromContext(ctx).Info("roadmap already ready, reusing it", logger.Op("roadmap.generate"), logger.String("profile_id", profile.String()))
		return nil
	}
	return s.run(ctx, candidate, profile, jdID, roadmap.ID, []domain.GenerationStatus{domain.StatusPending, domain.StatusFailed})
}

// run claims the Roadmap from one of the allowed states, generates and publishes, and settles the claim when
// the generation fails. A refused claim is explained by notClaimable.
func (s *Service) run(ctx context.Context, candidate, profile, jdID, roadmap uuid.UUID, allowed []domain.GenerationStatus) error {
	fence, err := s.store.Claim(ctx, roadmap, allowed, s.now().Add(-s.lease))
	if errors.Is(err, repository.ErrNotClaimable) {
		return s.notClaimable(ctx, candidate, profile)
	}
	if err != nil {
		return err
	}
	done := logger.Start(ctx, "roadmap.generate", logger.String("profile_id", profile.String()), logger.String("from_status", string(fence.Previous)))
	err = s.generate(ctx, candidate, jdID, roadmap, fence)
	done(err)
	if err != nil {
		s.settleFailure(ctx, roadmap, fence)
	}
	return err
}

// Retry generates the Roadmap of a confirmed profile whose first generation did not happen or failed (KAN-345).
// It is idempotent: a Roadmap that is ready is returned untouched (a double click, or a lost response), and one
// that another request is generating is a conflict, so concurrent retries start one generation. A profile that
// already has an interview keeps its bank, so its Roadmap cannot be generated now.
func (s *Service) Retry(ctx context.Context, candidate uuid.UUID, role string, profile uuid.UUID) (domain.Roadmap, error) {
	state, roadmap, err := s.entry(ctx, candidate, role, profile)
	if err != nil {
		return domain.Roadmap{}, err
	}
	if roadmap.GenerationStatus == domain.StatusReady {
		return roadmap, nil
	}
	if state.HasSessions {
		return domain.Roadmap{}, apperror.New(apperror.CodeRoadmapNotRetryable, "This learning roadmap can no longer be generated because practice has already started.")
	}
	if err = s.run(ctx, candidate, profile, state.JDID, roadmap.ID, []domain.GenerationStatus{domain.StatusPending, domain.StatusFailed}); err != nil {
		return domain.Roadmap{}, err
	}
	return s.store.EnsureForProfile(ctx, candidate, profile)
}

// Regenerate replaces the topics and the bank of a Roadmap before the first practice session of its profile
// (KAN-345). The Roadmap keeps its id. The old set stays until the new, validated one is committed in a single
// transaction under the profile's practice start lock, so a start and a regeneration exclude each other:
// whichever commits first wins, and no session can reference a question that was replaced. A regeneration that
// fails leaves the old set and puts the Roadmap back to ready.
func (s *Service) Regenerate(ctx context.Context, candidate uuid.UUID, role string, profile uuid.UUID) (domain.Roadmap, error) {
	state, roadmap, err := s.entry(ctx, candidate, role, profile)
	if err != nil {
		return domain.Roadmap{}, err
	}
	if state.HasSessions {
		return domain.Roadmap{}, apperror.New(apperror.CodeRoadmapRegenerateNotAllowed, "This learning roadmap can no longer be regenerated because practice has already started.")
	}
	allowed := []domain.GenerationStatus{domain.StatusReady, domain.StatusFailed, domain.StatusPending}
	if err = s.run(ctx, candidate, profile, state.JDID, roadmap.ID, allowed); err != nil {
		return domain.Roadmap{}, err
	}
	return s.store.EnsureForProfile(ctx, candidate, profile)
}

// entry checks what Retry and Regenerate share: a candidate, a profile of theirs whose requirements are
// confirmed, and its Roadmap.
func (s *Service) entry(ctx context.Context, candidate uuid.UUID, role string, profile uuid.UUID) (domain.ProfileState, domain.Roadmap, error) {
	if role != candidateRole {
		return domain.ProfileState{}, domain.Roadmap{}, apperror.New(apperror.CodeForbidden, "Only candidates have a learning roadmap.")
	}
	state, err := s.store.ProfileState(ctx, candidate, profile)
	if err != nil {
		return domain.ProfileState{}, domain.Roadmap{}, err
	}
	if !state.Confirmed {
		return domain.ProfileState{}, domain.Roadmap{}, apperror.New(apperror.CodeRequirementsNotConfirmed, "Confirm the requirements before generating the learning roadmap.")
	}
	roadmap, err := s.store.EnsureForProfile(ctx, candidate, profile)
	if err != nil {
		return domain.ProfileState{}, domain.Roadmap{}, err
	}
	return state, roadmap, nil
}

// notClaimable explains a refused claim by reading the Roadmap again: someone finished it meanwhile (reuse)
// or is still generating it (conflict).
func (s *Service) notClaimable(ctx context.Context, candidate, profile uuid.UUID) error {
	roadmap, err := s.store.EnsureForProfile(ctx, candidate, profile)
	if err != nil {
		return err
	}
	if roadmap.GenerationStatus == domain.StatusReady {
		return nil
	}
	return apperror.New(apperror.CodeRoadmapGenerating, "The learning roadmap is already being prepared.")
}

func (s *Service) generate(ctx context.Context, candidate, jdID, roadmap uuid.UUID, fence domain.Fence) error {
	input, err := s.inputs.GenerationInput(ctx, jdID)
	if err != nil {
		return err
	}
	if len(input.Requirements) == 0 {
		return apperror.New(apperror.CodeRequirementsNotConfirmed, "confirm the requirements before generating the learning roadmap")
	}
	topics, err := s.generateTopics(ctx, topicInput(input))
	if err != nil {
		return err
	}
	// The backend gives every topic its id now, so the bank can be generated for exactly these topics.
	refs := make([]questionbankdomain.RoadmapTopic, len(topics))
	for i := range topics {
		topics[i].ID = uuid.New()
		refs[i] = questionbankdomain.RoadmapTopic{ID: topics[i].ID, Title: topics[i].Title, Description: topics[i].Description}
	}
	questions, err := s.drafter.DraftPracticeBank(ctx, candidate, jdID, refs)
	if err != nil {
		return err
	}
	// Topics, bank and the ready status are published in one transaction: a Roadmap is never ready without
	// its bank, and an old bank is deleted only by the commit that replaces it.
	hook := func(ctx context.Context, tx pgx.Tx, _ []domain.Item) error {
		return s.banks.ReplacePracticeBank(ctx, tx, jdID, questions)
	}
	if _, err = s.store.Publish(ctx, roadmap, fence, topics, hook); err != nil {
		switch {
		case errors.Is(err, repository.ErrFenceLost):
			return apperror.Wrap(apperror.CodeRoadmapGenerating, "The learning roadmap was prepared by another request.", err)
		case errors.Is(err, questionbankdomain.ErrBankInUse):
			return apperror.Wrap(apperror.CodeRoadmapRegenerateNotAllowed, "This learning roadmap can no longer be regenerated.", err)
		}
		return err
	}
	return nil
}

// settleFailure records a failed generation on a fresh context: the request may have been cancelled, which is
// often why the generation failed. A Roadmap that was ready before the claim (a regeneration) goes back to
// ready, because its old topics and bank were never touched; any other goes to failed. A lost fence means a
// newer generation owns the Roadmap, which is left alone.
func (s *Service) settleFailure(ctx context.Context, roadmap uuid.UUID, fence domain.Fence) {
	cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), failWriteTimeout)
	defer cancel()
	settle := s.store.Fail
	if fence.Previous == domain.StatusReady {
		settle = s.store.Restore
	}
	if err := settle(cleanup, roadmap, fence); err != nil && !errors.Is(err, repository.ErrFenceLost) {
		logger.FromContext(ctx).Warn("roadmap failure could not be recorded", logger.Op("roadmap.generate"), logger.Cause(err))
	}
}

func topicInput(in questionbankdomain.GenerationInput) domain.TopicInput {
	out := domain.TopicInput{SeniorityName: in.Seniority.Name, SeniorityDescription: in.Seniority.Description, RefinementNote: in.RefinementNote}
	for _, requirement := range in.Requirements {
		out.Requirements = append(out.Requirements, domain.TopicRequirement{Name: requirement.Name, Target: requirement.Target,
			Type: requirement.Type, EvidenceText: requirement.EvidenceText})
	}
	return out
}

// generateTopics mirrors the question bank retry: one shared budget that includes proposals failing
// validation, a retry only for invalid output, and invalid output is never fed back to the model.
func (s *Service) generateTopics(ctx context.Context, input domain.TopicInput) ([]domain.NewItem, error) {
	fail := func(cause error) error {
		return apperror.Wrap(apperror.CodeRoadmapGenerationFailed, topicsFailedMessage, cause)
	}
	for attempt := 0; ; attempt++ {
		if err := ctx.Err(); err != nil {
			return nil, fail(err)
		}
		candidate, err := s.topics.GenerateTopics(ctx, input)
		if ctx.Err() != nil {
			return nil, fail(ctx.Err())
		}
		if err == nil {
			var items []domain.NewItem
			if items, err = validateTopics(candidate); err == nil {
				logger.FromContext(ctx).Debug("roadmap topics accepted", logger.Op("roadmap.generate"), logger.Int("attempt", attempt+1), logger.Int("topics", len(items)))
				return items, nil
			}
		}
		if !isInvalidTopicOutput(err) {
			err = fail(err)
		}
		willRetry := attempt < s.maxRetries && isInvalidTopicOutput(err)
		fields := []zap.Field{logger.Op("roadmap.generate"), logger.Int("attempt", attempt+1), zap.Bool("will_retry", willRetry), logger.Cause(err)}
		if willRetry {
			logger.FromContext(ctx).Warn("roadmap topic attempt rejected, retrying", fields...)
		} else {
			logger.FromContext(ctx).Debug("roadmap topic attempt rejected", fields...)
			return nil, err
		}
		if !waitBeforeRetry(ctx) {
			return nil, fail(ctx.Err())
		}
	}
}

func waitBeforeRetry(ctx context.Context) bool {
	timer := time.NewTimer(100 * time.Millisecond)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
