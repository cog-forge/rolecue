BEGIN;
-- DEV ROLLBACK ONLY: no password/session recovery. Reset DB or restore snapshot.
CREATE TABLE public.legacy_auth_accounts (
 id uuid PRIMARY KEY DEFAULT uuid_generate_v4(), email text UNIQUE NOT NULL,
 full_name text NOT NULL, password_hash varchar(128) NOT NULL DEFAULT '',
 role account_role NOT NULL DEFAULT 'participant', is_locked boolean NOT NULL DEFAULT false,
 created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now()
);
INSERT INTO public.legacy_auth_accounts (id, email, full_name, role, is_locked, created_at, updated_at)
SELECT id, email, name, CASE role WHEN 'admin' THEN 'admin'::account_role WHEN 'recruiter' THEN 'jury'::account_role ELSE 'participant'::account_role END, coalesce(is_locked, false), created_at, updated_at FROM public.users;
ALTER TABLE public.job_descriptions DROP CONSTRAINT job_descriptions_user_id_fkey;
ALTER TABLE public.interview_sessions DROP CONSTRAINT interview_sessions_user_id_fkey;
DROP TABLE public.two_factors;
DROP TABLE public.sessions;
DROP TABLE public.accounts;
DROP TABLE public.verifications;
DROP TABLE public.users;
ALTER TABLE public.legacy_auth_accounts RENAME TO accounts;
ALTER TABLE public.job_descriptions ADD CONSTRAINT job_descriptions_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.accounts(id) ON DELETE CASCADE;
ALTER TABLE public.interview_sessions ADD CONSTRAINT interview_sessions_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.accounts(id) ON DELETE CASCADE;
COMMIT;
