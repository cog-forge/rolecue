package provider

import (
	"context"
	"github.com/cog-forge/rolecue/api/internal/config"
	jdprovider "github.com/cog-forge/rolecue/api/internal/features/jd/provider"
	"github.com/cog-forge/rolecue/api/internal/features/jd/repository"
	"github.com/cog-forge/rolecue/api/internal/features/jd/service"
	qbservice "github.com/cog-forge/rolecue/api/internal/features/questionbank/service"
	"github.com/cog-forge/rolecue/api/internal/handler"
	"github.com/cog-forge/rolecue/api/internal/pkg/ai"
	"github.com/cog-forge/rolecue/api/internal/pkg/logger"
	"github.com/cog-forge/rolecue/api/internal/pkg/storage"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func ProvideJDRepository(pool *pgxpool.Pool) *repository.Repository {
	return repository.NewRepository(pool)
}

// jdQuestionBank adapts the question-bank service to what the JD service needs, so the JD
// feature doesn't depend on question-bank types.
type jdQuestionBank struct{ svc *qbservice.Service }

func (b jdQuestionBank) GenerateForPractice(ctx context.Context, candidateID, jdID uuid.UUID) error {
	_, err := b.svc.GenerateForPractice(ctx, candidateID, jdID)
	return err
}

func (b jdQuestionBank) Enabled() bool { return b.svc.Enabled() }

func ProvideJDService(cfg *config.Config, repo *repository.Repository, store storage.Store, bank *qbservice.Service, log *logger.Logger) (*service.Application, error) {
	model, err := ai.NewChatModel(context.Background(), cfg.LLM)
	if err != nil {
		return nil, err
	}
	extractor, err := jdprovider.New(model)
	if err != nil {
		return nil, err
	}
	extraction, err := service.New(extractor, cfg.LLM.MaxRetries)
	if err != nil {
		return nil, err
	}
	return service.NewApplication(extraction, repo, store, jdQuestionBank{svc: bank}, log)
}
func ProvideJDHandler(app *service.Application) *handler.JDHandler { return handler.NewJDHandler(app) }
