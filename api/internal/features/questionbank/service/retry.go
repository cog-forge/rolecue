package service

import (
	"context"
	"errors"
	"time"

	evaluationdomain "github.com/cog-forge/rolecue/api/internal/features/evaluation/domain"
	"github.com/cog-forge/rolecue/api/internal/features/questionbank/domain"
	"github.com/cog-forge/rolecue/api/internal/pkg/logger"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

const generationFailedMessage = "Question generation failed."

// generateWithRetry mirrors the JD extraction retry: one shared budget that includes proposals that
// fail validation, a retry only for invalid output, and invalid output is never fed back to the model.
//
// A proposal that is valid but whose rubrics cannot cover every active metric within the session question
// limit (KAN-377, KAN-378) counts as unsuccessful output too: it is retried while the budget lasts, never
// with an extra call, and when the attempts run out the best proposal seen (the one covering the most
// metrics) is returned instead of an error, so the bank is still saved and the coverage in the result is the
// warning. Only a proposal that is itself invalid fails the generation, and only when no valid proposal was
// seen before it.
func (s *Service) generateWithRetry(ctx context.Context, in domain.GenerationInput, forPosting bool) ([]domain.GeneratedQuestion, error) {
	fail := func(cause error) error {
		return apperror.Wrap(apperror.CodeQuestionGenerationFailed, generationFailedMessage, cause)
	}
	// A Roadmap bank is generated large enough for every topic to reach its minimum, and each topic is given
	// its share of it; the validation below still checks what the model actually wrote.
	size := s.size
	if len(in.Topics) > 0 {
		size = domain.PracticeBankSize(s.size, len(in.Topics))
		if in.Size > 0 {
			size = max(in.Size, domain.MinQuestionsPerTopic*len(in.Topics))
		}
		in.Topics = domain.BalancedQuotas(in.Topics, size)
	}
	var best []domain.GeneratedQuestion
	bestCovered := -1
	for attempt := 0; ; attempt++ {
		if err := ctx.Err(); err != nil {
			return nil, fail(err)
		}
		candidate, err := s.generator.Generate(ctx, in, size)
		if ctx.Err() != nil {
			return nil, fail(ctx.Err())
		}
		if err == nil {
			var questions []domain.GeneratedQuestion
			questions, err = validateCandidate(in, candidate, size)
			if err == nil {
				err = ValidateBank(in, questions)
			}
			if err == nil {
				coverage, coverErr := s.candidateCoverage(in, questions)
				if coverErr != nil {
					// The check itself failed (more metrics than it can search): it must not cost the bank.
					logger.FromContext(ctx).Warn("question generation coverage check failed", logger.Op("questionbank.generate"),
						logger.Int("attempt", attempt+1), logger.Cause(coverErr))
					return questions, nil
				}
				if coverage.Passes {
					logger.FromContext(ctx).Debug("question generation attempt accepted", logger.Op("questionbank.generate"), logger.Int("attempt", attempt+1))
					return questions, nil
				}
				if coverage.Covered > bestCovered {
					best, bestCovered = questions, coverage.Covered
				}
				willRetry := attempt < s.maxRetries
				logUncoveredAttempt(ctx, attempt, willRetry, coverage, forPosting)
				if !willRetry {
					return best, nil
				}
				if !waitBeforeRetry(ctx) {
					return nil, fail(ctx.Err())
				}
				continue
			}
		}
		if !isInvalidOutput(err) {
			err = fail(err)
		}
		willRetry := attempt < s.maxRetries && isInvalidOutput(err)
		logRejectedAttempt(ctx, attempt, willRetry, len(candidate.Questions), err)
		if !willRetry {
			if best != nil {
				return best, nil
			}
			return nil, err
		}
		if !waitBeforeRetry(ctx) {
			return nil, fail(ctx.Err())
		}
	}
}

// candidateCoverage applies the shared coverage rule to a validated proposal: each question covers the
// metrics of the rubric the model chose for it, read from the full criteria of the allowed rubrics (the
// prompt only carries a bounded summary of them).
func (s *Service) candidateCoverage(in domain.GenerationInput, questions []domain.GeneratedQuestion) (evaluationdomain.Coverage, error) {
	metricsOf := make(map[uuid.UUID][]uuid.UUID, len(in.Rubrics))
	for _, rubric := range in.Rubrics {
		metricsOf[rubric.ID] = rubric.MetricIDs()
	}
	bank := make([]evaluationdomain.CoverageQuestion, 0, len(questions))
	for i, question := range questions {
		item := evaluationdomain.CoverageQuestion{ID: uuid.UUID{byte(i >> 8), byte(i)}}
		if question.RubricID != nil {
			item.Metrics = metricsOf[*question.RubricID]
		}
		bank = append(bank, item)
	}
	return evaluationdomain.CheckCoverage(bank, in.ActiveMetrics, s.questionLimit)
}

// waitBeforeRetry pauses briefly between attempts. It reports false when the context ended meanwhile.
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

// logUncoveredAttempt records a valid proposal that cannot cover every active metric. Never question text,
// rubric or metric names. The counts are logged for a Recruiter's posting only: a practice bank belongs to a
// Candidate, whose logs carry no coverage detail.
func logUncoveredAttempt(ctx context.Context, attempt int, willRetry bool, coverage evaluationdomain.Coverage, forPosting bool) {
	fields := []zap.Field{logger.Op("questionbank.generate"), logger.Int("attempt", attempt+1), zap.Bool("will_retry", willRetry)}
	if forPosting {
		fields = append(fields, logger.Int("covered", coverage.Covered), logger.Int("total", coverage.Total))
	}
	if willRetry {
		logger.FromContext(ctx).Warn("question generation attempt cannot cover every metric, retrying", fields...)
		return
	}
	logger.FromContext(ctx).Warn("question generation attempts ran out, saving the widest bank", fields...)
}

// logRejectedAttempt records a rejected proposal with its rule and counts, never the question text.
// A retried rejection is swallowed, so it is logged at Warn; a final one is returned to the caller.
func logRejectedAttempt(ctx context.Context, attempt int, willRetry bool, proposed int, err error) {
	fields := []zap.Field{logger.Op("questionbank.generate"), logger.Int("attempt", attempt+1), zap.Bool("will_retry", willRetry),
		logger.Int("count", proposed), logger.Cause(err)}
	if willRetry {
		logger.FromContext(ctx).Warn("question generation attempt rejected, retrying", fields...)
		return
	}
	logger.FromContext(ctx).Debug("question generation attempt rejected", fields...)
}

func isInvalidOutput(err error) bool {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	var appErr *apperror.AppError
	return errors.As(err, &appErr) && appErr != nil && appErr.Code == apperror.CodeInvalidQuestionGenerationOutput
}
