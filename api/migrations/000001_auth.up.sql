BEGIN;

CREATE TABLE public.users (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name text NOT NULL,
    email text NOT NULL UNIQUE,
    email_verified boolean NOT NULL DEFAULT false,
    image text,
    created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    username text,
    display_username text,
    role text DEFAULT 'candidate',
    is_locked boolean DEFAULT false,
    lock_reason text,
    lock_expires_at timestamptz,
    two_factor_enabled boolean DEFAULT false,
    company_name text,
    company_website text,
    CONSTRAINT users_role_check CHECK (role IN ('candidate', 'recruiter', 'admin'))
);

CREATE TABLE public.sessions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    expires_at timestamptz NOT NULL,
    token text NOT NULL UNIQUE,
    created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamptz NOT NULL,
    ip_address text,
    user_agent text,
    user_id uuid NOT NULL REFERENCES public.users(id) ON DELETE CASCADE,
    impersonated_by text
);
CREATE INDEX sessions_user_id_idx ON public.sessions(user_id);

CREATE TABLE public.accounts (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    account_id text NOT NULL,
    provider_id text NOT NULL,
    user_id uuid NOT NULL REFERENCES public.users(id) ON DELETE CASCADE,
    access_token text,
    refresh_token text,
    id_token text,
    access_token_expires_at timestamptz,
    refresh_token_expires_at timestamptz,
    scope text,
    password text,
    created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamptz NOT NULL
);
CREATE INDEX accounts_user_id_idx ON public.accounts(user_id);

CREATE TABLE public.verifications (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    identifier text NOT NULL,
    value text NOT NULL,
    expires_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    user_id uuid REFERENCES public.users(id)
);
CREATE INDEX verifications_identifier_idx ON public.verifications(identifier);

CREATE TABLE public.two_factors (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    secret text NOT NULL,
    backup_codes text NOT NULL,
    user_id uuid NOT NULL REFERENCES public.users(id) ON DELETE CASCADE,
    verified boolean,
    failed_verification_count integer,
    locked_until timestamptz
);
CREATE INDEX two_factors_secret_idx ON public.two_factors(secret);
CREATE INDEX two_factors_user_id_idx ON public.two_factors(user_id);

COMMIT;
