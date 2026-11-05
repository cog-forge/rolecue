package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cog-forge/rolecue/api/internal/features/interview/domain"
	"github.com/gorilla/websocket"
)

func TestTokenBucketRefillsOverTime(t *testing.T) {
	now := time.Unix(1000, 0)
	b := newTokenBucket(2, 3, func() time.Time { return now })
	for i := 0; i < 3; i++ {
		if !b.allow() {
			t.Fatalf("burst token %d refused", i)
		}
	}
	if b.allow() {
		t.Fatal("empty bucket allowed a message")
	}
	now = now.Add(500 * time.Millisecond) // one token at 2/s
	if !b.allow() || b.allow() {
		t.Fatal("half a second at 2/s must refill exactly one token")
	}
	now = now.Add(time.Hour)
	for i := 0; i < 3; i++ {
		if !b.allow() {
			t.Fatalf("refill must cap at the burst, token %d refused", i)
		}
	}
	if b.allow() {
		t.Fatal("refill exceeded the burst")
	}
}

// A revoked login (logout, ban, password reset) closes the open socket at the next re-check.
func TestRevokedLoginClosesTheSocketWithinTheInterval(t *testing.T) {
	var revoked atomic.Bool
	verify := func(context.Context) error {
		if revoked.Load() {
			return ErrSessionInvalid
		}
		return nil
	}
	h := newHarness(t, newFakeSessions(domain.TypePractice, "Q1", "Q2"),
		withGuards(Settings{RevalidateInterval: 30 * time.Millisecond}), withVerify(verify))
	c, _, _ := h.connect()

	time.Sleep(100 * time.Millisecond) // several checks pass while the login holds
	c.send(TypePing, "p1", empty{})
	c.expect(TypePong, nil)

	revoked.Store(true)
	c.expectClose(CloseSessionInvalid)
}

// A failing check (the auth service is down) is not a revocation: the socket stays open.
func TestVerifierOutageKeepsTheSocket(t *testing.T) {
	var calls atomic.Int32
	verify := func(context.Context) error {
		calls.Add(1)
		return errors.New("auth service unavailable")
	}
	h := newHarness(t, newFakeSessions(domain.TypePractice, "Q1"),
		withGuards(Settings{RevalidateInterval: 20 * time.Millisecond}), withVerify(verify))
	c, _, _ := h.connect()

	deadline := time.Now().Add(2 * time.Second)
	for calls.Load() < 3 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if calls.Load() < 3 {
		t.Fatalf("verifier ran %d times, want at least 3", calls.Load())
	}
	c.send(TypePing, "p1", empty{})
	c.expect(TypePong, nil)
}

// A client that floods pings is throttled, told so, and finally closed with 1008.
func TestMessageFloodIsThrottledThenClosed(t *testing.T) {
	h := newHarness(t, newFakeSessions(domain.TypePractice, "Q1"), withGuards(Settings{MessageRate: 1, MessageBurst: 5}))
	c, _, _ := h.connect()

	go func() {
		for i := 0; i < 400; i++ {
			raw, _ := json.Marshal(ClientMessage{V: ProtocolVersion, ID: fmt.Sprintf("p%d", i), Type: TypePing})
			if c.ws.WriteMessage(websocket.TextMessage, raw) != nil {
				return
			}
		}
	}()

	pongs, limited, closedClean := 0, 0, false
	_ = c.ws.SetReadDeadline(time.Now().Add(5 * time.Second))
	for {
		_, data, err := c.ws.ReadMessage()
		if err != nil {
			// The server closes with 1008 while this client is still writing, so the OS may answer
			// with a TCP reset that discards the close frame. Both endings mean the flood was cut off.
			var closeErr *websocket.CloseError
			var opErr *net.OpError
			switch {
			case errors.As(err, &closeErr) && closeErr.Code == websocket.ClosePolicyViolation:
				closedClean = true
			case errors.As(err, &closeErr) && closeErr.Code == websocket.CloseAbnormalClosure:
			case errors.As(err, &opErr), errors.Is(err, io.EOF), errors.Is(err, io.ErrUnexpectedEOF):
			default:
				t.Fatalf("flood ended with %v, want close 1008 or a reset connection", err)
			}
			break
		}
		var msg received
		if err := json.Unmarshal(data, &msg); err != nil {
			t.Fatal(err)
		}
		switch msg.Type {
		case TypePong:
			pongs++
		case TypeError:
			var e ErrorData
			_ = json.Unmarshal(msg.Data, &e)
			if e.Code == CodeRateLimited {
				limited++
			}
		}
	}
	if pongs > 8 {
		t.Fatalf("answered %d pings of a flood, the bucket allows about 5", pongs)
	}
	// A reset can drop the notice together with the close frame, so only require it after a clean close.
	if closedClean && limited == 0 {
		t.Fatal("the flooding client was never told it was rate limited")
	}
}

// A well-behaved client stays far below the default limits.
func TestNormalTrafficIsNotThrottled(t *testing.T) {
	h := newHarness(t, newFakeSessions(domain.TypePractice, "Q1"))
	c, _, _ := h.connect()
	for i := 0; i < 40; i++ {
		c.send(TypePing, fmt.Sprintf("p%d", i), empty{})
		c.expect(TypePong, nil)
	}
}
