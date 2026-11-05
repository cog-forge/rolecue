package provider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

// A redirect must not carry the subscription key to another host: the answer is the 3xx itself, which is
// an error, and the target never sees a request.
func TestAzureDoesNotFollowRedirectsWithTheKey(t *testing.T) {
	var targetHits atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		targetHits.Add(1)
		_, _ = w.Write([]byte(azureBody))
	}))
	t.Cleanup(target.Close)
	a := stub(t, func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL+"/cognitiveservices/voices/list", http.StatusFound)
	})
	if _, err := a.ListVoices(context.Background()); err == nil || !strings.Contains(err.Error(), "302") {
		t.Fatalf("ListVoices error = %v, want the redirect status", err)
	}
	if n := targetHits.Load(); n != 0 {
		t.Fatalf("the redirect target received %d requests", n)
	}
}
