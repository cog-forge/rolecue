-- name: ConsumeLLMCall :one
-- Counts one LLM call for the user on the given UTC day unless the day's budget is spent. A limit of 0
-- means unlimited. It returns no row when the budget is spent, so the cap never overshoots under
-- concurrent requests.
INSERT INTO public.llm_usage AS u (user_id, day, calls)
VALUES (@user_id, @day, 1)
ON CONFLICT (user_id, day) DO UPDATE
    SET calls = u.calls + 1, updated_at = CURRENT_TIMESTAMP
    WHERE @call_limit::int = 0 OR u.calls < @call_limit::int
RETURNING u.calls;

-- name: GetLLMUsage :one
SELECT calls FROM public.llm_usage WHERE user_id = @user_id AND day = @day;
