-- Least-privilege runtime role for the api and auth services (KAN-286).
--
-- The `migrate` service and this script run as the owner role (DB_USERNAME) and own every table.
-- api and auth connect as `rolecue_app`, which gets data access only: no DDL, no role or database
-- management, no access to other schemas. A SQL injection or application bug can then no longer
-- drop tables or read other databases.
--
-- Run once as the owner after the first migration, and again only to rotate the password:
--   docker exec -i rolecue-postgres psql -U "$DB_USERNAME" -d "$DB_DATABASE" \
--     -v ON_ERROR_STOP=1 -v app_password="$APP_DB_PASSWORD" < .docker/postgres/roles.sql
-- Then set APP_DATABASE_URL=postgres://rolecue_app:<password>@postgres:5432/<db>?sslmode=disable
-- in .docker/deploy.env. Leaving APP_DATABASE_URL empty keeps api and auth on DATABASE_URL.

SELECT format('CREATE ROLE rolecue_app LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION PASSWORD %L', :'app_password')
WHERE NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'rolecue_app')
\gexec

SELECT format('ALTER ROLE rolecue_app PASSWORD %L', :'app_password')
\gexec

SELECT format('REVOKE ALL ON DATABASE %I FROM PUBLIC', current_database())
\gexec
SELECT format('GRANT CONNECT ON DATABASE %I TO rolecue_app', current_database())
\gexec

REVOKE CREATE ON SCHEMA public FROM PUBLIC;
GRANT USAGE ON SCHEMA public TO rolecue_app;
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO rolecue_app;
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO rolecue_app;

-- Tables and sequences created by later migrations (run by this owner) get the same grants.
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO rolecue_app;
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT USAGE, SELECT ON SEQUENCES TO rolecue_app;
