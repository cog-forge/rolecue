-- Audit of Recruiter reads of recordings and transcripts (KAN-296). One row per successful read, written
-- with the read. Additive only: no existing table changes. The ids are plain columns, not foreign keys: an
-- audit trail must outlive the data and the account it records, so erasure does not remove it.
BEGIN;

CREATE TABLE public.recruiter_access_audit (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    recruiter_id uuid NOT NULL,
    application_id uuid NOT NULL,
    interview_id uuid NOT NULL,
    action text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT recruiter_access_audit_action_check CHECK (action IN ('recording_view', 'transcript_view'))
);

CREATE INDEX recruiter_access_audit_created_at_idx ON public.recruiter_access_audit (created_at DESC, id DESC);
CREATE INDEX recruiter_access_audit_recruiter_id_idx ON public.recruiter_access_audit (recruiter_id);
CREATE INDEX recruiter_access_audit_application_id_idx ON public.recruiter_access_audit (application_id);
CREATE INDEX recruiter_access_audit_interview_id_idx ON public.recruiter_access_audit (interview_id);

COMMIT;
