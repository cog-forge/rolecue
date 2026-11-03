package ai

import (
	"context"
	"errors"

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
	rd "github.com/cog-forge/rolecue/api/internal/features/roleprofile/domain"
	rp "github.com/cog-forge/rolecue/api/internal/features/roleprofile/provider"
	rs "github.com/cog-forge/rolecue/api/internal/features/roleprofile/service"
)

type TaskAdapter struct{ route *Route }

func NewTaskAdapter(ctx context.Context, cfg *config.Config, task config.Task) (*TaskAdapter, error) {
	r, err := NewRoute(ctx, cfg, task)
	if err != nil {
		return nil, err
	}
	return &TaskAdapter{r}, nil
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
		if err == nil && !a.route.legacy {
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
func (a *TaskAdapter) Decide(ctx context.Context, in id.FollowUpContext) (id.FollowUpProposal, error) {
	return Run(ctx, a.route, func(ctx context.Context, m model.BaseChatModel) (id.FollowUpProposal, error) {
		d, err := ip.New(m)
		if err != nil {
			return id.FollowUpProposal{}, err
		}
		p, err := d.Decide(ctx, in)
		if err == nil && !a.route.legacy {
			err = is.ValidateFollowUp(&p, in)
		}
		if a.route.legacy {
			return p, err
		}
		return p, outputError(err)
	})
}
func (a *TaskAdapter) Score(ctx context.Context, in ed.ScoreInput) (ed.ScoreOutput, error) {
	return Run(ctx, a.route, func(ctx context.Context, m model.BaseChatModel) (ed.ScoreOutput, error) {
		s, err := ep.New(m)
		if err != nil {
			return ed.ScoreOutput{}, err
		}
		out, err := s.Score(ctx, in)
		if err == nil && !a.route.legacy {
			err = es.ValidateScores(in, out)
		}
		if a.route.legacy {
			return out, err
		}
		return out, outputError(err)
	})
}
func (a *TaskAdapter) Generate(ctx context.Context, in qd.GenerationInput, count int) (qd.GenerationCandidate, error) {
	if !a.route.legacy {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, a.route.budget)
		defer cancel()
	}
	// Legacy Gemini-only routing keeps the original single-call behavior.
	batchSize := count
	for _, entry := range a.route.entries {
		if entry.config.Provider == "groq" && batchSize > 10 {
			batchSize = 10
		}
	}
	merged := qd.GenerationCandidate{Questions: []string{}}
	for offset := 0; offset < count; offset += batchSize {
		size := min(batchSize, count-offset)
		batch, err := Run(ctx, a.route, func(ctx context.Context, m model.BaseChatModel) (qd.GenerationCandidate, error) {
			g, err := qp.New(m)
			if err != nil {
				return qd.GenerationCandidate{}, err
			}
			c, err := g.GenerateBatch(ctx, in, size, merged.Questions)
			if err == nil && !a.route.legacy {
				combined := qd.GenerationCandidate{Questions: append(append([]string{}, merged.Questions...), c.Questions...)}
				var valid []string
				valid, err = qs.ValidateGenerated(combined, len(merged.Questions)+size)
				if err == nil {
					c.Questions = valid[len(merged.Questions):]
				}
			}
			if a.route.legacy {
				return c, err
			}
			return c, outputError(err)
		})
		if err != nil {
			return qd.GenerationCandidate{}, err
		}
		merged.Questions = append(merged.Questions, batch.Questions...)
	}
	if a.route.legacy {
		return merged, nil
	}
	_, err := qs.ValidateGenerated(merged, count)
	return merged, err
}
