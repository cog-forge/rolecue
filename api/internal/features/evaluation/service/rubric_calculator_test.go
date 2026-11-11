package service

import (
	"errors"
	"math"
	"math/rand"
	"testing"

	"github.com/cog-forge/rolecue/api/internal/features/evaluation/domain"
	"github.com/google/uuid"
)

func uid(n byte) uuid.UUID { return uuid.UUID{15: n} }

var (
	rbAccuracy = uid(1)
	rbProblem  = uid(2)
	rbDepth    = uid(3)
)

func crit(n byte, metric uuid.UUID, max, points int32) domain.CriterionPoints {
	return domain.CriterionPoints{CriterionID: uid(100 + n), MetricID: metric, MaxPoints: max, Points: points}
}

func question(position int32, criteria ...domain.CriterionPoints) domain.QuestionPoints {
	return domain.QuestionPoints{Position: position, Criteria: criteria}
}

func weights(pairs ...any) []domain.ScoredMetric {
	out := []domain.ScoredMetric{}
	for i := 0; i < len(pairs); i += 2 {
		out = append(out, domain.ScoredMetric{MetricID: pairs[i].(uuid.UUID), Weight: pairs[i+1].(float64)})
	}
	return out
}

// canonical is the worked example of the spec (section 2): rubric R01 with C01 and C02 on Technical Accuracy
// (2 and 3 points) and C03 on Problem-Solving (5 points), answered in two questions.
func canonical() domain.RubricScoreInput {
	return domain.RubricScoreInput{
		Questions: []domain.QuestionPoints{
			question(1, crit(1, rbAccuracy, 2, 2), crit(2, rbAccuracy, 3, 2), crit(3, rbProblem, 5, 3)),
			question(2, crit(1, rbAccuracy, 2, 1), crit(2, rbAccuracy, 3, 2), crit(3, rbProblem, 5, 2)),
		},
		Weights:           weights(rbAccuracy, 60.0, rbProblem, 40.0),
		AssignedQuestions: 2,
	}
}

func mustCalculate(t *testing.T, in domain.RubricScoreInput, policy domain.ScoringPolicy) domain.RubricScore {
	t.Helper()
	score, err := CalculateRubricScore(in, policy)
	if err != nil {
		t.Fatalf("CalculateRubricScore() error = %v", err)
	}
	return score
}

func metricScore(t *testing.T, score domain.RubricScore, id uuid.UUID) domain.RubricMetricScore {
	t.Helper()
	for _, metric := range score.Metrics {
		if metric.MetricID == id {
			return metric
		}
	}
	t.Fatalf("no session metric %s in %+v", id, score.Metrics)
	return domain.RubricMetricScore{}
}

func closeTo(got, want float64) bool { return math.Abs(got-want) < 1e-9 }

func TestCanonicalExampleOverall62NotMeanOfQuestionTotals(t *testing.T) {
	score := mustCalculate(t, canonical(), domain.DefaultScoringPolicy())

	if len(score.Questions) != 2 || !closeTo(score.Questions[0].Total, 70) || !closeTo(score.Questions[1].Total, 50) {
		t.Fatalf("question totals = %+v, want 70 and 50", score.Questions)
	}
	if got := metricScore(t, score, rbAccuracy).Score; !closeTo(got, 70) {
		t.Errorf("Technical Accuracy session metric = %v, want 70", got)
	}
	if got := metricScore(t, score, rbProblem).Score; !closeTo(got, 50) {
		t.Errorf("Problem-Solving session metric = %v, want 50", got)
	}
	if score.Overall == nil || *score.Overall != 62 {
		t.Fatalf("overall = %v, want 62", score.Overall)
	}
	if score.Outcome != domain.RubricComplete {
		t.Errorf("outcome = %q, want complete", score.Outcome)
	}
	// The trap: the mean of the question totals is 60 and must never become the overall.
	if mean := (score.Questions[0].Total + score.Questions[1].Total) / 2; !closeTo(mean, 60) || *score.Overall == 60 {
		t.Fatalf("mean of question totals = %v, overall = %v; they must differ (60 vs 62)", mean, *score.Overall)
	}
	// Question level detail of the spec: Q01 Technical Accuracy = (2+2)/(2+3) = 80, Q02 = (1+2)/(2+3) = 60.
	wantQ01 := map[uuid.UUID]float64{rbAccuracy: 80, rbProblem: 60}
	for _, metric := range score.Questions[0].Metrics {
		if want := wantQ01[metric.MetricID]; !closeTo(metric.Score, want) {
			t.Errorf("Q01 metric %s = %v, want %v", metric.MetricID, metric.Score, want)
		}
	}
	wantQ02 := map[uuid.UUID]float64{rbAccuracy: 60, rbProblem: 40}
	for _, metric := range score.Questions[1].Metrics {
		if want := wantQ02[metric.MetricID]; !closeTo(metric.Score, want) {
			t.Errorf("Q02 metric %s = %v, want %v", metric.MetricID, metric.Score, want)
		}
	}
}

func TestCriteriaOfOneMetricAreNormalizedByTotalEarnedOverTotalMax(t *testing.T) {
	// 1/2 (50%) and 3/3 (100%) on one metric: pooled 4/5 = 80, not the mean of the ratios (75).
	in := domain.RubricScoreInput{
		Questions: []domain.QuestionPoints{question(1, crit(1, rbAccuracy, 2, 1), crit(2, rbAccuracy, 3, 3))},
		Weights:   weights(rbAccuracy, 1.0),
	}
	score := mustCalculate(t, in, domain.DefaultScoringPolicy())
	if got := score.Questions[0].Metrics[0].Score; !closeTo(got, 80) {
		t.Fatalf("question metric = %v, want 80", got)
	}
}

func TestEachQuestionIsNormalizedBeforeTheSessionMean(t *testing.T) {
	// Q1 earns 5/10 and Q2 earns 2/2 on the same metric: mean(50, 100) = 75. Pooling the points
	// across questions (7/12 = 58.33) would let the bigger rubric dominate.
	in := domain.RubricScoreInput{
		Questions: []domain.QuestionPoints{
			question(1, crit(1, rbAccuracy, 10, 5)),
			question(2, crit(2, rbAccuracy, 2, 2)),
		},
		Weights: weights(rbAccuracy, 1.0),
	}
	score := mustCalculate(t, in, domain.DefaultScoringPolicy())
	if got := metricScore(t, score, rbAccuracy); !closeTo(got.Score, 75) || got.Questions != 2 {
		t.Fatalf("session metric = %+v, want 75 over 2 questions", got)
	}
}

func TestQuestionWithoutAMetricDoesNotCountAsZeroForIt(t *testing.T) {
	// Only Q1 assesses Problem-Solving; its session score is Q1's alone, not halved by a silent Q2.
	in := domain.RubricScoreInput{
		Questions: []domain.QuestionPoints{
			question(1, crit(1, rbAccuracy, 4, 4), crit(2, rbProblem, 4, 2)),
			question(2, crit(1, rbAccuracy, 4, 2)),
		},
		Weights: weights(rbAccuracy, 50.0, rbProblem, 50.0),
	}
	score := mustCalculate(t, in, domain.DefaultScoringPolicy())
	if got := metricScore(t, score, rbProblem); !closeTo(got.Score, 50) || got.Questions != 1 {
		t.Fatalf("Problem-Solving = %+v, want 50 from one question", got)
	}
	if got := metricScore(t, score, rbAccuracy); !closeTo(got.Score, 75) {
		t.Fatalf("Technical Accuracy = %v, want mean(100, 50) = 75", got.Score)
	}
	if score.Overall == nil || *score.Overall != 63 { // (75 + 50) / 2 = 62.5 -> 63 half-up
		t.Fatalf("overall = %v, want 63", score.Overall)
	}
}

func TestFollowUpCannotGrowTheDenominator(t *testing.T) {
	// A follow-up's evidence updates the criteria of its core question. A second entry for the same core
	// position (what a follow-up counted as its own question would look like) is refused.
	in := canonical()
	in.Questions = append(in.Questions, question(2, crit(1, rbAccuracy, 2, 2), crit(3, rbProblem, 5, 5)))
	if _, err := CalculateRubricScore(in, domain.DefaultScoringPolicy()); !errors.Is(err, ErrInvalidRubricScore) {
		t.Fatalf("error = %v, want ErrInvalidRubricScore for a repeated question position", err)
	}
	// And the maximum of a question is only its own criteria: 10 here, whatever was asked afterwards.
	score := mustCalculate(t, canonical(), domain.DefaultScoringPolicy())
	if score.Questions[0].Max != 10 || score.Questions[0].Earned != 7 {
		t.Fatalf("question 1 earned/max = %d/%d, want 7/10", score.Questions[0].Earned, score.Questions[0].Max)
	}
}

func TestRoundingIsHalfUpAndExact(t *testing.T) {
	cases := []struct {
		name         string
		earned, max  int32
		wantQuestion float64
	}{
		{"one third", 1, 3, 33.33},
		{"two thirds", 2, 3, 66.67},
		// 2469 / 20000 x 100 = 12.345 exactly: half-up gives 12.35, and no binary float noise may flip it.
		{"exact tie at the third decimal", 2469, 20000, 12.35},
		{"exact tie at the third decimal, other side", 2471, 20000, 12.36},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := domain.RubricScoreInput{
				Questions: []domain.QuestionPoints{question(1, crit(1, rbAccuracy, tc.max, tc.earned))},
				Weights:   weights(rbAccuracy, 1.0),
			}
			score := mustCalculate(t, in, domain.DefaultScoringPolicy())
			if got := score.Questions[0].Total; !closeTo(got, tc.wantQuestion) {
				t.Errorf("question total = %v, want %v", got, tc.wantQuestion)
			}
			if got := metricScore(t, score, rbAccuracy).Score; !closeTo(got, tc.wantQuestion) {
				t.Errorf("session metric = %v, want %v", got, tc.wantQuestion)
			}
		})
	}
}

func TestOverallRoundsHalfUpFromTheRoundedSessionMetrics(t *testing.T) {
	// Metrics 70 and 55 with equal weights: 62.5 rounds up to 63.
	in := domain.RubricScoreInput{
		Questions: []domain.QuestionPoints{question(1, crit(1, rbAccuracy, 10, 7), crit(2, rbProblem, 20, 11))},
		Weights:   weights(rbAccuracy, 1.0, rbProblem, 1.0),
	}
	if got := mustCalculate(t, in, domain.DefaultScoringPolicy()).Overall; got == nil || *got != 63 {
		t.Fatalf("overall = %v, want 63", got)
	}

	// The overall is calculated from the stored (rounded) session metric, as settled on 11/11: 12499 / 20000
	// is 62.495 exactly, stored as 62.50, and 62.50 rounds up to 63. The unrounded 62.495 would give 62.
	in = domain.RubricScoreInput{
		Questions: []domain.QuestionPoints{question(1, crit(1, rbAccuracy, 20000, 12499))},
		Weights:   weights(rbAccuracy, 1.0),
	}
	score := mustCalculate(t, in, domain.DefaultScoringPolicy())
	if got := metricScore(t, score, rbAccuracy).Score; !closeTo(got, 62.5) {
		t.Fatalf("stored session metric = %v, want 62.5", got)
	}
	if score.Overall == nil || *score.Overall != 63 {
		t.Fatalf("overall = %v, want 63 (computed from the stored 62.50)", score.Overall)
	}
}

func TestMetricDecimalsPolicy(t *testing.T) {
	in := domain.RubricScoreInput{
		Questions: []domain.QuestionPoints{question(1, crit(1, rbAccuracy, 3, 1))},
		Weights:   weights(rbAccuracy, 1.0),
	}
	policy := domain.DefaultScoringPolicy()
	policy.MetricDecimals = 0
	score := mustCalculate(t, in, policy)
	if got := metricScore(t, score, rbAccuracy).Score; got != 33 {
		t.Fatalf("session metric with 0 decimals = %v, want 33", got)
	}
	policy.MetricDecimals = 4
	score = mustCalculate(t, in, policy)
	if got := metricScore(t, score, rbAccuracy).Score; !closeTo(got, 33.3333) {
		t.Fatalf("session metric with 4 decimals = %v, want 33.3333", got)
	}
	for _, decimals := range []int{-1, domain.MaxMetricDecimals + 1} {
		policy.MetricDecimals = decimals
		if _, err := CalculateRubricScore(in, policy); !errors.Is(err, ErrInvalidScoringPolicy) {
			t.Errorf("MetricDecimals = %d: error = %v, want ErrInvalidScoringPolicy", decimals, err)
		}
	}
}

func TestMissingMetricCoveragePolicy(t *testing.T) {
	// Depth carries weight but no scored question assesses it.
	in := domain.RubricScoreInput{
		Questions: []domain.QuestionPoints{question(1, crit(1, rbAccuracy, 10, 8), crit(2, rbProblem, 10, 4))},
		Weights:   weights(rbAccuracy, 50.0, rbProblem, 30.0, rbDepth, 20.0),
	}

	score := mustCalculate(t, in, domain.DefaultScoringPolicy())
	if score.Outcome != domain.RubricWithheld || score.Reason != domain.WithheldMissingCoverage || score.Overall != nil {
		t.Fatalf("default policy: outcome=%q reason=%q overall=%v, want withheld for missing coverage", score.Outcome, score.Reason, score.Overall)
	}
	if len(score.MissingMetrics) != 1 || score.MissingMetrics[0] != rbDepth {
		t.Fatalf("missing metrics = %v, want [%s]", score.MissingMetrics, rbDepth)
	}
	// Even withheld, the metrics that do have evidence are reported, and the gap is not filled with 0.
	if len(score.Metrics) != 2 {
		t.Fatalf("session metrics = %+v, want the two assessed ones", score.Metrics)
	}

	policy := domain.DefaultScoringPolicy()
	policy.MissingCoverage = domain.MissingCoverageRenormalize
	score = mustCalculate(t, in, policy)
	// (80 x 50 + 40 x 30) / 80 = 65
	if score.Overall == nil || *score.Overall != 65 || score.Outcome != domain.RubricPartial {
		t.Fatalf("renormalize policy: overall=%v outcome=%q, want 65 and partial", score.Overall, score.Outcome)
	}
	if len(score.MissingMetrics) != 1 {
		t.Fatalf("renormalized result must still list the missing metric, got %v", score.MissingMetrics)
	}
}

func TestZeroWeightMetricNeedsNoCoverageAndUnweightedMetricIsReportedOnly(t *testing.T) {
	in := domain.RubricScoreInput{
		Questions: []domain.QuestionPoints{question(1, crit(1, rbAccuracy, 10, 9), crit(2, rbDepth, 10, 1))},
		// Problem-Solving has weight 0 and no evidence; Depth is assessed but not weighted at all.
		Weights: weights(rbAccuracy, 100.0, rbProblem, 0.0),
	}
	score := mustCalculate(t, in, domain.DefaultScoringPolicy())
	if score.Overall == nil || *score.Overall != 90 || score.Outcome != domain.RubricComplete {
		t.Fatalf("overall = %v outcome = %q, want 90 complete", score.Overall, score.Outcome)
	}
	if got := metricScore(t, score, rbDepth); got.Weight != 0 || !closeTo(got.Score, 10) {
		t.Fatalf("unweighted metric = %+v, want listed with weight 0 and score 10", got)
	}
}

func TestPartialSessionPolicy(t *testing.T) {
	in := canonical()
	in.AssignedQuestions = 4 // two questions were assigned but have no evidence

	score := mustCalculate(t, in, domain.DefaultScoringPolicy())
	if score.Outcome != domain.RubricPartial || score.Overall == nil || *score.Overall != 62 {
		t.Fatalf("default: outcome=%q overall=%v, want partial with 62 (unscored questions are not zeros)", score.Outcome, score.Overall)
	}
	if score.ScoredQuestions != 2 || score.AssignedQuestions != 4 {
		t.Fatalf("scored/assigned = %d/%d, want 2/4", score.ScoredQuestions, score.AssignedQuestions)
	}

	policy := domain.DefaultScoringPolicy()
	policy.PartialSession = domain.PartialSessionWithhold
	score = mustCalculate(t, in, policy)
	if score.Outcome != domain.RubricWithheld || score.Reason != domain.WithheldPartialSession || score.Overall != nil {
		t.Fatalf("withhold policy: outcome=%q reason=%q overall=%v", score.Outcome, score.Reason, score.Overall)
	}

	in.AssignedQuestions = 0 // unknown: no partial check
	if got := mustCalculate(t, in, policy); got.Outcome != domain.RubricComplete {
		t.Fatalf("unknown assigned count: outcome = %q, want complete", got.Outcome)
	}
	in.AssignedQuestions = 1 // fewer assigned than scored cannot be right
	if _, err := CalculateRubricScore(in, domain.DefaultScoringPolicy()); !errors.Is(err, ErrInvalidRubricScore) {
		t.Fatalf("assigned < scored: error = %v, want ErrInvalidRubricScore", err)
	}
}

func TestInvalidInputIsRefused(t *testing.T) {
	good := func() domain.RubricScoreInput { return canonical() }
	cases := []struct {
		name   string
		mutate func(*domain.RubricScoreInput)
	}{
		{"no questions", func(in *domain.RubricScoreInput) { in.Questions = nil }},
		{"a question without criteria", func(in *domain.RubricScoreInput) { in.Questions[0].Criteria = nil }},
		{"negative points", func(in *domain.RubricScoreInput) { in.Questions[0].Criteria[0].Points = -1 }},
		{"points above the maximum", func(in *domain.RubricScoreInput) { in.Questions[0].Criteria[0].Points = 3 }},
		{"zero maximum", func(in *domain.RubricScoreInput) { in.Questions[0].Criteria[0].MaxPoints = 0 }},
		{"negative maximum", func(in *domain.RubricScoreInput) { in.Questions[0].Criteria[0].MaxPoints = -2 }},
		{"duplicate criterion in a question", func(in *domain.RubricScoreInput) {
			in.Questions[0].Criteria = append(in.Questions[0].Criteria, in.Questions[0].Criteria[0])
		}},
		{"duplicate question position", func(in *domain.RubricScoreInput) { in.Questions[1].Position = 1 }},
		{"nil criterion id", func(in *domain.RubricScoreInput) { in.Questions[0].Criteria[0].CriterionID = uuid.Nil }},
		{"nil metric id", func(in *domain.RubricScoreInput) { in.Questions[0].Criteria[0].MetricID = uuid.Nil }},
		{"same criterion with another maximum in another question", func(in *domain.RubricScoreInput) {
			in.Questions[1].Criteria[0].MaxPoints = 4
		}},
		{"same criterion on another metric in another question", func(in *domain.RubricScoreInput) {
			in.Questions[1].Criteria[0].MetricID = rbProblem
		}},
		{"negative assigned count", func(in *domain.RubricScoreInput) { in.AssignedQuestions = -1 }},
		{"no weights", func(in *domain.RubricScoreInput) { in.Weights = nil }},
		{"negative weight", func(in *domain.RubricScoreInput) { in.Weights[0].Weight = -1 }},
		{"NaN weight", func(in *domain.RubricScoreInput) { in.Weights[0].Weight = math.NaN() }},
		{"infinite weight", func(in *domain.RubricScoreInput) { in.Weights[0].Weight = math.Inf(1) }},
		{"weights adding up to zero", func(in *domain.RubricScoreInput) { in.Weights[0].Weight, in.Weights[1].Weight = 0, 0 }},
		{"a metric weighted twice", func(in *domain.RubricScoreInput) { in.Weights[1].MetricID = in.Weights[0].MetricID }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := good()
			tc.mutate(&in)
			if _, err := CalculateRubricScore(in, domain.DefaultScoringPolicy()); !errors.Is(err, ErrInvalidRubricScore) {
				t.Fatalf("error = %v, want ErrInvalidRubricScore", err)
			}
		})
	}
}

func TestCalculatorDoesNotModifyItsInput(t *testing.T) {
	in := canonical()
	_ = mustCalculate(t, in, domain.DefaultScoringPolicy())
	want := canonical()
	for i, q := range in.Questions {
		for j, c := range q.Criteria {
			if c != want.Questions[i].Criteria[j] {
				t.Fatalf("input criterion %d/%d changed: %+v", i, j, c)
			}
		}
	}
}

func TestResultDoesNotDependOnInputOrder(t *testing.T) {
	want := mustCalculate(t, canonical(), domain.DefaultScoringPolicy())
	rng := rand.New(rand.NewSource(7))
	for i := 0; i < 50; i++ {
		in := canonical()
		rng.Shuffle(len(in.Questions), func(a, b int) { in.Questions[a], in.Questions[b] = in.Questions[b], in.Questions[a] })
		for _, q := range in.Questions {
			rng.Shuffle(len(q.Criteria), func(a, b int) { q.Criteria[a], q.Criteria[b] = q.Criteria[b], q.Criteria[a] })
		}
		rng.Shuffle(len(in.Weights), func(a, b int) { in.Weights[a], in.Weights[b] = in.Weights[b], in.Weights[a] })
		got := mustCalculate(t, in, domain.DefaultScoringPolicy())
		if got.Overall == nil || *got.Overall != *want.Overall || len(got.Metrics) != len(want.Metrics) {
			t.Fatalf("shuffled input %d gave %+v, want %+v", i, got, want)
		}
		for k := range got.Metrics {
			if got.Metrics[k] != want.Metrics[k] {
				t.Fatalf("shuffled input %d metric %d = %+v, want %+v", i, k, got.Metrics[k], want.Metrics[k])
			}
		}
	}
}

// TestScoreProperties runs seeded random sessions and checks what must hold for any of them.
func TestScoreProperties(t *testing.T) {
	rng := rand.New(rand.NewSource(20261111))
	metrics := []uuid.UUID{rbAccuracy, rbProblem, rbDepth}
	for run := 0; run < 300; run++ {
		var questions []domain.QuestionPoints
		allMax, allZero := true, true
		rubricMax := map[byte]int32{}
		covered := map[uuid.UUID]bool{}
		for q := int32(1); q <= int32(1+rng.Intn(5)); q++ {
			var criteria []domain.CriterionPoints
			for c := byte(1); c <= byte(1+rng.Intn(4)); c++ {
				if _, ok := rubricMax[c]; !ok {
					rubricMax[c] = int32(1 + rng.Intn(6))
				}
				max := rubricMax[c]
				metric := metrics[int(c)%len(metrics)]
				points := int32(rng.Intn(int(max) + 1))
				allMax = allMax && points == max
				allZero = allZero && points == 0
				covered[metric] = true
				criteria = append(criteria, crit(c, metric, max, points))
			}
			questions = append(questions, domain.QuestionPoints{Position: q, Criteria: criteria})
		}
		var ws []domain.ScoredMetric
		for metric := range covered {
			ws = append(ws, domain.ScoredMetric{MetricID: metric, Weight: float64(1 + rng.Intn(100))})
		}
		in := domain.RubricScoreInput{Questions: questions, Weights: ws}
		score := mustCalculate(t, in, domain.DefaultScoringPolicy())
		if score.Overall == nil {
			t.Fatalf("run %d: no overall although every weighted metric is covered", run)
		}
		lo, hi := math.Inf(1), math.Inf(-1)
		for _, metric := range score.Metrics {
			if metric.Score < 0 || metric.Score > 100 {
				t.Fatalf("run %d: session metric %v out of 0..100", run, metric.Score)
			}
			lo, hi = math.Min(lo, metric.Score), math.Max(hi, metric.Score)
		}
		// A weighted mean lies between the smallest and largest metric (allow the final whole-number rounding).
		if float64(*score.Overall) < math.Floor(lo) || float64(*score.Overall) > math.Ceil(hi) {
			t.Fatalf("run %d: overall %d outside the metric range [%v, %v]", run, *score.Overall, lo, hi)
		}
		for _, q := range score.Questions {
			if q.Total < 0 || q.Total > 100 {
				t.Fatalf("run %d: question total %v out of 0..100", run, q.Total)
			}
		}
		if allMax && *score.Overall != 100 {
			t.Fatalf("run %d: every criterion at its maximum gave overall %d, want 100", run, *score.Overall)
		}
		if allZero && *score.Overall != 0 {
			t.Fatalf("run %d: every criterion at zero gave overall %d, want 0", run, *score.Overall)
		}
	}
}
