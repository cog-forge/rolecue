-- Least-privilege runtime role for the api and auth services on prod (the compose twin is .docker/postgres/roles.sql).
--
-- Run as the master user (rolecue_admin, DATABASE_URL_MIGRATE) after the migrations; it is idempotent, so re-running it
-- is safe and also rotates the password. Every table, sequence and function is owned by rolecue_admin, because that is
-- the role that runs the migrations. rolecue_app gets data access only: no DDL, no role or database management.
-- Use it through the one-off task helper, which keeps the password off every command line and out of the logs:
--   scripts/prod/run-once.sh app-role --sql scripts/prod/sql/00-app-role.sql -- \
--     'printf %s "$SQL_B64" | base64 -d | gunzip | psql "$DATABASE_URL_MIGRATE" -X -q -v ON_ERROR_STOP=1 -f -'
--
-- App schemas: only `public`. Every table, type and function in api/migrations (better-auth's tables included) is
-- schema-qualified public.*; no migration creates a schema or an extension. The report at the end proves that on the
-- live database. If a migration ever adds a schema, add it to the three groups below (USAGE, grants, default
-- privileges).
--
-- No EXECUTE grant is needed: the two functions are trigger functions (public.core_question_item_same_profile and
-- public.interview_focus_same_profile) and fire without the caller holding EXECUTE, and the app's own calls
-- (pg_advisory_xact_lock, hashtextextended) live in pg_catalog.

\set ON_ERROR_STOP on
\set VERBOSITY terse

-- The password comes from the environment, never from a command line. An unset or empty one aborts before anything
-- changes (an empty one would create a role with no password).
\getenv app_password APP_DB_PASSWORD
\if :{?app_password}
\else
  \set app_password ''
\endif
SELECT (:'app_password' = '') AS password_missing \gset
\if :password_missing
  DO $$ BEGIN RAISE EXCEPTION 'APP_DB_PASSWORD is not set or empty; nothing was changed'; END $$;
\endif

BEGIN;

-- Create the role if it is missing, then always (re)set the password. The attributes are set only at creation: on RDS
-- the master user is rds_superuser, not a real superuser, so it may not re-assert NOSUPERUSER or NOREPLICATION on an
-- existing role (permission denied to alter role). A role that already exists must therefore already be right, and the
-- check below refuses to continue if it is not.
SELECT 'CREATE ROLE rolecue_app LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION'
WHERE NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'rolecue_app')
\gexec
DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'rolecue_app'
             AND (NOT rolcanlogin OR rolsuper OR rolcreatedb OR rolcreaterole OR rolreplication OR rolbypassrls)) THEN
    RAISE EXCEPTION 'rolecue_app exists with unexpected attributes; fix or drop it by hand';
  END IF;
END $$;
SELECT format('ALTER ROLE rolecue_app PASSWORD %L', :'app_password')
\gexec

-- Database: nobody gets in or creates temporary tables through PUBLIC, only the roles named explicitly.
-- (The owner and rds_superuser keep their own access.)
SELECT format('REVOKE ALL ON DATABASE %I FROM PUBLIC', current_database())
\gexec
SELECT format('GRANT CONNECT ON DATABASE %I TO rolecue_app', current_database())
\gexec

-- Schema public.
REVOKE CREATE ON SCHEMA public FROM PUBLIC;
GRANT USAGE ON SCHEMA public TO rolecue_app;
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO rolecue_app;
GRANT USAGE, SELECT, UPDATE ON ALL SEQUENCES IN SCHEMA public TO rolecue_app;

-- The migrate tool's bookkeeping table is read-only for the app: auth's /readyz runs `SELECT dirty FROM
-- schema_migrations` to wait for a clean, applied schema. Only migrate (rolecue_admin) may change it.
SELECT 'REVOKE INSERT, UPDATE, DELETE ON TABLE public.schema_migrations FROM rolecue_app'
WHERE to_regclass('public.schema_migrations') IS NOT NULL
\gexec

-- Tables and sequences that later migrations create (they run as rolecue_admin) get the same grants.
ALTER DEFAULT PRIVILEGES FOR ROLE rolecue_admin IN SCHEMA public
  GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO rolecue_app;
ALTER DEFAULT PRIVILEGES FOR ROLE rolecue_admin IN SCHEMA public
  GRANT USAGE, SELECT, UPDATE ON SEQUENCES TO rolecue_app;

COMMIT;

-- Evidence (read-only): which schemas hold anything, and who owns it. Expected: public only, owner rolecue_admin.
\echo '--- objects per schema, kind and owner ---'
SELECT n.nspname AS schema,
       CASE c.relkind WHEN 'r' THEN 'table' WHEN 'p' THEN 'partitioned table' WHEN 'S' THEN 'sequence'
                      WHEN 'v' THEN 'view' WHEN 'm' THEN 'matview' WHEN 'f' THEN 'foreign table' END AS kind,
       pg_get_userbyid(c.relowner) AS owner,
       count(*) AS objects
FROM pg_class c
JOIN pg_namespace n ON n.oid = c.relnamespace
WHERE n.nspname !~ '^pg_' AND n.nspname <> 'information_schema' AND c.relkind IN ('r', 'p', 'S', 'v', 'm', 'f')
GROUP BY 1, 2, 3
ORDER BY 1, 2, 3;
\echo '--- schemas, extensions, functions outside pg_catalog ---'
SELECT 'schema' AS what, nspname AS name, pg_get_userbyid(nspowner) AS owner
FROM pg_namespace WHERE nspname !~ '^pg_' AND nspname <> 'information_schema'
UNION ALL
SELECT 'extension', extname, pg_get_userbyid(extowner) FROM pg_extension
UNION ALL
SELECT 'function', n.nspname || '.' || p.proname, pg_get_userbyid(p.proowner)
FROM pg_proc p JOIN pg_namespace n ON n.oid = p.pronamespace
WHERE n.nspname !~ '^pg_' AND n.nspname <> 'information_schema'
ORDER BY 1, 2;
\echo '--- rolecue_app: attributes and tables without full data privileges (expected: schema_migrations only) ---'
SELECT rolname, rolcanlogin, rolsuper, rolcreatedb, rolcreaterole, rolreplication, rolinherit FROM pg_roles WHERE rolname = 'rolecue_app';
SELECT c.relname AS table_without_full_dml
FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
WHERE n.nspname = 'public' AND c.relkind IN ('r', 'p')
  AND NOT (has_table_privilege('rolecue_app', c.oid, 'SELECT') AND has_table_privilege('rolecue_app', c.oid, 'INSERT')
           AND has_table_privilege('rolecue_app', c.oid, 'UPDATE') AND has_table_privilege('rolecue_app', c.oid, 'DELETE'))
ORDER BY 1;
