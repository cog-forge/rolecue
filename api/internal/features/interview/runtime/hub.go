package runtime

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
)

// errShuttingDown refuses a new connection while the API stops.
var errShuttingDown = errors.New("live interviews are shutting down")

// Hub keeps the in-process state of each live session: the stream that numbers its messages, the ids of
// the commands it accepted, the lock that serializes its turns, and the one socket that drives it. It is
// memory only and fits a single API instance; everything a session needs to recover is in the database.
type Hub struct {
	mu       sync.Mutex
	streams  map[uuid.UUID]*sessionStream
	closing  bool
	shutdown chan struct{}
	active   sync.WaitGroup
	now      func() time.Time
}

// NewHub returns an empty hub.
func NewHub() *Hub {
	return &Hub{streams: make(map[uuid.UUID]*sessionStream), shutdown: make(chan struct{}), now: time.Now}
}

// sessionStream is the hub state of one session.
type sessionStream struct {
	id  string
	seq atomic.Int64
	// turn serializes the turn work of the session (deliver, answer, complete) across the sockets of this
	// process, so a socket that takes over waits for the answer its predecessor is still storing.
	turn sync.Mutex

	// The fields below are guarded by Hub.mu.
	current    *lease
	attached   int
	everJoined bool
	detachedAt time.Time

	seenMu sync.Mutex
	seen   map[string]struct{}
	order  []string
}

// lease is one socket's hold on a session stream.
type lease struct {
	stream     *sessionStream
	superseded chan struct{}
	resumed    bool
}

func (s *sessionStream) nextSeq() int64 { return s.seq.Add(1) }

// accepted reports whether a command id was already accepted for the session.
func (s *sessionStream) accepted(id string) bool {
	s.seenMu.Lock()
	defer s.seenMu.Unlock()
	_, ok := s.seen[id]
	return ok
}

// accept remembers a command id, forgetting the oldest beyond rememberedCommandIDs.
func (s *sessionStream) accept(id string) {
	if id == "" {
		return
	}
	s.seenMu.Lock()
	defer s.seenMu.Unlock()
	if _, ok := s.seen[id]; ok {
		return
	}
	s.seen[id] = struct{}{}
	s.order = append(s.order, id)
	if len(s.order) > rememberedCommandIDs {
		delete(s.seen, s.order[0])
		s.order = s.order[1:]
	}
}

// attach gives a new socket the session's stream. An older socket of the same session is superseded:
// its superseded channel closes and it must stop. Streams idle longer than streamRetention are dropped.
func (h *Hub) attach(sessionID uuid.UUID) (*lease, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closing {
		return nil, errShuttingDown
	}
	now := h.now()
	for id, stream := range h.streams {
		if stream.attached == 0 && now.Sub(stream.detachedAt) > streamRetention {
			delete(h.streams, id)
		}
	}
	stream, ok := h.streams[sessionID]
	if !ok {
		stream = &sessionStream{id: uuid.NewString(), seen: make(map[string]struct{})}
		h.streams[sessionID] = stream
	}
	if stream.current != nil {
		close(stream.current.superseded)
	}
	l := &lease{stream: stream, superseded: make(chan struct{}), resumed: stream.everJoined}
	stream.current = l
	stream.attached++
	stream.everJoined = true
	h.active.Add(1)
	return l, nil
}

// release ends a socket's hold. The stream stays for streamRetention so a reconnect keeps its sequence.
func (h *Hub) release(l *lease) {
	h.mu.Lock()
	defer h.mu.Unlock()
	stream := l.stream
	if stream.current == l {
		stream.current = nil
	}
	stream.attached--
	if stream.attached == 0 {
		stream.detachedAt = h.now()
	}
	h.active.Done()
}

// Shutdown closes every live socket with 1001 and waits, until ctx ends, for their in-flight answers to
// be stored. New connections are refused from now on.
func (h *Hub) Shutdown(ctx context.Context) error {
	h.mu.Lock()
	if !h.closing {
		h.closing = true
		close(h.shutdown)
	}
	h.mu.Unlock()
	done := make(chan struct{})
	go func() {
		h.active.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
