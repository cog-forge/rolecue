package ai

import (
	"context"
	"math"
	"time"

	"go.uber.org/zap"

	"github.com/cog-forge/rolecue/api/internal/config"
	id "github.com/cog-forge/rolecue/api/internal/features/interview/domain"
	ip "github.com/cog-forge/rolecue/api/internal/features/interview/provider"
	is "github.com/cog-forge/rolecue/api/internal/features/interview/service"
	"github.com/cog-forge/rolecue/api/internal/pkg/logger"
)

// clefTask names the decision step in attempt logs, apart from the follow_up chat route.
const clefTask = "follow_up_decision"

// clefDeclinedReason is the internal reason of a follow-up Clef declined; it is never shown to the Candidate.
const clefDeclinedReason = "The decision model judged the answer complete enough to move on."

// ClefFollowUpDecider lets Clef decide whether a follow-up is asked (KAN-274). Clef owns the decision: below
// the threshold nothing else is called; at or above it the follow-up chat route only words the question and
// cannot decline. When Clef fails (down, timeout, 429, rejected credentials or invalid output) the chat route
// decides and words as before, within the rest of the live budget. The service still bounds the whole call,
// validates the proposal and charges one budget unit for it.
type ClefFollowUpDecider struct {
	clef      *clefClient
	threshold float64
	timeout   time.Duration
	llm       *TaskAdapter
}

var _ is.FollowUpDecider = (*ClefFollowUpDecider)(nil)

func newClefFollowUpDecider(clef *clefClient, threshold float64, timeout time.Duration, llm *TaskAdapter) *ClefFollowUpDecider {
	return &ClefFollowUpDecider{clef: clef, threshold: threshold, timeout: timeout, llm: llm}
}

// NewClefFollowUpDecider builds the decider from config. It fails when the decision route is off or the
// follow-up chat route cannot be built; the caller then keeps the chat decider or turns follow-ups off.
func NewClefFollowUpDecider(ctx context.Context, cfg *config.Config) (*ClefFollowUpDecider, error) {
	r := cfg.AI.FollowUpDecisionRoute()
	if r.Provider != config.DecisionProviderClef {
		return nil, errClefNotConfigured
	}
	llm, err := NewTaskAdapter(ctx, cfg, config.TaskFollowUp)
	if err != nil {
		return nil, err
	}
	client := newClefClient(cloudflareAPIBase, cfg.AI.CloudflareAccountID, cfg.AI.CloudflareAPIToken, r.Model, r.Timeout)
	return newClefFollowUpDecider(client, r.Threshold, r.Timeout, llm), nil
}

func (d *ClefFollowUpDecider) Decide(ctx context.Context, in id.FollowUpContext) (id.FollowUpProposal, error) {
	p, err := d.probability(ctx, in)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return id.FollowUpProposal{}, ctxErr
		}
		return d.llm.Decide(ctx, in)
	}
	if p < d.threshold {
		return id.FollowUpProposal{FollowUp: false, Reason: clefDeclinedReason}, nil
	}
	return d.llm.Word(ctx, in)
}

// probability asks Clef within its own timeout, at most half of what the caller has left, so a slow Clef
// always leaves time for the fallback. The attempt log carries numbers only.
func (d *ClefFollowUpDecider) probability(ctx context.Context, in id.FollowUpContext) (float64, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	budget := d.timeout
	if deadline, ok := ctx.Deadline(); ok {
		if half := time.Until(deadline) / 2; half < budget {
			budget = half
		}
	}
	attemptCtx, cancel := context.WithTimeout(ctx, budget)
	defer cancel()
	done := logger.Start(ctx, "llm.task_attempt", logger.String("provider", config.DecisionProviderClef),
		logger.String("task", clefTask), logger.String("model", d.clef.model))
	q := ip.FollowUpDecisionQuestion(in.Guideline)
	p, usage, err := d.clef.YesProbability(attemptCtx, ip.FollowUpState(in), ClefYesNo{Instructions: q.Instructions, Yes: q.Yes, No: q.No})
	if err != nil && attemptCtx.Err() != nil && ctx.Err() == nil {
		err = context.DeadlineExceeded
	}
	fields := append(failureFields(err), logger.Int("input_tokens", usage.InputTokens), logger.Int("output_tokens", usage.OutputTokens))
	if err == nil {
		fields = append(fields, zap.Float64("probability", math.Round(p*100)/100), zap.Bool("ask", p >= d.threshold))
	}
	done(sanitizedFailure(err), fields...)
	return p, err
}
