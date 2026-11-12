-- name: GetAccount :one
SELECT id, email, COALESCE(role, 'candidate')::text AS role
FROM public.users
WHERE id = @id;

-- Locks the user row for the erasure transaction. No row means the account is already gone.
-- name: LockAccountEmail :one
SELECT email
FROM public.users
WHERE id = @id
FOR UPDATE;

-- Live Job Postings ('open', 'intake_closed') and every Application on a Recruiter's postings, decided or not.
-- name: CountActiveWork :one
SELECT
  (SELECT count(*) FROM public.job_postings op WHERE op.recruiter_id = @id AND op.status IN ('open', 'intake_closed'))::bigint AS open_postings,
  (SELECT count(*) FROM public.applications a JOIN public.job_postings jp ON jp.id = a.job_posting_id
     WHERE jp.recruiter_id = @id)::bigint AS applications_in_progress;

-- Every object key the account owns: CVs, recordings, avatar models, JD PDFs and the keys of issued upload
-- grants. The interview subquery repeats ListOwnedInterviewIDs (sqlc has no shared fragments); keep the two
-- identical.
-- name: ListStorageKeys :many
SELECT a.cv_path AS key FROM public.applications a
 WHERE a.cv_path IS NOT NULL
   AND (a.candidate_id = @id OR a.job_posting_id IN (SELECT jp.id FROM public.job_postings jp WHERE jp.recruiter_id = @id))
UNION
SELECT i.record_path FROM public.interviews i
 WHERE i.record_path IS NOT NULL
   AND i.id IN (
     SELECT oi.id FROM public.interviews oi
     WHERE oi.application_id IN (
             SELECT a.id FROM public.applications a
             WHERE a.candidate_id = @id
                OR a.job_posting_id IN (SELECT jp.id FROM public.job_postings jp WHERE jp.recruiter_id = @id))
        OR oi.role_profile_id IN (SELECT rp.id FROM public.role_profiles rp WHERE rp.candidate_id = @id))
UNION
SELECT av.model_path FROM public.avatars av JOIN public.inventories inv ON inv.id = av.inventory_id WHERE inv.user_id = @id
UNION
SELECT jd.content_path FROM public.job_descriptions jd
 WHERE jd.content_path IS NOT NULL
   AND jd.id IN (SELECT rp.jd_id FROM public.role_profiles rp WHERE rp.candidate_id = @id
                 UNION SELECT jp.jd_id FROM public.job_postings jp WHERE jp.recruiter_id = @id)
UNION
SELECT g.key::text FROM public.upload_grants g WHERE g.owner_id = @id;

-- The interviews that disappear with the account: its own Applications, the Applications on its Job Postings
-- and its practice Role Profiles.
-- name: ListOwnedInterviewIDs :many
SELECT i.id
FROM public.interviews i
WHERE i.application_id IN (
        SELECT a.id FROM public.applications a
        WHERE a.candidate_id = @id
           OR a.job_posting_id IN (SELECT jp.id FROM public.job_postings jp WHERE jp.recruiter_id = @id))
   OR i.role_profile_id IN (SELECT rp.id FROM public.role_profiles rp WHERE rp.candidate_id = @id);

-- name: ListOwnedJobDescriptionIDs :many
SELECT rp.jd_id FROM public.role_profiles rp WHERE rp.candidate_id = @id
UNION SELECT jp.jd_id FROM public.job_postings jp WHERE jp.recruiter_id = @id;

-- An interview the Candidate used stays counted as a used slot of the Recruiter's posting: the number is kept
-- on the posting (no personal data) before the rows that carried it are deleted.
-- name: KeepUsedInterviewSlots :exec
UPDATE public.job_postings jp SET erased_interviews = jp.erased_interviews + used.n
FROM (SELECT a.job_posting_id, count(*)::int AS n
        FROM public.interviews i JOIN public.applications a ON a.id = i.application_id
       WHERE a.candidate_id = @id GROUP BY a.job_posting_id) used
WHERE jp.id = used.job_posting_id AND jp.recruiter_id <> @id;

-- The turns are tombstoned before they are deleted so the text is gone from any copy of the rows that
-- outlives the delete (replicas read at the snapshot, long-running readers).
-- name: TombstoneConversationTurns :exec
UPDATE public.conversation_turns SET question = @tombstone::text,
    answer = CASE WHEN answer IS NULL THEN NULL ELSE @tombstone::text END,
    feedback = CASE WHEN feedback IS NULL THEN NULL ELSE @tombstone::text END
WHERE interview_id = ANY(@interview_ids::uuid[]);

-- Criterion results quote the answers too (KAN-335): tombstoned, then deleted before the interview's
-- questions and turns they reference.
-- name: TombstoneCriterionScores :exec
UPDATE public.interview_criterion_scores SET
    evidence = CASE WHEN evidence IS NULL THEN NULL ELSE @tombstone::text END,
    feedback = CASE WHEN feedback IS NULL THEN NULL ELSE @tombstone::text END
WHERE interview_id = ANY(@interview_ids::uuid[]);

-- name: DeleteCriterionScores :exec
DELETE FROM public.interview_criterion_scores WHERE interview_id = ANY(@interview_ids::uuid[]);

-- name: DeleteScoreDetails :exec
DELETE FROM public.score_details WHERE interview_id = ANY(@interview_ids::uuid[]);

-- name: DeleteInterviewQuestions :exec
DELETE FROM public.interview_questions WHERE interview_id = ANY(@interview_ids::uuid[]);

-- name: DeleteConversationTurns :exec
DELETE FROM public.conversation_turns WHERE interview_id = ANY(@interview_ids::uuid[]);

-- name: DeleteInterviews :exec
DELETE FROM public.interviews WHERE id = ANY(@interview_ids::uuid[]);

-- Rows of other accounts that point at this account's avatars lose the pointer, not the row.
-- name: DetachInterviewAvatars :exec
UPDATE public.interviews SET avatar_id = NULL
WHERE avatar_id IN (SELECT av.id FROM public.avatars av JOIN public.inventories inv ON inv.id = av.inventory_id WHERE inv.user_id = @id);

-- name: DetachPostingRequiredAvatars :exec
UPDATE public.job_postings SET required_avatar_id = NULL
WHERE required_avatar_id IN (SELECT av.id FROM public.avatars av JOIN public.inventories inv ON inv.id = av.inventory_id WHERE inv.user_id = @id);

-- name: DeleteOwnedApplications :exec
DELETE FROM public.applications
WHERE candidate_id = @id OR job_posting_id IN (SELECT jp.id FROM public.job_postings jp WHERE jp.recruiter_id = @id);

-- name: DeleteOwnedPostingMetrics :exec
DELETE FROM public.metrics_percentage WHERE job_posting_id IN (SELECT jp.id FROM public.job_postings jp WHERE jp.recruiter_id = @id);

-- name: DeleteOwnedJobPostings :exec
DELETE FROM public.job_postings WHERE recruiter_id = @id;

-- name: DeleteOwnedRoleProfiles :exec
DELETE FROM public.role_profiles WHERE candidate_id = @id;

-- name: DeleteCoreQuestions :exec
DELETE FROM public.core_questions WHERE jd_id = ANY(@jd_ids::uuid[]);

-- name: DeleteRequirements :exec
DELETE FROM public.requirements WHERE jd_id = ANY(@jd_ids::uuid[]);

-- name: DeleteJobDescriptions :exec
DELETE FROM public.job_descriptions WHERE id = ANY(@jd_ids::uuid[]);

-- name: DeleteOwnedAvatars :exec
DELETE FROM public.avatars WHERE inventory_id IN (SELECT inv.id FROM public.inventories inv WHERE inv.user_id = @id);

-- name: DeleteOwnedInventories :exec
DELETE FROM public.inventories WHERE user_id = @id;

-- name: DeleteOwnedWallets :exec
DELETE FROM public.wallets WHERE user_id = @id;

-- name: DeleteOwnedVerifications :exec
DELETE FROM public.verifications WHERE user_id = @id OR lower(identifier) = lower(@email::text);

-- sessions, accounts, two_factors and upload_grants go with the user row (ON DELETE CASCADE).
-- name: DeleteAccount :exec
DELETE FROM public.users WHERE id = @id;
