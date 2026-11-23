package ai

import (
	"context"
	"errors"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cog-forge/rolecue/api/internal/config"
	ed "github.com/cog-forge/rolecue/api/internal/features/evaluation/domain"
	ep "github.com/cog-forge/rolecue/api/internal/features/evaluation/provider"
	es "github.com/cog-forge/rolecue/api/internal/features/evaluation/service"
	id "github.com/cog-forge/rolecue/api/internal/features/interview/domain"
	ip "github.com/cog-forge/rolecue/api/internal/features/interview/provider"
	is "github.com/cog-forge/rolecue/api/internal/features/interview/service"
	qd "github.com/cog-forge/rolecue/api/internal/features/questionbank/domain"
	qp "github.com/cog-forge/rolecue/api/internal/features/questionbank/provider"
	qs "github.com/cog-forge/rolecue/api/internal/features/questionbank/service"
	md "github.com/cog-forge/rolecue/api/internal/features/roadmap/domain"
	mp "github.com/cog-forge/rolecue/api/internal/features/roadmap/provider"
	ms "github.com/cog-forge/rolecue/api/internal/features/roadmap/service"
	rd "github.com/cog-forge/rolecue/api/internal/features/roleprofile/domain"
	rp "github.com/cog-forge/rolecue/api/internal/features/roleprofile/provider"
	rs "github.com/cog-forge/rolecue/api/internal/features/roleprofile/service"
	"github.com/cog-forge/rolecue/api/internal/pkg/llmerr"
	"github.com/cog-forge/rolecue/api/internal/pkg/logger"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
)

type TaskAdapter struct {
	route *Route
	// topicCheck verifies the topic of each Roadmap bank question with Clef (KAN-384); nil means off.
	topicCheck *clefTopicCheck
	// retry overrides the generation retry policy; tests use it. Nil means generationRetry.
	retry *llmerr.Policy
}

const (
	// generationRetries is how many times one generation call (a topic proposal or one bank batch) is repeated
	// after a transient failure of the whole route: every provider of it failed with a timeout, a 429, a 5xx,
	// a network error or an unusable reply. Retries stay inside the call's absolute budget.
	generationRetries = 2
	// generationMinRemaining is the budget a repeat needs to have a chance: the route splits what is left
	// between its providers, so less than this cannot complete a call.
	generationMinRemaining = 8 * time.Second
)

// generationRetry is the retry policy of generation (KAN-382): bounded, backing off 1 s then 2 s, and never
// started when the budget has no room for it.
func (a *TaskAdapter) generationRetry() llmerr.Policy {
	if a.retry != nil {
		return *a.retry
	}
	return llmerr.Policy{
		MaxRetries:   generationRetries,
		Backoff:      llmerr.Exponential(time.Second, 4*time.Second),
		MinRemaining: generationMinRemaining,
		Retryable:    llmerr.TransientOrInvalid,
	}
}

// retried repeats one routed generation call under the generation policy. A legacy route keeps its single
// call: the service above it owns that retry.
func retried[T any](ctx context.Context, a *TaskAdapter, call func(context.Context) (T, error)) (T, error) {
	if a.route.legacy {
		return call(ctx)
	}
	policy := a.generationRetry()
	policy.OnRetry = func(retry int, err error) {
		logger.FromContext(ctx).Warn("llm generation call failed, retrying", logger.Op("ai.retry"),
			logger.String("task", string(a.route.task)), logger.Int("retry", retry), logger.String("reason", llmerr.Reason(err)))
	}
	return llmerr.Do(ctx, policy, call)
}

func NewTaskAdapter(ctx context.Context, cfg *config.Config, task config.Task) (*TaskAdapter, error) {
	r, err := NewRoute(ctx, cfg, task)
	if err != nil {
		return nil, err
	}
	adapter := &TaskAdapter{route: r}
	if task == config.TaskBank {
		adapter.topicCheck = newTopicCheckFromConfig(ctx, cfg)
	}
	return adapter, nil
}

// outputError keeps transport errors classified, and marks decoder/domain failures uniformly.
func outputError(err error) error {
	if err == nil {
		return nil
	}
	if canFallback(err) || llmFailureReason(err) == "auth_rejected" || llmFailureReason(err) == "http_4xx" || errors.Is(err, context.Canceled) {
		return err
	}
	return errors.Join(ErrInvalidOutput, err)
}
func (a *TaskAdapter) Extract(ctx context.Context, jd string) (rd.ExtractionCandidate, error) {
	return Run(ctx, a.route, func(ctx context.Context, m model.BaseChatModel) (rd.ExtractionCandidate, error) {
		e, err := rp.New(m)
		if err != nil {
			return rd.ExtractionCandidate{}, err
		}
		c, err := e.Extract(ctx, jd)
		if err == nil {
			var structured rd.StructuredJD
			structured, err = rs.ValidateCandidate(c)
			if err == nil {
				var review rd.RoleReview
				review, err = rs.ValidateRoleReview(rs.ReviewFromExtraction(structured), false)
				if err == nil {
					err = rs.ValidateEvidence(review, jd)
				}
			}
		}
		if a.route.legacy {
			return c, err
		}
		return c, outputError(err)
	})
}

// GenerateTopics proposes the topics of a Learning Roadmap (KAN-343). Like every task it is one routed call:
// the reply is decoded and validated inside each provider attempt, so an unusable proposal is retried or falls
// back instead of reaching the caller.
func (a *TaskAdapter) GenerateTopics(ctx context.Context, in md.TopicInput) (md.TopicCandidate, error) {
	if !a.route.legacy {
		// One budget for the proposal and its repeats, like a bank, so the topic step cannot take the time
		// the bank step needs inside the caller's deadline.
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, a.route.budget)
		defer cancel()
	}
	return retried(ctx, a, func(ctx context.Context) (md.TopicCandidate, error) {
		return a.generateTopicsOnce(ctx, in)
	})
}

func (a *TaskAdapter) generateTopicsOnce(ctx context.Context, in md.TopicInput) (md.TopicCandidate, error) {
	return Run(ctx, a.route, func(ctx context.Context, m model.BaseChatModel) (md.TopicCandidate, error) {
		g, err := mp.New(m)
		if err != nil {
			return md.TopicCandidate{}, err
		}
		c, err := g.GenerateTopics(ctx, in)
		if err == nil {
			_, err = ms.ValidateTopics(c)
		}
		if a.route.legacy {
			return c, err
		}
		return c, outputError(err)
	})
}
func (a *TaskAdapter) Decide(ctx context.Context, in id.FollowUpContext) (id.FollowUpProposal, error) {
	return a.followUp(ctx, in, ip.New)
}

// Word words a follow-up that a decision model already decided to ask (KAN-274): the same route, decoder and
// validator as Decide, with a prompt whose reply must ask.
func (a *TaskAdapter) Word(ctx context.Context, in id.FollowUpContext) (id.FollowUpProposal, error) {
	return a.followUp(ctx, in, ip.NewWording)
}

func (a *TaskAdapter) followUp(ctx context.Context, in id.FollowUpContext, build func(model.BaseChatModel) (*ip.EinoFollowUpDecider, error)) (id.FollowUpProposal, error) {
	return Run(ctx, a.route, func(ctx context.Context, m model.BaseChatModel) (id.FollowUpProposal, error) {
		d, err := build(m)
		if err != nil {
			return id.FollowUpProposal{}, err
		}
		p, err := d.Decide(ctx, in)
		if err == nil {
			err = is.ValidateFollowUp(&p, in)
		}
		if a.route.legacy {
			return p, err
		}
		return p, outputError(err)
	})
}

// GradeQuestion grades one core question against its rubric (KAN-334). Each question is its own routed call:
// the reply is decoded and validated inside every provider attempt, so an unusable reply is retried or falls
// back, and one bad question never reruns the others.
func (a *TaskAdapter) GradeQuestion(ctx context.Context, in ed.QuestionGradeInput) (ed.QuestionGrade, error) {
	return Run(ctx, a.route, func(ctx context.Context, m model.BaseChatModel) (ed.QuestionGrade, error) {
		g, err := ep.NewGrader(m)
		if err != nil {
			return ed.QuestionGrade{}, err
		}
		candidate, err := g.Grade(ctx, in)
		var grade ed.QuestionGrade
		if err == nil {
			grade, err = es.ValidateQuestionGrade(in, candidate)
		}
		logRejectedGrade(ctx, err, in.QuestionPosition)
		if a.route.legacy {
			return grade, err
		}
		return grade, outputError(err)
	})
}
func (a *TaskAdapter) Generate(ctx context.Context, in qd.GenerationInput, count int) (qd.GenerationCandidate, error) {
	if !a.route.legacy {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, a.route.budget)
		defer cancel()
	}
	// Legacy Gemini-only routing keeps the single-call route, but every output now passes the same domain
	// validators as the routed path (KAN-300).
	batchSize := count
	for _, entry := range a.route.entries {
		if entry.config.Provider == "groq" && batchSize > 10 {
			batchSize = 10
		}
	}
	merged := qd.GenerationCandidate{Questions: []qd.GeneratedQuestion{}}
	// Questions the topic check dropped (KAN-384) are excluded from later batches like stored ones, so a refill
	// does not write them again. drops counts the batches that lost questions; it bounds the refills.
	var rejected []string
	drops := 0
	// The bank is filled until it holds count questions: a batch that lost off-topic questions leaves the topics
	// short, and the next batch owes them the difference (BatchQuotas). The loop ends because every batch either
	// adds size questions or uses one of the maxTopicRefills drops.
	for len(merged.Questions) < count {
		size := min(batchSize, count-len(merged.Questions))
		// A failed batch is repeated on its own: the batches already accepted stay in merged and are not asked
		// again, and nothing is stored before the complete bank is published.
		batch, err := retried(ctx, a, func(ctx context.Context) (qd.GenerationCandidate, error) {
			return Run(ctx, a.route, func(ctx context.Context, m model.BaseChatModel) (qd.GenerationCandidate, error) {
				g, err := qp.New(m)
				if err != nil {
					return qd.GenerationCandidate{}, err
				}
				batchIn := in
				if len(in.Topics) > 0 {
					// A Roadmap bank is written in batches that each owe the topics still short of their quota,
					// so the distribution does not depend on one reply covering everything (KAN-344).
					batchIn.Topics = qd.BatchQuotas(in.Topics, qd.CountByTopic(merged.Questions), size)
				}
				c, err := g.GenerateBatch(ctx, batchIn, size, append(append(append([]string{}, in.Existing...), merged.Contents()...), rejected...))
				if err == nil && len(in.Topics) > 0 {
					// Each batch must write the questions it owes every topic; one that does not is invalid
					// output of this attempt, so it is retried or falls back without redoing earlier batches.
					c.Questions, err = qs.FitBatchToTopics(c.Questions, batchIn.Topics)
				}
				if err == nil {
					combined := qd.GenerationCandidate{Questions: append(append([]qd.GeneratedQuestion{}, merged.Questions...), c.Questions...)}
					var valid []qd.GeneratedQuestion
					valid, err = qs.ValidateGenerated(in, combined, len(merged.Questions)+size)
					if err == nil {
						c.Questions = valid[len(merged.Questions):]
					}
				}
				logRejectedBankOutput(ctx, err, len(merged.Questions), size, len(c.Questions))
				if a.route.legacy {
					return c, err
				}
				return c, outputError(err)
			})
		})
		if err != nil {
			return qd.GenerationCandidate{}, err
		}
		kept, dropped := a.checkTopics(ctx, in.Topics, batch.Questions, drops)
		if len(dropped) > 0 {
			drops++
			rejected = append(rejected, dropped...)
		}
		merged.Questions = append(merged.Questions, kept...)
	}
	_, err := qs.ValidateGenerated(in, merged, count)
	if err == nil {
		// The distribution can only be judged on the complete bank, never on a batch of it.
		err = qs.ValidateBank(in, merged.Questions)
	}
	return merged, err
}

// logRejectedBankOutput records why a batch of generated questions was refused: the rule that failed and the
// counts, so an invalid_output from a provider can be diagnosed. The causes built by the decoder and the
// validators name rules and positions only; they never contain question or answer text.
func logRejectedBankOutput(ctx context.Context, err error, accepted, requested, proposed int) {
	var appErr *apperror.AppError
	if !errors.As(err, &appErr) || appErr == nil || appErr.Code != apperror.CodeInvalidQuestionGenerationOutput {
		return
	}
	logger.FromContext(ctx).Warn("question bank batch rejected", logger.Op("ai.question_bank.validate"),
		logger.Int("accepted", accepted), logger.Int("requested", requested), logger.Int("proposed", proposed), logger.Cause(err))
}

// logRejectedGrade records why a grading reply was refused: the rule that failed, never reply, answer, rubric
// or expected-answer text. The decoder and validator causes name rules and criterion keys only (KAN-334).
func logRejectedGrade(ctx context.Context, err error, question int32) {
	if err == nil {
		return
	}
	var appErr *apperror.AppError
	rule := ""
	switch {
	case es.IsInvalidGrade(err):
		rule = err.Error()
	case errors.As(err, &appErr) && appErr != nil && appErr.Err != nil && appErr.Message == ep.InvalidGradeMessage:
		rule = appErr.Err.Error()
	default:
		return
	}
	logger.FromContext(ctx).Warn("grading reply rejected", logger.Op("ai.evaluation.validate"),
		logger.Int("question_position", int(question)), logger.String("rule", rule))
}
