-- Only the locked interviewer of an owned, currently eligible Application. Session reads must
-- additionally identify its unfinished recruitment session; practice cannot match this join.
-- name: GetCompanyInterviewer :one
SELECT av.name, av.model_path, v.name AS voice_name, a.interview_deadline
FROM public.applications a
JOIN public.job_postings jp ON jp.id = a.job_posting_id
JOIN public.avatars av ON av.id = jp.required_avatar_id
JOIN public.inventories inv ON inv.id = av.inventory_id AND inv.user_id = jp.recruiter_id
JOIN public.voices v ON v.id = jp.required_voice_id
WHERE a.candidate_id = @candidate_id
  AND a.status = 'interview_eligible'
  AND a.interview_deadline > statement_timestamp()
  AND jp.status IN ('open', 'intake_closed') AND jp.interview_slot IS NOT NULL
  AND ((a.id = @application_id AND @session_id::uuid = '00000000-0000-0000-0000-000000000000')
       OR EXISTS (SELECT 1 FROM public.interviews i
                  WHERE i.id = @session_id AND i.application_id = a.id
                    AND i.type = 'recruitment' AND i.completed_at IS NULL));
