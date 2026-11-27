-- Verification of the least-privilege role: run it connected AS rolecue_app (DATABASE_URL_APP). Exit code 0 means every
-- expectation held (ON_ERROR_STOP aborts on the first one that did not). Only counts and flags are read, never row data.
--   scripts/prod/run-once.sh app-role-verify --sql scripts/prod/sql/01-verify-app-role.sql -- \
--     'printf %s "$SQL_B64" | base64 -d | gunzip | psql "$DATABASE_URL_APP" -X -q -v ON_ERROR_STOP=1 -f -'

\set ON_ERROR_STOP on
\set VERBOSITY terse

\echo '--- session ---'
SELECT current_user AS connected_as,
       s.ssl, s.version AS tls_version, s.cipher
FROM pg_stat_ssl s WHERE s.pid = pg_backend_pid();

\echo '--- role attributes ---'
SELECT rolname, rolcanlogin, rolsuper, rolcreatedb, rolcreaterole, rolreplication, rolbypassrls
FROM pg_roles WHERE rolname = current_user;

\echo '--- reads: one table per app schema (only public is in use) ---'
SELECT 'public.users' AS "table", count(*) AS "rows" FROM public.users;
SELECT 'public.job_postings' AS "table", count(*) AS "rows" FROM public.job_postings;
\echo '--- auth /readyz query: SELECT dirty FROM schema_migrations (must succeed, one clean row) ---'
DO $$
DECLARE n int; bad boolean;
BEGIN
  SELECT count(*), COALESCE(bool_or(dirty), false) INTO n, bad FROM public.schema_migrations;
  IF n <> 1 OR bad THEN
    RAISE EXCEPTION 'schema_migrations is not one clean row (rows=%, dirty=%)', n, bad;
  END IF;
  RAISE NOTICE 'ok: SELECT dirty FROM schema_migrations succeeds (1 row, dirty = false)';
END $$;

\echo '--- expected failures (each must be permission denied) ---'
DO $$
BEGIN
  BEGIN
    CREATE TABLE public.rolecue_app_probe (id int);
    DROP TABLE public.rolecue_app_probe;
    RAISE EXCEPTION 'UNEXPECTED: CREATE TABLE succeeded';
  EXCEPTION WHEN insufficient_privilege THEN
    RAISE NOTICE 'ok: CREATE TABLE in public is denied (insufficient_privilege)';
  END;
  BEGIN
    CREATE TEMP TABLE rolecue_app_probe (id int);
    RAISE EXCEPTION 'UNEXPECTED: CREATE TEMP TABLE succeeded';
  EXCEPTION WHEN insufficient_privilege THEN
    RAISE NOTICE 'ok: CREATE TEMP TABLE is denied (insufficient_privilege)';
  END;
  BEGIN
    CREATE SCHEMA rolecue_app_probe;
    RAISE EXCEPTION 'UNEXPECTED: CREATE SCHEMA succeeded';
  EXCEPTION WHEN insufficient_privilege THEN
    RAISE NOTICE 'ok: CREATE SCHEMA is denied (insufficient_privilege)';
  END;
  IF to_regclass('public.schema_migrations') IS NULL THEN
    RAISE EXCEPTION 'public.schema_migrations does not exist';
  END IF;
  BEGIN
    UPDATE public.schema_migrations SET dirty = dirty;
    RAISE EXCEPTION 'UNEXPECTED: UPDATE on schema_migrations succeeded';
  EXCEPTION WHEN insufficient_privilege THEN
    RAISE NOTICE 'ok: UPDATE on public.schema_migrations is denied (insufficient_privilege)';
  END;
  BEGIN
    DELETE FROM public.schema_migrations;
    RAISE EXCEPTION 'UNEXPECTED: DELETE on schema_migrations succeeded';
  EXCEPTION WHEN insufficient_privilege THEN
    RAISE NOTICE 'ok: DELETE on public.schema_migrations is denied (insufficient_privilege)';
  END;
END $$;
