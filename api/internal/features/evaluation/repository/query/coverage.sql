-- Metric coverage of a question bank (KAN-377, KAN-378). These queries feed the one coverage rule of
-- evaluation/domain (AnalyzeCoverage) for every gate that asks "can this bank cover the active metrics?":
-- the Recruiter's bank screen, posting submission, Admin approval, posting weight changes and practice
-- generation. The interview start keeps its own copies in interview/repository/query/interview.sql (that
-- feature does not import this one); keep the three bank/active-metric queries equivalent to those.

-- The bank of a Job Description with the distinct metrics its rubric has criteria for. metric_ids is empty
-- for a question without a rubric and for a rubric without criteria. It joins the rubric as it is, whether
-- or not it is active: a question keeps the rubric it was authored with and grading uses it.
-- name: ListCoverageBank :many
SELECT cq.id,
       COALESCE(array_agg(DISTINCT rc.metric_id) FILTER (WHERE rc.metric_id IS NOT NULL), '{}')::uuid[] AS metric_ids
FROM public.core_questions cq
LEFT JOIN public.rubric_criteria rc ON rc.rubric_id = cq.rubric_id
WHERE cq.jd_id = @jd_id
GROUP BY cq.id
ORDER BY cq.id;

-- The metrics a posting is scored on, as evaluation reads them: with saved weights the active metrics are
-- the ones with a positive weight; with none the equal-split defaults apply, which is the whole catalog.
-- name: ListCoveragePostingMetrics :many
SELECT m.id
FROM public.metrics m
WHERE NOT EXISTS (SELECT 1 FROM public.metrics_percentage w WHERE w.job_posting_id = @job_posting_id)
   OR EXISTS (SELECT 1 FROM public.metrics_percentage w
              WHERE w.job_posting_id = @job_posting_id AND w.metric_id = m.id AND w.percentage > 0)
ORDER BY m.id;

-- The JD of a posting, for a caller that holds only the posting id (a weight change).
-- name: GetCoveragePostingJD :one
SELECT jd_id FROM public.job_postings WHERE id = @id;

-- Active system rubrics with the metric of each criterion: what the Recruiter can assign to cover a missing
-- metric. Names only; nothing here is a Candidate-facing value.
-- name: ListSystemRubricMetrics :many
SELECT r.id AS rubric_id, r.name AS rubric_name, rc.metric_id
FROM public.rubrics r
JOIN public.rubric_criteria rc ON rc.rubric_id = r.id
WHERE r.scope = 'system' AND r.is_active
GROUP BY r.id, r.name, rc.metric_id
ORDER BY r.name, r.id;
