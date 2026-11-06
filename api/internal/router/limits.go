package router

import (
	"github.com/gin-gonic/gin"

	"github.com/cog-forge/rolecue/api/internal/config"
	"github.com/cog-forge/rolecue/api/internal/middleware"
	"github.com/cog-forge/rolecue/api/internal/pkg/llmbudget"
)

// WithLLMBudget sets the per-user daily LLM budget (KAN-278) charged on the routes that start an LLM call.
// Without one (a nil guard) those routes are not budgeted.
func (r *Router) WithLLMBudget(guard llmbudget.Guard) *Router {
	r.llmBudget = guard
	return r
}

// requestLimits are the middleware chains of the routes that cost money (KAN-277, KAN-278). Each chain is
// ordered cheapest check first: the client address, then the user, then the daily budget, so a request
// refused by a rate limit never burns budget. An empty chain adds nothing.
type requestLimits struct {
	// llm is for routes that start an LLM call: per-IP and per-user rate limits, then the daily budget.
	llm []gin.HandlerFunc
	// llmOwnLimiter is for the LLM route that carries its own per-user limiter and pays for its LLM work
	// elsewhere: per-IP only. Completing an interview is paid by the budget reserved when the session
	// started (and by the service for a retried evaluation), so the live socket and the abandoned-session
	// job are covered the same way as this HTTP route.
	llmOwnLimiter []gin.HandlerFunc
	// expensive is for routes that cost money or heavy work without an LLM call: per-IP and per-user.
	expensive []gin.HandlerFunc
}

func (r *Router) requestLimits() requestLimits {
	var ipFor = func(perMinute, burst int) gin.HandlerFunc {
		return middleware.NewIPRateLimiter(middleware.PerMinute(perMinute), burst).Middleware()
	}
	var userFor = func(perMinute, burst int) gin.HandlerFunc {
		return middleware.NewUserRateLimiter(middleware.PerMinute(perMinute), burst).Middleware()
	}
	cfg := r.cfg.RateLimit
	return buildRequestLimits(cfg, ipFor, userFor, r.llmBudget)
}

func buildRequestLimits(cfg config.RateLimitConfig, ipFor, userFor func(perMinute, burst int) gin.HandlerFunc, budget llmbudget.Guard) requestLimits {
	var limits requestLimits
	// One IP limiter per chain family, so the LLM and expensive routes do not drain each other's bucket.
	if cfg.Enabled && cfg.IPPerMinute > 0 {
		limits.llm = append(limits.llm, ipFor(cfg.IPPerMinute, cfg.IPBurst))
		limits.llmOwnLimiter = append(limits.llmOwnLimiter, ipFor(cfg.IPPerMinute, cfg.IPBurst))
		limits.expensive = append(limits.expensive, ipFor(cfg.IPPerMinute, cfg.IPBurst))
	}
	if cfg.Enabled && cfg.LLMPerMinute > 0 {
		limits.llm = append(limits.llm, userFor(cfg.LLMPerMinute, cfg.LLMBurst))
	}
	if cfg.Enabled && cfg.ExpensivePerMinute > 0 {
		limits.expensive = append(limits.expensive, userFor(cfg.ExpensivePerMinute, cfg.ExpensiveBurst))
	}
	if budget != nil {
		charge := middleware.LLMBudgetMiddleware(budget)
		limits.llm = append(limits.llm, charge)
	}
	return limits
}

// with returns the chain followed by the route's handler, ready to spread into a route registration.
func with(chain []gin.HandlerFunc, handler gin.HandlerFunc) []gin.HandlerFunc {
	out := make([]gin.HandlerFunc, 0, len(chain)+1)
	out = append(out, chain...)
	return append(out, handler)
}
