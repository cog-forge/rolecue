package provider

import (
	"github.com/cog-forge/rolecue/api/internal/config"
	"github.com/cog-forge/rolecue/api/internal/pkg/llmbudget"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ProvideLLMBudget is the per-user daily LLM budget (KAN-278) every LLM-backed operation charges.
func ProvideLLMBudget(cfg *config.Config, pool *pgxpool.Pool) llmbudget.Guard {
	return llmbudget.New(llmbudget.NewPostgresStore(pool), cfg.LLM.UserDailyCallBudget)
}
