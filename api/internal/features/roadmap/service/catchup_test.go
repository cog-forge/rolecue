package service_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/cog-forge/rolecue/api/internal/features/roadmap/service"
)

type fakeLister struct {
	pending   []uuid.UUID // sessions still missing links, oldest first
	limits    []int32
	excluded  [][]uuid.UUID
	err       error
	listCalls int
}

func (f *fakeLister) ListSessionsMissingAssessments(_ context.Context, limit int32, exclude []uuid.UUID) ([]uuid.UUID, error) {
	f.listCalls++
	f.limits = append(f.limits, limit)
	f.excluded = append(f.excluded, append([]uuid.UUID(nil), exclude...))
	if f.err != nil {
		return nil, f.err
	}
	skip := map[uuid.UUID]bool{}
	for _, id := range exclude {
		skip[id] = true
	}
	var out []uuid.UUID
	for _, id := range f.pending {
		if skip[id] || int32(len(out)) >= limit {
			continue
		}
		out = append(out, id)
	}
	return out, nil
}

type scriptedLinker struct {
	lister  *fakeLister
	results map[uuid.UUID]service.LinkResult
	errs    map[uuid.UUID]error
	calls   []uuid.UUID
}

func (l *scriptedLinker) LinkPracticeResults(_ context.Context, id uuid.UUID) (service.LinkResult, error) {
	l.calls = append(l.calls, id)
	if err := l.errs[id]; err != nil {
		return service.LinkResult{}, err
	}
	res, ok := l.results[id]
	if !ok {
		res = service.LinkResult{Eligible: true, Counted: 1, Linked: 1}
	}
	if res.Linked > 0 {
		var rest []uuid.UUID
		for _, p := range l.lister.pending {
			if p != id {
				rest = append(rest, p)
			}
		}
		l.lister.pending = rest
	}
	return res, nil
}

func ids(n int) []uuid.UUID {
	out := make([]uuid.UUID, n)
	for i := range out {
		out[i] = uid(byte(100 + i))
	}
	return out
}

var catchUpSettings = service.CatchUpSettings{BatchSize: 3, BackoffBase: time.Minute, BackoffMax: 8 * time.Minute}

func newCatchUp(t *testing.T, lister *fakeLister, linker *scriptedLinker) *service.CatchUp {
	t.Helper()
	c, err := service.NewCatchUp(lister, linker, catchUpSettings)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestCatchUpRejectsBadSettings(t *testing.T) {
	l := &fakeLister{}
	for name, s := range map[string]service.CatchUpSettings{
		"zero batch":          {BatchSize: 0, BackoffBase: time.Second, BackoffMax: time.Minute},
		"huge batch":          {BatchSize: 100000, BackoffBase: time.Second, BackoffMax: time.Minute},
		"zero backoff":        {BatchSize: 10, BackoffBase: 0, BackoffMax: time.Minute},
		"max below base":      {BatchSize: 10, BackoffBase: time.Minute, BackoffMax: time.Second},
		"negative base delay": {BatchSize: 10, BackoffBase: -time.Second, BackoffMax: time.Minute},
	} {
		if _, err := service.NewCatchUp(l, &scriptedLinker{lister: l}, s); err == nil {
			t.Errorf("%s accepted", name)
		}
	}
}

func TestCatchUpTakesAtMostOneBatchPerRunAndDrainsTheBacklogOverRuns(t *testing.T) {
	lister := &fakeLister{pending: ids(7)}
	linker := &scriptedLinker{lister: lister}
	c := newCatchUp(t, lister, linker)
	now := time.Date(2026, 11, 13, 12, 0, 0, 0, time.UTC)

	var perRun []int
	for run := 0; run < 4 && len(lister.pending) > 0; run++ {
		n, err := c.Run(context.Background(), now.Add(time.Duration(run)*time.Minute))
		if err != nil {
			t.Fatal(err)
		}
		perRun = append(perRun, n)
	}
	if len(perRun) != 3 || perRun[0] != 3 || perRun[1] != 3 || perRun[2] != 1 {
		t.Fatalf("sessions linked per run = %v, want [3 3 1]: bounded batches, oldest first, until the backlog is gone", perRun)
	}
	for _, l := range lister.limits {
		if l != 3 {
			t.Fatalf("list limit = %d, want the batch size 3", l)
		}
	}
	// Oldest first: the first session listed is the first processed.
	if linker.calls[0] != ids(7)[0] {
		t.Fatal("the oldest session must be processed first")
	}
}

func TestCatchUpDoesNothingWhenNothingIsMissing(t *testing.T) {
	lister := &fakeLister{}
	linker := &scriptedLinker{lister: lister}
	n, err := newCatchUp(t, lister, linker).Run(context.Background(), time.Now())
	if n != 0 || err != nil || len(linker.calls) != 0 {
		t.Fatalf("n = %d, err = %v, calls = %d", n, err, len(linker.calls))
	}
}

func TestCatchUpFailureBacksOffAndNeverStarvesTheRest(t *testing.T) {
	all := ids(5)
	poison := all[0]
	lister := &fakeLister{pending: all}
	linker := &scriptedLinker{lister: lister, errs: map[uuid.UUID]error{poison: errors.New("fk violation for answer text SECRET-ANSWER")}}
	c := newCatchUp(t, lister, linker)
	now := time.Date(2026, 11, 13, 12, 0, 0, 0, time.UTC)

	n, err := c.Run(context.Background(), now)
	if n != 2 {
		t.Fatalf("linked = %d, want the 2 healthy sessions of the batch", n)
	}
	if err == nil {
		t.Fatal("a failed session must be reported as an error so the run is counted as failed")
	}
	if strings.Contains(err.Error(), "SECRET-ANSWER") {
		t.Fatalf("the returned error leaks the cause: %v", err)
	}

	// Next run, still inside the back-off: the poison session is excluded, so the batch moves on.
	n, err = c.Run(context.Background(), now.Add(30*time.Second))
	if err != nil || n != 2 {
		t.Fatalf("second run linked %d (err %v), want the two remaining healthy sessions", n, err)
	}
	last := lister.excluded[len(lister.excluded)-1]
	if len(last) != 1 || last[0] != poison {
		t.Fatalf("excluded = %v, want the poison session during its back-off", last)
	}

	// After the back-off it is tried again, and the delay doubles when it fails again.
	callsBefore := len(linker.calls)
	if _, err = c.Run(context.Background(), now.Add(2*time.Minute)); err == nil {
		t.Fatal("the retry of the poison session must fail again")
	}
	if len(linker.calls) != callsBefore+1 || linker.calls[callsBefore] != poison {
		t.Fatal("the poison session must be retried after its back-off")
	}
	if _, err = c.Run(context.Background(), now.Add(2*time.Minute+90*time.Second)); err != nil {
		t.Fatalf("90s after the second failure it must still be backed off (delay doubled to 2 minutes): %v", err)
	}
	if _, err = c.Run(context.Background(), now.Add(2*time.Minute+3*time.Minute)); err == nil {
		t.Fatal("after the doubled delay it is tried again")
	}
}

func TestCatchUpTreatsCountedButUnlinkableSessionsAsStuck(t *testing.T) {
	// The store refused every position (for example a topic that no longer matches): nothing was linked although
	// results count. Without a back-off the same sessions would be listed first on every run and starve the rest.
	all := ids(4)
	stuck := all[0]
	lister := &fakeLister{pending: all}
	linker := &scriptedLinker{lister: lister, results: map[uuid.UUID]service.LinkResult{stuck: {Eligible: true, Counted: 2, Linked: 0, Unlinked: 2}}}
	c := newCatchUp(t, lister, linker)
	now := time.Now()
	if _, err := c.Run(context.Background(), now); err == nil {
		t.Fatal("a session that counts but cannot be linked must be reported")
	}
	n, err := c.Run(context.Background(), now.Add(time.Second))
	if err != nil || n != 1 {
		t.Fatalf("second run linked %d (err %v): the stuck session must not block the last healthy one", n, err)
	}
}

func TestCatchUpRecoversWhenALaterRunSucceeds(t *testing.T) {
	all := ids(1)
	lister := &fakeLister{pending: all}
	linker := &scriptedLinker{lister: lister, errs: map[uuid.UUID]error{all[0]: errors.New("temporary")}}
	c := newCatchUp(t, lister, linker)
	now := time.Now()
	if _, err := c.Run(context.Background(), now); err == nil {
		t.Fatal("first run must fail")
	}
	delete(linker.errs, all[0])
	n, err := c.Run(context.Background(), now.Add(time.Hour))
	if err != nil || n != 1 || len(lister.pending) != 0 {
		t.Fatalf("n = %d, err = %v, pending = %d: after the back-off the session must be linked", n, err, len(lister.pending))
	}
}

func TestCatchUpListErrorIsReturned(t *testing.T) {
	boom := errors.New("db down")
	lister := &fakeLister{err: boom}
	if _, err := newCatchUp(t, lister, &scriptedLinker{lister: lister}).Run(context.Background(), time.Now()); !errors.Is(err, boom) {
		t.Fatalf("err = %v", err)
	}
}

func TestCatchUpStopsWhenTheContextIsCancelled(t *testing.T) {
	lister := &fakeLister{pending: ids(3)}
	linker := &scriptedLinker{lister: lister}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	n, err := newCatchUp(t, lister, linker).Run(ctx, time.Now())
	if n != 0 || len(linker.calls) != 0 {
		t.Fatalf("n = %d, calls = %d: a cancelled run must not link", n, len(linker.calls))
	}
	if err == nil {
		t.Fatal("a cancelled run reports the cancellation")
	}
}
