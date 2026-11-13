package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/cog-forge/rolecue/api/internal/features/roadmap/domain"
	"github.com/cog-forge/rolecue/api/internal/features/roadmap/service"
)

type fakeSource struct {
	info  service.SessionInfo
	err   error
	loads int
}

func (f *fakeSource) LoadSession(context.Context, uuid.UUID) (service.SessionInfo, error) {
	f.loads++
	return f.info, f.err
}

type fakeStore struct {
	positions [][]int32
	owner     uuid.UUID
	err       error
	outcome   service.LinkOutcome
}

func (f *fakeStore) LinkCores(_ context.Context, candidate, _ uuid.UUID, positions []int32) (service.LinkOutcome, error) {
	f.owner = candidate
	f.positions = append(f.positions, append([]int32(nil), positions...))
	if f.err != nil {
		return service.LinkOutcome{}, f.err
	}
	return f.outcome, nil
}

func eligibleSession(cores ...domain.CoreFacts) service.SessionInfo {
	done := time.Date(2026, 11, 1, 9, 0, 0, 0, time.UTC)
	return service.SessionInfo{
		CandidateID: uid(9),
		Session:     domain.SessionFacts{Type: "practice", CompletedAt: &done, EvaluationStatus: "completed"},
		Cores:       cores,
	}
}

func sc(v float64) *float64 { return &v }

func TestLinkWritesOnlyCountedCoresInOnePass(t *testing.T) {
	source := &fakeSource{info: eligibleSession(
		domain.CoreFacts{Position: 1, Delivered: true, Answered: true, QuestionScore: sc(80)}, // scored
		domain.CoreFacts{Position: 2, Delivered: true, Answered: true},                        // answered, ungraded: not counted
		domain.CoreFacts{Position: 3, Delivered: true},                                        // delivered, unanswered: skip = 0, counted
		domain.CoreFacts{Position: 4},                                                         // never reached: not counted
	)}
	store := &fakeStore{outcome: service.LinkOutcome{Written: 2, Present: 2, Mappable: 2}}
	result, err := service.NewAssessmentService(source, store).LinkPracticeResults(context.Background(), uid(1))
	if err != nil {
		t.Fatal(err)
	}
	if len(store.positions) != 1 {
		t.Fatalf("store called %d times, want exactly one atomic write", len(store.positions))
	}
	got := store.positions[0]
	if len(got) != 2 || got[0] != 1 || got[1] != 3 {
		t.Fatalf("positions = %v, want [1 3] (the scored core and the skipped core)", got)
	}
	if store.owner != uid(9) {
		t.Fatalf("store got owner %s, want the session's candidate", store.owner)
	}
	if !result.Eligible || result.Counted != 2 || result.Linked != 2 || result.Unlinked != 0 {
		t.Fatalf("result = %+v", result)
	}
}

func TestLinkTellsLinkedAndUnmappedCoresFromRefusedOnes(t *testing.T) {
	info := eligibleSession(
		domain.CoreFacts{Position: 1, Delivered: true, Answered: true, QuestionScore: sc(80)},
		domain.CoreFacts{Position: 2, Delivered: true, Answered: true, QuestionScore: sc(70)},
		domain.CoreFacts{Position: 3, Delivered: true, Answered: true, QuestionScore: sc(60)},
	)
	for name, tc := range map[string]struct {
		outcome      service.LinkOutcome
		wantLinked   int
		wantUnlinked int
	}{
		"all newly written":                           {service.LinkOutcome{Written: 3, Present: 3, Mappable: 3}, 3, 0},
		"all linked by another worker already":        {service.LinkOutcome{Written: 0, Present: 3, Mappable: 3}, 0, 0},
		"one core has no topic (not a failure)":       {service.LinkOutcome{Written: 2, Present: 2, Mappable: 2}, 2, 0},
		"no core has a topic":                         {service.LinkOutcome{Written: 0, Present: 0, Mappable: 0}, 0, 0},
		"one mappable core refused by the link rules": {service.LinkOutcome{Written: 2, Present: 2, Mappable: 3}, 2, 1},
		"all mappable cores refused":                  {service.LinkOutcome{Written: 0, Present: 0, Mappable: 3}, 0, 3},
	} {
		res, err := service.NewAssessmentService(&fakeSource{info: info}, &fakeStore{outcome: tc.outcome}).LinkPracticeResults(context.Background(), uid(1))
		if err != nil || res.Linked != tc.wantLinked || res.Unlinked != tc.wantUnlinked {
			t.Errorf("%s: result = %+v, err %v, want linked %d unlinked %d", name, res, err, tc.wantLinked, tc.wantUnlinked)
		}
	}
}

func TestLinkSkipsSessionsThatAreNotEvidence(t *testing.T) {
	done := time.Date(2026, 11, 1, 9, 0, 0, 0, time.UTC)
	core := domain.CoreFacts{Position: 1, Delivered: true, Answered: true, QuestionScore: sc(90)}
	tests := map[string]func(*service.SessionInfo){
		"recruitment":        func(i *service.SessionInfo) { i.Session.Type = "recruitment" },
		"unfinished":         func(i *service.SessionInfo) { i.Session.CompletedAt = nil },
		"evaluation pending": func(i *service.SessionInfo) { i.Session.EvaluationStatus = "pending" },
		"evaluation failed":  func(i *service.SessionInfo) { i.Session.EvaluationStatus = "failed" },
		"injection flagged":  func(i *service.SessionInfo) { i.Session.InjectionFlagged = true },
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			info := eligibleSession(core)
			info.Session.CompletedAt = &done
			mutate(&info)
			store := &fakeStore{}
			result, err := service.NewAssessmentService(&fakeSource{info: info}, store).LinkPracticeResults(context.Background(), uid(1))
			if err != nil || result.Eligible || len(store.positions) != 0 {
				t.Fatalf("result = %+v, err = %v, store calls = %d: a session that is not evidence must write nothing", result, err, len(store.positions))
			}
		})
	}
}

func TestLinkNothingCountedWritesNothing(t *testing.T) {
	store := &fakeStore{}
	source := &fakeSource{info: eligibleSession(domain.CoreFacts{Position: 1}, domain.CoreFacts{Position: 2, Delivered: true, Answered: true})}
	result, err := service.NewAssessmentService(source, store).LinkPracticeResults(context.Background(), uid(1))
	if err != nil || !result.Eligible || result.Counted != 0 || len(store.positions) != 0 {
		t.Fatalf("result = %+v, err = %v, calls = %d", result, err, len(store.positions))
	}
}

func TestLinkPropagatesStoreAndSourceErrors(t *testing.T) {
	boom := errors.New("db down")
	info := eligibleSession(domain.CoreFacts{Position: 1, Delivered: true, Answered: true, QuestionScore: sc(50)})
	if _, err := service.NewAssessmentService(&fakeSource{info: info}, &fakeStore{err: boom}).LinkPracticeResults(context.Background(), uid(1)); !errors.Is(err, boom) {
		t.Fatalf("store error = %v, want it wrapped", err)
	}
	if _, err := service.NewAssessmentService(&fakeSource{err: boom}, &fakeStore{}).LinkPracticeResults(context.Background(), uid(1)); !errors.Is(err, boom) {
		t.Fatalf("source error = %v, want it wrapped", err)
	}
}

// --- The evaluation hook ---

type fakeEvaluator struct {
	err   error
	calls int
}

func (f *fakeEvaluator) Evaluate(context.Context, uuid.UUID) error { f.calls++; return f.err }

type fakeLinker struct {
	calls int
	err   error
	boom  any
}

func (f *fakeLinker) LinkPracticeResults(context.Context, uuid.UUID) (service.LinkResult, error) {
	f.calls++
	if f.boom != nil {
		panic(f.boom)
	}
	return service.LinkResult{}, f.err
}

func TestEvaluateThenLinkRunsTheLinkerAfterASuccessfulEvaluation(t *testing.T) {
	inner, linker := &fakeEvaluator{}, &fakeLinker{}
	if err := service.NewEvaluateThenLink(inner, linker).Evaluate(context.Background(), uid(1)); err != nil {
		t.Fatal(err)
	}
	if inner.calls != 1 || linker.calls != 1 {
		t.Fatalf("calls = %d/%d, want 1/1", inner.calls, linker.calls)
	}
}

func TestEvaluateThenLinkDoesNotLinkAFailedEvaluation(t *testing.T) {
	boom := errors.New("grader failed")
	inner, linker := &fakeEvaluator{err: boom}, &fakeLinker{}
	err := service.NewEvaluateThenLink(inner, linker).Evaluate(context.Background(), uid(1))
	if !errors.Is(err, boom) || linker.calls != 0 {
		t.Fatalf("err = %v, linker calls = %d: the evaluation error must pass through untouched and nothing may be linked", err, linker.calls)
	}
}

func TestEvaluateThenLinkNeverFailsTheEvaluationBecauseOfTheRoadmap(t *testing.T) {
	for name, linker := range map[string]*fakeLinker{
		"error": {err: errors.New("link failed")},
		"panic": {boom: "linker exploded"},
	} {
		t.Run(name, func(t *testing.T) {
			if err := service.NewEvaluateThenLink(&fakeEvaluator{}, linker).Evaluate(context.Background(), uid(1)); err != nil {
				t.Fatalf("Evaluate = %v: a Roadmap failure must not undo or fail the interview result (the catch-up job links later)", err)
			}
			if linker.calls != 1 {
				t.Fatalf("linker calls = %d", linker.calls)
			}
		})
	}
}
