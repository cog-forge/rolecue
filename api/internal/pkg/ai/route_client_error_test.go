package ai

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/cloudwego/eino/components/model"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"github.com/cog-forge/rolecue/api/internal/pkg/logger"
)

// A client error (rejected key, bad request, no credit, unknown model, restricted account) from one provider must
// not take the providers behind it down (KAN-386, spec 8.3a).
func TestRouteSkipsEntryOnClientError(t *testing.T) {
	for _, status := range []int{400, 401, 402, 403, 404} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			a := &countedModel{err: &HTTPError{Status: status}}
			b := &countedModel{}
			got, err := runChain(chainOf(NewProviderHealth(time.Now), a, b))
			if err != nil || got != "ok" || a.calls != 1 || b.calls != 1 {
				t.Fatalf("result=%q err=%v a=%d b=%d", got, err, a.calls, b.calls)
			}
		})
	}
}

func TestRouteClientErrorNotRetriedOnSameEntry(t *testing.T) {
	a := &countedModel{err: &HTTPError{Status: 400}}
	b := &countedModel{}
	r := chainOf(NewProviderHealth(time.Now), a, b)
	r.entries[0].config.MaxRetries = 2
	if _, err := runChain(r); err != nil || a.calls != 1 || b.calls != 1 {
		t.Fatalf("err=%v a=%d b=%d", err, a.calls, b.calls)
	}
}

func TestRouteClientErrorStartsNoCooldown(t *testing.T) {
	a := &countedModel{err: &HTTPError{Status: 401}}
	b := &countedModel{}
	r := chainOf(NewProviderHealth(time.Now), a, b)
	_, _ = runChain(r)
	_, _ = runChain(r)
	if a.calls != 2 {
		t.Fatalf("a was called %d times, want 2: a client error is not a cooldown", a.calls)
	}
}

// When every entry fails, the reason that reaches the caller is the most informative one: a congestion error must
// not be hidden behind a configuration-looking one.
func TestRouteFinalErrorPrefersTransient(t *testing.T) {
	for _, tc := range []struct {
		name   string
		errs   []error
		reason string
	}{
		{"5xx then auth", []error{&HTTPError{Status: 503}, &HTTPError{Status: 401}}, "http_5xx"},
		{"429, 503, then no credit", []error{&HTTPError{Status: 429}, &HTTPError{Status: 503}, &HTTPError{Status: 402}}, "http_5xx"},
		{"auth first, congestion last", []error{&HTTPError{Status: 403}, &HTTPError{Status: 429}}, "rate_limited"},
		{"invalid output then bad request", []error{ErrInvalidOutput, &HTTPError{Status: 400}}, "invalid_output"},
		{"all client errors keep the last", []error{&HTTPError{Status: 401}, &HTTPError{Status: 403}, &HTTPError{Status: 400}}, "http_4xx"},
		{"only auth", []error{&HTTPError{Status: 401}, &HTTPError{Status: 403}}, "auth_rejected"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var models []*countedModel
			for _, err := range tc.errs {
				models = append(models, &countedModel{err: err})
			}
			_, err := runChain(chainOf(NewProviderHealth(time.Now), models...))
			if err == nil || llmFailureReason(err) != tc.reason {
				t.Fatalf("reason = %q (error %v), want %q", llmFailureReason(err), err, tc.reason)
			}
			for i, m := range models {
				if m.calls != 1 {
					t.Errorf("entry %d was called %d times, want 1", i, m.calls)
				}
			}
		})
	}
}

func TestRouteStillStopsOnCancelAndUnclassifiedErrors(t *testing.T) {
	for name, stop := range map[string]error{"canceled": context.Canceled, "unclassified": errors.New("programming error")} {
		t.Run(name, func(t *testing.T) {
			a := &countedModel{err: stop}
			b := &countedModel{}
			if _, err := runChain(chainOf(NewProviderHealth(time.Now), a, b)); err == nil {
				t.Fatal("expected an error")
			}
			if a.calls != 1 || b.calls != 0 {
				t.Fatalf("a=%d b=%d: the route must stop at once", a.calls, b.calls)
			}
		})
	}
}

// The skipped entry is named in one Warn line, so a rejected key or a restricted account is visible, and the line
// carries no provider text.
func TestRouteWarnsWhenAProviderRejectsTheCall(t *testing.T) {
	core, logs := observer.New(zapcore.DebugLevel)
	ctx := logger.ToContext(context.Background(), &logger.Logger{Logger: zap.New(core)})
	a := &countedModel{err: &HTTPError{Status: 401}}
	b := &countedModel{}
	r := chainOf(NewProviderHealth(time.Now), a, b)
	if _, err := Run(ctx, r, func(ctx context.Context, m model.BaseChatModel) (string, error) {
		msg, err := m.Generate(ctx, nil)
		if err != nil {
			return "", err
		}
		return msg.Content, nil
	}); err != nil {
		t.Fatal(err)
	}
	warned := logs.FilterMessage("llm provider rejected the call, trying the next entry").All()
	if len(warned) != 1 || warned[0].Level != zapcore.WarnLevel {
		t.Fatalf("rejection lines = %d, want one Warn: %v", len(warned), warned)
	}
	fields := warned[0].ContextMap()
	if fields["provider"] != "p0" || fields["reason"] != "auth_rejected" || fields["status_code"] != int64(401) {
		t.Fatalf("fields = %v", fields)
	}
}
