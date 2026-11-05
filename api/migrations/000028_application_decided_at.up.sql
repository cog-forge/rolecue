-- Decision time of an Application (KAN-305). Set once when it reaches a terminal status (approved or
-- rejected); the recording retention clock reads it instead of updated_at, which any later edit moves.
-- Rows already decided are backfilled from updated_at, the value the retention clock used until now.
BEGIN;

ALTER TABLE public.applications ADD COLUMN decided_at timestamptz;

UPDATE public.applications SET decided_at = updated_at WHERE status IN ('approved', 'rejected');

COMMIT;
