-- Database-backed auth rate limits (KAN-255). Additive only: one new table, no existing table changes.
BEGIN;

-- Better Auth counts requests per key (client IP plus path) in last_request milliseconds since epoch,
-- so last_request is bigint. key is unique because it is the lookup key for each counter. id follows
-- the other auth tables: Better Auth writes every model row with an id.
CREATE TABLE public.rate_limits (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    key text NOT NULL UNIQUE,
    count integer NOT NULL,
    last_request bigint NOT NULL
);

-- Better Auth deletes rows whose window has ended; this index keeps that cleanup cheap.
CREATE INDEX rate_limits_last_request_idx ON public.rate_limits(last_request);

COMMIT;
