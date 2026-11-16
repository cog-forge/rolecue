package service

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/cog-forge/rolecue/api/internal/features/evaluation/domain"
	"github.com/cog-forge/rolecue/api/internal/pkg/llmerr"
)

// flakyGrader wraps a grader and fails the first failures[position] calls of a question with err.
type flakyGrader struct {
	inner    *fakeGrader
	err      error
	mu       sync.Mutex
	failures map[int32]int
	calls    map[int32]int
}

func newFlakyGrader(inner *fakeGrader, err error, failures map[int32]int) *flakyGrader {
	return &flakyGrader{inner: inner, err: err, failures: failures, calls: map[int32]int{}}
}

func (g *flakyGrader) GradeQuestion(ctx context.Context, in domain.QuestionGradeInput) (domain.QuestionGrade, error) {
	g.mu.Lock()
	g.calls[in.QuestionPosition]++
	fail := g.calls[in.QuestionPosition] <= g.failures[in.QuestionPosition]
	g.mu.Unlock()
	if fail {
		return domain.QuestionGrade{}, g.err
	}
	return g.inner.GradeQuestion(ctx, in)
}

func (g *flakyGrader) callsOf(position int32) int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.calls[position]
}

func retryingScoring(t *testing.T, repo *fakeInterviewRepository, grader Grader) *Scoring {
	t.Helper()
	scoring := newScoringForTest(t, repo, grader)
	scoring.retry.Sleep = func(context.Context, time.Duration) bool { return true }
	scoring.retry.MinRemaining = 0
	return scoring
}

// The score of a session that needed a repeated call is the score of a session that did not.
func TestTransientFailureIsRetriedPerQuestionWithTheSameScores(t *testing.T) {
	for name, failure := range map[string]error{
		"429": llmerr.NewFailure(llmerr.ReasonRateLimited), "504": llmerr.NewFailure(llmerr.ReasonHTTP5xx),
		"timeout": llmerr.NewFailure(llmerr.ReasonTimeout), "network": llmerr.NewFailure(llmerr.ReasonNetwork),
	} {
		t.Run(name, func(t *testing.T) {
			repo := workedExampleRepository()
			grader := newFlakyGrader(pointsGrader(workedExamplePoints()), failure, map[int32]int{1: 2})
			if err := retryingScoring(t, repo, grader).Evaluate(context.Background(), scoringInterviewID); err != nil {
				t.Fatalf("Evaluate() error = %v", err)
			}
			if repo.marked || repo.saved == nil || repo.saved.Overall == nil || !closeEnough(*repo.saved.Overall, 62) {
				t.Fatalf("marked=%v saved=%+v, want the worked example's overall of 62 and no failure", repo.marked, repo.saved)
			}
			if grader.callsOf(1) != 3 || grader.callsOf(2) != 1 {
				t.Fatalf("calls q1=%d q2=%d, want only the failing question repeated", grader.callsOf(1), grader.callsOf(2))
			}
		})
	}
}

// A question whose provider keeps failing fails the session as before (no partial credit), but the other
// question's grade is kept: the next claim asks only for the missing one.
func TestExhaustedRetriesFailTheSessionAndKeepTheGradedQuestions(t *testing.T) {
	repo := workedExampleRepository()
	grader := newFlakyGrader(pointsGrader(workedExamplePoints()), llmerr.NewFailure(llmerr.ReasonRateLimited), map[int32]int{1: 3})
	scoring := retryingScoring(t, repo, grader)
	if err := scoring.Evaluate(context.Background(), scoringInterviewID); err == nil || !repo.marked || repo.saved != nil {
		t.Fatalf("error=%v marked=%v saved=%+v, want a failed session with nothing saved", err, repo.marked, repo.saved)
	}
	if grader.callsOf(1) != 1+gradeRetries || grader.callsOf(2) != 1 {
		t.Fatalf("calls q1=%d q2=%d, want %d repeats of q1 and q2 graded once", grader.callsOf(1), grader.callsOf(2), gradeRetries)
	}
	repo.marked = false
	if err := scoring.Evaluate(context.Background(), scoringInterviewID); err != nil {
		t.Fatalf("second claim: %v", err)
	}
	if grader.callsOf(2) != 1 || grader.callsOf(1) != 1+gradeRetries+1 {
		t.Fatalf("calls q1=%d q2=%d, want q2 not graded again by the next claim", grader.callsOf(1), grader.callsOf(2))
	}
	if repo.saved == nil || repo.saved.Overall == nil || !closeEnough(*repo.saved.Overall, 62) {
		t.Fatalf("saved = %+v, want the same overall of 62", repo.saved)
	}
	// Once saved nothing is kept: a later evaluation of the session would grade again.
	if len(scoring.graded.entries) != 0 {
		t.Fatalf("%d grades kept after the evaluation was saved", len(scoring.graded.entries))
	}
}

func TestAGradeIsNotReusedForADifferentAnswer(t *testing.T) {
	repo := workedExampleRepository()
	grader := newFlakyGrader(pointsGrader(workedExamplePoints()), llmerr.NewFailure(llmerr.ReasonHTTP5xx), map[int32]int{1: 3})
	scoring := retryingScoring(t, repo, grader)
	if err := scoring.Evaluate(context.Background(), scoringInterviewID); err == nil {
		t.Fatal("expected the first claim to fail")
	}
	changed := "A different answer to question two."
	for i := range repo.turns {
		if repo.turns[i].QuestionNumber == 2 {
			repo.turns[i].Answer = &changed
		}
	}
	if err := scoring.Evaluate(context.Background(), scoringInterviewID); err != nil {
		t.Fatalf("second claim: %v", err)
	}
	if grader.callsOf(2) != 2 {
		t.Fatalf("q2 calls = %d, want it graded again because its answer changed", grader.callsOf(2))
	}
}

func TestOnlyTransientFailuresAreRetried(t *testing.T) {
	for name, failure := range map[string]error{
		"auth rejected": llmerr.NewFailure("auth_rejected"), "invalid output": llmerr.NewFailure(llmerr.ReasonInvalidOutput),
		"unclassified": errors.New("provider down"),
	} {
		t.Run(name, func(t *testing.T) {
			repo := workedExampleRepository()
			grader := newFlakyGrader(pointsGrader(workedExamplePoints()), failure, map[int32]int{1: 1})
			if err := retryingScoring(t, repo, grader).Evaluate(context.Background(), scoringInterviewID); err == nil || !repo.marked {
				t.Fatalf("error=%v marked=%v, want a failed session", err, repo.marked)
			}
			if grader.callsOf(1) != 1 {
				t.Fatalf("q1 calls = %d, want no repeat of a failure that is not transient", grader.callsOf(1))
			}
		})
	}
}

func TestGradeCacheIsBoundedAndExpires(t *testing.T) {
	cache := newGradeCache()
	now := time.Now()
	cache.now = func() time.Time { return now }
	in := domain.QuestionGradeInput{QuestionPosition: 1, ExpectedAnswer: "x"}
	cache.put(scoringInterviewID, in, domain.QuestionGrade{QuestionPosition: 1})
	if _, ok := cache.get(scoringInterviewID, in); !ok {
		t.Fatal("a fresh grade must be reused")
	}
	now = now.Add(gradeCacheTTL + time.Second)
	if _, ok := cache.get(scoringInterviewID, in); ok {
		t.Fatal("an expired grade must not be reused")
	}
	for i := int32(0); i < 2*gradeCacheMax; i++ {
		cache.put(scoringInterviewID, domain.QuestionGradeInput{QuestionPosition: i, ExpectedAnswer: "x"}, domain.QuestionGrade{})
	}
	if len(cache.entries) > gradeCacheMax {
		t.Fatalf("%d entries kept, want at most %d", len(cache.entries), gradeCacheMax)
	}
}
