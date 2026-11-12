//go:build integration

package tests

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	moderationdomain "github.com/cog-forge/rolecue/api/internal/features/admin/moderation/domain"
	evaluationdomain "github.com/cog-forge/rolecue/api/internal/features/evaluation/domain"
	evaluationrepository "github.com/cog-forge/rolecue/api/internal/features/evaluation/repository"
	"github.com/cog-forge/rolecue/api/internal/features/questionbank/domain"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

// KAN-377: metric coverage of a posting's bank, on the real repositories. The bank screen, the submission and
// the Admin approval apply one rule (evaluation/domain.AnalyzeCoverage); the approval runs it under the
// posting lock, so no bank, weight or rubric change can slip in between the check and the decision.

// catalogMetrics returns the metric catalog ordered by id.
func (s *seed) catalogMetrics() []uuid.UUID {
	rows, err := s.pool.Query(s.ctx, `SELECT id FROM public.metrics ORDER BY id`)
	if err != nil {
		s.t.Fatal(err)
	}
	defer rows.Close()
	var out []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			s.t.Fatal(err)
		}
		out = append(out, id)
	}
	if len(out) < 4 {
		s.t.Fatalf("the metric catalog has %d rows, the coverage tests need at least 4", len(out))
	}
	return out
}

// rubricOn seeds a rubric owned by the recruiter with one criterion for each given metric.
func (s *seed) rubricOn(name string, owner uuid.UUID, metrics ...uuid.UUID) uuid.UUID {
	id := uuid.New()
	s.exec(`INSERT INTO public.rubrics (id, name, description, question_type, scope, owner_id)
		VALUES ($1, $2, 'coverage rubric', 'concept_explanation', 'recruiter', $3)`, id, name, owner)
	s.later(`DELETE FROM public.rubrics WHERE id = $1`, id)
	// Cleanup runs in reverse: free the questions that still use the rubric before it is deleted, because a
	// rubric seeded after the posting would otherwise be deleted while a question still points at it.
	s.later(`UPDATE public.core_questions SET rubric_id = NULL WHERE rubric_id = $1`, id)
	for i, metric := range metrics {
		s.exec(`INSERT INTO public.rubric_criteria (rubric_id, metric_id, name, max_points, scoring_guide, position)
			VALUES ($1, $2, 'c', 3, '0 = no; 3 = yes', $3)`, id, metric, i+1)
	}
	return id
}

// weigh saves weights for a posting (one row per metric).
func (s *seed) weigh(postingID uuid.UUID, weights map[uuid.UUID]float64) {
	for metric, percentage := range weights {
		s.exec(`INSERT INTO public.metrics_percentage (job_posting_id, metric_id, percentage) VALUES ($1, $2, $3)`, postingID, metric, percentage)
	}
	s.later(`DELETE FROM public.metrics_percentage WHERE job_posting_id = $1`, postingID)
}

func (f gradingFixture) coverage(t *testing.T, ctx context.Context) evaluationdomain.CoverageReport {
	t.Helper()
	report, err := f.bank.Coverage(ctx, f.posting)
	if err != nil {
		t.Fatal(err)
	}
	return report
}

// useRubric points a question at a rubric through the Recruiter's edit path, with a valid expected answer.
func (f gradingFixture) useRubric(t *testing.T, ctx context.Context, question domain.BankQuestion, rubric uuid.UUID) domain.BankQuestion {
	t.Helper()
	updated, err := f.bank.WriteQuestion(ctx, f.recruiter, f.posting, &question.ID, domain.QuestionPatch{
		SetRubric: true, RubricID: &rubric, SetExpectedAnswer: true, ExpectedAnswer: answer("Key points of a strong answer."),
		PreviousRevision: question.Revision})
	if err != nil {
		t.Fatal(err)
	}
	return updated
}

func TestBankCoverageNamesTheMissingMetricsAndClearsAfterTheFix(t *testing.T) {
	ctx, pool := openPool(t, 90*time.Second)
	s := newSeed(t, ctx, pool)
	defer s.cleanup()
	metrics := s.catalogMetrics()
	f := newGradingFixture(t, ctx, s)
	partial := s.rubricOn("qb partial "+s.tag, f.recruiter, metrics[0], metrics[1])
	saved := f.fill(t, ctx,
		domain.GeneratedQuestion{Content: "One?", RubricID: &partial, ExpectedAnswer: answer("a")},
		domain.GeneratedQuestion{Content: "Two?", RubricID: &partial, ExpectedAnswer: answer("a")})

	report := f.coverage(t, ctx)
	if report.Passes || !report.Assessed || report.Covered != 2 || report.Total != len(metrics) || report.QuestionLimit != evaluationdomain.DefaultQuestionLimit {
		t.Fatalf("coverage = %+v, want a failing 2/%d under the default limit", report, len(metrics))
	}
	if len(report.MissingMetrics) != len(metrics)-2 || report.ExceedsLimit {
		t.Fatalf("missing = %+v, want every uncovered metric named", report.MissingMetrics)
	}
	for _, missing := range report.MissingMetrics {
		if missing.Name == "" || missing.SuggestedRubrics == nil {
			t.Fatalf("missing metric %+v has no name or a nil suggestion list", missing)
		}
	}

	// A question whose rubric covers the rest fixes it.
	f.useRubric(t, ctx, saved[0], f.rubric)
	if report := f.coverage(t, ctx); !report.Passes || report.Covered != report.Total || len(report.MissingMetrics) != 0 {
		t.Fatalf("coverage after the fix = %+v, want a pass", report)
	}
}

func TestSubmitAndApprovalAreBlockedWithoutCoverageAndOpenAfterTheFix(t *testing.T) {
	ctx, pool := openPool(t, 90*time.Second)
	s := newSeed(t, ctx, pool)
	defer s.cleanup()
	metrics := s.catalogMetrics()
	f := newGradingFixture(t, ctx, s)
	partial := s.rubricOn("qb partial "+s.tag, f.recruiter, metrics[0], metrics[1])
	saved := f.fill(t, ctx, domain.GeneratedQuestion{Content: "One?", RubricID: &partial, ExpectedAnswer: answer("a")})

	err := f.jobs.Submit(ctx, f.recruiter, f.posting)
	if codeOf(err) != apperror.CodeMetricCoverageIncomplete {
		t.Fatalf("submit without coverage = %v, want %s", err, apperror.CodeMetricCoverageIncomplete)
	}
	if status, submitted := f.state(t, ctx); status != "pending" || submitted {
		t.Fatalf("a blocked submit left status=%s submitted=%v", status, submitted)
	}

	// Force a submitted state the way an old posting could have it, to prove the approval gate on its own.
	s.exec(`UPDATE public.job_postings SET review_submitted = true WHERE id = $1`, f.posting)
	// The Admin's dossier names what is missing before the Admin decides.
	dossier, err := f.moderate.Get(ctx, f.posting)
	if err != nil || dossier.Coverage == nil || dossier.Coverage.Passes || len(dossier.Coverage.MissingMetrics) != len(metrics)-2 {
		t.Fatalf("dossier coverage = %+v, err %v; want the failing report with the missing metrics", dossier.Coverage, err)
	}
	if err := f.moderate.Decide(ctx, f.posting, moderationdomain.PostingStatusOpen, nil); !errors.Is(err, moderationdomain.ErrMetricCoverageIncomplete) {
		t.Fatalf("approval without coverage = %v, want ErrMetricCoverageIncomplete", err)
	}
	if status, _ := f.state(t, ctx); status != "pending" {
		t.Fatalf("a blocked approval left status=%s", status)
	}
	// Rejection never needs coverage.
	reason := "not yet"
	if err := f.moderate.Decide(ctx, f.posting, moderationdomain.PostingStatusRejected, &reason); err != nil {
		t.Fatalf("rejection of an uncovered bank = %v, want it allowed", err)
	}

	// The Recruiter fixes the bank (the edit also reopens the posting for resubmission) and both gates open.
	f.useRubric(t, ctx, saved[0], f.rubric)
	if err := f.jobs.Resubmit(ctx, f.recruiter, f.posting); err != nil {
		t.Fatalf("resubmit after the fix = %v", err)
	}
	if err := f.moderate.Decide(ctx, f.posting, moderationdomain.PostingStatusOpen, nil); err != nil {
		t.Fatalf("approval after the fix = %v", err)
	}
	if status, _ := f.state(t, ctx); status != "open" {
		t.Fatalf("status = %s, want open", status)
	}
}

func TestWeightsDefineWhichMetricsNeedCoverage(t *testing.T) {
	ctx, pool := openPool(t, 90*time.Second)
	s := newSeed(t, ctx, pool)
	defer s.cleanup()
	metrics := s.catalogMetrics()
	f := newGradingFixture(t, ctx, s)
	partial := s.rubricOn("qb partial "+s.tag, f.recruiter, metrics[0], metrics[1])
	f.fill(t, ctx, domain.GeneratedQuestion{Content: "One?", RubricID: &partial, ExpectedAnswer: answer("a")})
	weights := evaluationrepository.NewRepository(pool)

	// No saved weights: the whole catalog is active, so the partial bank fails.
	if f.coverage(t, ctx).Passes {
		t.Fatal("a bank that covers two metrics passed the whole catalog")
	}
	// Weights on the two covered metrics: those are the only active ones, and the bank passes. A zero weight
	// is not active, so a metric nobody covers does not count against the bank.
	if _, _, err := weights.ReplaceWeights(ctx, f.recruiter, f.posting, []evaluationdomain.Weight{
		{MetricID: metrics[0], Percentage: 50}, {MetricID: metrics[1], Percentage: 50}, {MetricID: metrics[2], Percentage: 0}}); err != nil {
		t.Fatal(err)
	}
	if report := f.coverage(t, ctx); !report.Passes || report.Total != 2 {
		t.Fatalf("coverage under two weighted metrics = %+v, want a pass of 2/2", report)
	}
	if err := f.jobs.Submit(ctx, f.recruiter, f.posting); err != nil {
		t.Fatalf("submit under weights the bank covers = %v", err)
	}
	// A weight change the bank still covers keeps the submission: not every change is a change of what the Admin
	// reviewed.
	if _, withdrawn, err := weights.ReplaceWeights(ctx, f.recruiter, f.posting, []evaluationdomain.Weight{
		{MetricID: metrics[0], Percentage: 100}}); err != nil || withdrawn {
		t.Fatalf("weights the bank still covers: withdrawn=%v err=%v", withdrawn, err)
	}
	if _, submitted := f.state(t, ctx); !submitted {
		t.Fatal("a weight change that keeps the coverage withdrew the submission")
	}
	// A weight on a metric nobody covers re-runs the check, fails it, and takes the posting out of the queue.
	if _, withdrawn, err := weights.ReplaceWeights(ctx, f.recruiter, f.posting, []evaluationdomain.Weight{
		{MetricID: metrics[0], Percentage: 50}, {MetricID: metrics[2], Percentage: 50}}); err != nil || !withdrawn {
		t.Fatalf("weights that uncover the bank: withdrawn=%v err=%v", withdrawn, err)
	}
	if _, submitted := f.state(t, ctx); submitted {
		t.Fatal("a weight change that uncovered the bank left the posting submitted")
	}
	report := f.coverage(t, ctx)
	if report.Passes || len(report.MissingMetrics) != 1 || report.MissingMetrics[0].ID != metrics[2] {
		t.Fatalf("coverage after the weight change = %+v, want only the third metric missing", report)
	}
	// The approval gate itself, with a submission forced back by SQL (the withdrawal is what normally prevents it).
	s.exec(`UPDATE public.job_postings SET review_submitted = true WHERE id = $1`, f.posting)
	if err := f.moderate.Decide(ctx, f.posting, moderationdomain.PostingStatusOpen, nil); !errors.Is(err, moderationdomain.ErrMetricCoverageIncomplete) {
		t.Fatalf("approval after the weight change = %v, want ErrMetricCoverageIncomplete", err)
	}
	// Resetting to the defaults makes the whole catalog active again; the forced submission above is withdrawn.
	if withdrawn, err := weights.ClearWeights(ctx, f.recruiter, f.posting); err != nil || !withdrawn {
		t.Fatalf("reset to the defaults: withdrawn=%v err=%v, want the submission withdrawn", withdrawn, err)
	}
	if f.coverage(t, ctx).Passes {
		t.Fatal("the default weights must put the whole catalog back in play")
	}
}

func TestQuestionLimitDecidesWhetherASmallOverlapCovers(t *testing.T) {
	ctx, pool := openPool(t, 90*time.Second)
	s := newSeed(t, ctx, pool)
	defer s.cleanup()
	metrics := s.catalogMetrics()
	f := newGradingFixture(t, ctx, s)
	one := s.rubricOn("qb one "+s.tag, f.recruiter, metrics[0], metrics[1])
	two := s.rubricOn("qb two "+s.tag, f.recruiter, metrics[2], metrics[3])
	f.fill(t, ctx, domain.GeneratedQuestion{Content: "One?", RubricID: &one, ExpectedAnswer: answer("a")},
		domain.GeneratedQuestion{Content: "Two?", RubricID: &two, ExpectedAnswer: answer("a")})
	s.weigh(f.posting, map[uuid.UUID]float64{metrics[0]: 25, metrics[1]: 25, metrics[2]: 25, metrics[3]: 25})

	atLimit := func(limit int) evaluationdomain.CoverageReport {
		report, err := evaluationrepository.PostingCoverage(ctx, pool, f.posting, limit)
		if err != nil {
			t.Fatal(err)
		}
		return report
	}
	if report := atLimit(2); !report.Passes {
		t.Fatalf("limit 2 = %+v, want the two questions to cover four metrics", report)
	}
	report := atLimit(1)
	if report.Passes || !report.ExceedsLimit || report.MinQuestions != 2 || len(report.MissingMetrics) != 0 || report.Covered != 2 {
		t.Fatalf("limit 1 = %+v, want a fail that needs two questions with nothing missing from the bank", report)
	}
}

// The approval and any change that could uncover the bank run one after the other under the posting lock:
// either the approval wins (and the change is refused as locked) or the change wins (and the approval is
// refused). A posting is never left approved with a bank that cannot cover.
func TestApprovalRacingAChangeNeverLeavesAnUncoveredApprovedPosting(t *testing.T) {
	ctx, pool := openPool(t, 4*time.Minute)
	s := newSeed(t, ctx, pool)
	defer s.cleanup()
	metrics := s.catalogMetrics()
	weights := evaluationrepository.NewRepository(pool)

	type change struct {
		name string
		// setup returns a runner that uncovers the bank (a bank, weight or rubric change).
		setup func(f gradingFixture, saved []domain.BankQuestion) func() error
	}
	changes := []change{
		{"rubric change on a question", func(f gradingFixture, saved []domain.BankQuestion) func() error {
			partial := s.rubricOn("qb partial "+uuid.NewString()[:6], f.recruiter, metrics[0])
			return func() error {
				_, err := f.bank.WriteQuestion(ctx, f.recruiter, f.posting, &saved[0].ID, domain.QuestionPatch{
					SetRubric: true, RubricID: &partial, SetExpectedAnswer: true, ExpectedAnswer: answer("a"), PreviousRevision: saved[0].Revision})
				return err
			}
		}},
		{"bulk rubric assignment", func(f gradingFixture, saved []domain.BankQuestion) func() error {
			partial := s.rubricOn("qb partial "+uuid.NewString()[:6], f.recruiter, metrics[0])
			return func() error {
				_, err := f.bank.ApplyRubric(ctx, f.recruiter, f.posting, domain.BulkRubricRequest{
					QuestionIDs: []uuid.UUID{saved[0].ID}, RubricID: partial,
					PreviousRevisions: map[uuid.UUID]string{saved[0].ID: saved[0].Revision}})
				return err
			}
		}},
		{"delete of the covering question", func(f gradingFixture, saved []domain.BankQuestion) func() error {
			return func() error { return f.bank.Delete(ctx, f.recruiter, f.posting, saved[0].ID) }
		}},
		{"weight on an uncovered metric", func(f gradingFixture, saved []domain.BankQuestion) func() error {
			return func() error {
				_, _, err := weights.ReplaceWeights(ctx, f.recruiter, f.posting, []evaluationdomain.Weight{
					{MetricID: metrics[0], Percentage: 50}, {MetricID: metrics[2], Percentage: 50}})
				return err
			}
		}},
	}
	for _, c := range changes {
		t.Run(c.name, func(t *testing.T) {
			approvedFirst, changedFirst := 0, 0
			for i := 0; i < 15; i++ {
				f := newGradingFixture(t, ctx, s)
				// The one question covers exactly the two weighted metrics, so each change below uncovers the bank.
				onlyFirst := s.rubricOn("qb first "+uuid.NewString()[:6], f.recruiter, metrics[0], metrics[1])
				s.weigh(f.posting, map[uuid.UUID]float64{metrics[0]: 50, metrics[1]: 50})
				saved := f.fill(t, ctx, domain.GeneratedQuestion{Content: "Cover?", RubricID: &onlyFirst, ExpectedAnswer: answer("a")})
				if err := f.jobs.Submit(ctx, f.recruiter, f.posting); err != nil {
					t.Fatal(err)
				}
				run := c.setup(f, saved)

				var approveErr, changeErr error
				start := make(chan struct{})
				var wg sync.WaitGroup
				wg.Add(2)
				go func() {
					defer wg.Done()
					<-start
					approveErr = f.moderate.Decide(ctx, f.posting, moderationdomain.PostingStatusOpen, nil)
				}()
				go func() {
					defer wg.Done()
					<-start
					changeErr = run()
				}()
				close(start)
				wg.Wait()

				status, _ := f.state(t, ctx)
				if report := f.coverage(t, ctx); status == "open" && !report.Passes {
					t.Fatalf("iteration %d: the posting is open but its bank no longer covers: %+v (approve=%v change=%v)", i, report, approveErr, changeErr)
				}
				switch {
				case approveErr == nil:
					approvedFirst++
					if status != "open" || changeErr == nil {
						t.Fatalf("iteration %d: approval won but status=%s change=%v", i, status, changeErr)
					}
				case changeErr == nil:
					changedFirst++
					if status == "open" {
						t.Fatalf("iteration %d: the change committed and the approval still opened the posting", i)
					}
					// Whatever the change, the posting is out of the queue (a bank edit and an uncovering weight
					// change both withdraw it), so the approval found nothing submitted to decide.
					if _, submitted := f.state(t, ctx); submitted || !errors.Is(approveErr, moderationdomain.ErrNotPending) {
						t.Fatalf("iteration %d: change first but submitted=%v approve=%v, want withdrawn and ErrNotPending", i, submitted, approveErr)
					}
				default:
					t.Fatalf("iteration %d: approve=%v change=%v", i, approveErr, changeErr)
				}
			}
			t.Logf("approval first %d times, change first %d times", approvedFirst, changedFirst)
		})
	}
}

// KAN-377 decision (user, 12/11): a weight change on a submitted pending posting takes it out of the Admin queue
// only when the bank no longer covers every active metric afterwards. Same for PUT and DELETE, and nothing else
// is touched.
func TestWeightChangeWithdrawsTheSubmissionOnlyWhenItUncoversTheBank(t *testing.T) {
	ctx, pool := openPool(t, 2*time.Minute)
	s := newSeed(t, ctx, pool)
	defer s.cleanup()
	metrics := s.catalogMetrics()
	weights := evaluationrepository.NewRepository(pool)

	type step struct {
		name string
		// run changes the weights and returns whether the change reported a withdrawal.
		run  func(f gradingFixture) (bool, error)
		want bool
	}
	replace := func(w ...evaluationdomain.Weight) func(f gradingFixture) (bool, error) {
		return func(f gradingFixture) (bool, error) {
			_, withdrawn, err := weights.ReplaceWeights(ctx, f.recruiter, f.posting, w)
			return withdrawn, err
		}
	}
	reset := func(f gradingFixture) (bool, error) { return weights.ClearWeights(ctx, f.recruiter, f.posting) }
	cases := []struct {
		name  string
		bank  string // "partial" covers metrics 0 and 1; "full" covers the whole catalog
		saved []evaluationdomain.Weight
		step  step
	}{
		{"PUT that uncovers", "partial", []evaluationdomain.Weight{{MetricID: metrics[0], Percentage: 100}},
			step{"PUT metric 3", replace(evaluationdomain.Weight{MetricID: metrics[2], Percentage: 100}), true}},
		{"PUT the bank still covers", "partial", []evaluationdomain.Weight{{MetricID: metrics[0], Percentage: 100}},
			step{"PUT metric 2", replace(evaluationdomain.Weight{MetricID: metrics[1], Percentage: 100}), false}},
		{"DELETE that uncovers (whole catalog active again)", "partial", []evaluationdomain.Weight{{MetricID: metrics[0], Percentage: 100}},
			step{"DELETE", reset, true}},
		{"DELETE the bank still covers", "full", []evaluationdomain.Weight{{MetricID: metrics[0], Percentage: 100}},
			step{"DELETE", reset, false}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := newGradingFixture(t, ctx, s)
			rubric := f.rubric
			if c.bank == "partial" {
				rubric = s.rubricOn("qb partial "+uuid.NewString()[:6], f.recruiter, metrics[0], metrics[1])
			}
			f.fill(t, ctx, domain.GeneratedQuestion{Content: "One?", RubricID: &rubric, ExpectedAnswer: answer("a")})
			s.weigh(f.posting, map[uuid.UUID]float64{c.saved[0].MetricID: c.saved[0].Percentage})
			if err := f.jobs.Submit(ctx, f.recruiter, f.posting); err != nil {
				t.Fatalf("submit: %v", err)
			}
			withdrawn, err := c.step.run(f)
			if err != nil || withdrawn != c.step.want {
				t.Fatalf("%s: withdrawn=%v err=%v, want withdrawn=%v", c.step.name, withdrawn, err, c.step.want)
			}
			status, submitted := f.state(t, ctx)
			if status != "pending" || submitted == c.step.want {
				t.Fatalf("%s: status=%s submitted=%v, want pending and submitted=%v", c.step.name, status, submitted, !c.step.want)
			}
		})
	}

	t.Run("a posting that was never submitted is not touched", func(t *testing.T) {
		f := newGradingFixture(t, ctx, s)
		partial := s.rubricOn("qb partial "+uuid.NewString()[:6], f.recruiter, metrics[0])
		f.fill(t, ctx, domain.GeneratedQuestion{Content: "One?", RubricID: &partial, ExpectedAnswer: answer("a")})
		s.later(`DELETE FROM public.metrics_percentage WHERE job_posting_id = $1`, f.posting)
		_, withdrawn, err := weights.ReplaceWeights(ctx, f.recruiter, f.posting, []evaluationdomain.Weight{{MetricID: metrics[2], Percentage: 100}})
		if err != nil || withdrawn {
			t.Fatalf("withdrawn=%v err=%v, want nothing to withdraw", withdrawn, err)
		}
	})
}
