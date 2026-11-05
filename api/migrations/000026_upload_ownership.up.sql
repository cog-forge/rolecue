-- Upload ownership (KAN-263). Each uploaded PDF key is bound to the user it was issued to and can
-- be attached once. Additive only: existing tables are untouched. Keys uploaded before this
-- migration have no row here and are refused when attached; the upload is repeated (see
-- internal/pkg/uploadgrant).
BEGIN;

CREATE TABLE public.upload_grants (
    key uuid PRIMARY KEY,
    owner_id uuid NOT NULL REFERENCES public.users(id) ON DELETE CASCADE,
    purpose text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    consumed_at timestamptz,
    CONSTRAINT upload_grants_purpose_check CHECK (purpose IN ('role_profile_pdf', 'job_posting_pdf'))
);

CREATE INDEX upload_grants_owner_id_idx ON public.upload_grants (owner_id);

COMMIT;
