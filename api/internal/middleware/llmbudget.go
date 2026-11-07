package middleware

import (
	"context"
	"errors"

	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/cog-forge/rolecue/api/pkg/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// LLMBudget is the part of llmbudget.Guard the middleware needs.
type LLMBudget interface {
	Consume(ctx context.Context, userID uuid.UUID) error
}

// LLMBudgetMiddleware charges one call of the user's daily LLM budget (KAN-278) before the handler runs
// and answers 429 LLM_BUDGET_EXCEEDED when the budget is spent. Put it on the routes that start an LLM
// call, after RequireAuth. A nil budget lets every request through. A request that fails validation
// afterwards still counted, which is why the default budget is generous.
func LLMBudgetMiddleware(budget LLMBudget) gin.HandlerFunc {
	return func(c *gin.Context) {
		if budget == nil {
			c.Next()
			return
		}
		userID, ok := CurrentUserID(c)
		if !ok {
			response.Error(c, apperror.Wrap(apperror.CodeInvalidToken, "authentication required", errors.New("no authenticated user in context")))
			c.Abort()
			return
		}
		if err := budget.Consume(c.Request.Context(), userID); err != nil {
			response.Error(c, err)
			c.Abort()
			return
		}
		c.Next()
	}
}
