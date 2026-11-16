package llmerr

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

var noSleep = func(context.Context, time.Duration) bool { return true }

func TestReasonSurvivesWrapping(t *testing.T) {
	err := fmt.Errorf("outer: %w", NewFailure(ReasonRateLimited))
	if Reason(err) != ReasonRateLimited || !Transient(err) {
		t.Fatalf("reason %q transient %v", Reason(err), Transient(err))
	}
	if Transient(errors.New("plain")) || Transient(NewFailure("http_4xx")) || Transient(NewFailure("auth_rejected")) {
		t.Fatal("non-provider and client errors must not be transient")
	}
	if Transient(NewFailure(ReasonInvalidOutput)) || !TransientOrInvalid(NewFailure(ReasonInvalidOutput)) {
		t.Fatal("invalid output is retried by generation only")
	}
}

func TestDoRetriesOnlyTransientErrorsUpToTheLimit(t *testing.T) {
	calls := 0
	p := Policy{MaxRetries: 2, Sleep: noSleep}
	got, err := Do(context.Background(), p, func(context.Context) (int, error) {
		calls++
		if calls < 3 {
			return 0, NewFailure(ReasonHTTP5xx)
		}
		return 7, nil
	})
	if err != nil || got != 7 || calls != 3 {
		t.Fatalf("got %d err %v calls %d", got, err, calls)
	}
	calls = 0
	_, err = Do(context.Background(), p, func(context.Context) (int, error) { calls++; return 0, NewFailure(ReasonRateLimited) })
	if Reason(err) != ReasonRateLimited || calls != 3 {
		t.Fatalf("exhaustion: err %v calls %d", err, calls)
	}
	calls = 0
	_, err = Do(context.Background(), p, func(context.Context) (int, error) { calls++; return 0, NewFailure("auth_rejected") })
	if err == nil || calls != 1 {
		t.Fatalf("permanent failure: err %v calls %d", err, calls)
	}
}

func TestDoStopsWhenTheDeadlineHasNoRoomOrTheContextEnds(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	calls := 0
	p := Policy{MaxRetries: 5, Backoff: func(int) time.Duration { return time.Millisecond }, MinRemaining: time.Minute, Sleep: noSleep}
	_, err := Do(ctx, p, func(context.Context) (int, error) { calls++; return 0, NewFailure(ReasonTimeout) })
	if err == nil || calls != 1 {
		t.Fatalf("no room: err %v calls %d", err, calls)
	}
	cancelled, stop := context.WithCancel(context.Background())
	calls = 0
	_, err = Do(cancelled, Policy{MaxRetries: 5, Sleep: noSleep}, func(context.Context) (int, error) {
		calls++
		stop()
		return 0, NewFailure(ReasonTimeout)
	})
	if err == nil || calls != 1 {
		t.Fatalf("cancelled: err %v calls %d", err, calls)
	}
}

func TestExponentialBackoffIsCapped(t *testing.T) {
	b := Exponential(time.Second, 3*time.Second)
	if b(1) != time.Second || b(2) != 2*time.Second || b(3) != 3*time.Second || b(40) != 3*time.Second {
		t.Fatalf("%v %v %v %v", b(1), b(2), b(3), b(40))
	}
}
