package runtime

import (
	"bytes"
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/cog-forge/rolecue/api/internal/features/interview/domain"
	"github.com/cog-forge/rolecue/api/internal/features/interview/service"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

// These tests run the engine over the real interview service, with an in-memory repository that applies
// the same domain rules as the database one (domain.NextStep, domain.AcceptAnswer) and a scripted
// follow-up decider.

// internalReason is the decider's reason, which is stored but never sent to the Candidate.
const internalReason = "INTERNAL-REASON the answer skipped burst handling"

// memoryRepository is one practice session in memory.
type memoryRepository struct {
	mu        sync.Mutex
	questions []string
	turns     []domain.Turn
	completed bool
}

func newMemoryRepository(questions ...string) *memoryRepository {
	return &memoryRepository{questions: questions}
}

func (r *memoryRepository) session() domain.Session {
	session := domain.Session{
		ID: testSession, Type: domain.TypePractice, QuestionCount: int32(len(r.questions)),
		AnsweredTurns: int32(len(r.turns)), EvaluationStatus: domain.EvaluationPending,
	}
	for _, turn := range r.turns {
		if turn.Kind == domain.TurnCore {
			session.AnsweredCount++
		}
	}
	if r.completed {
		now := time.Now()
		session.CompletedAt = &now
	}
	return session
}

func (r *memoryRepository) Create(context.Context, domain.CreateParams) (domain.Session, bool, error) {
	return domain.Session{}, false, errors.New("not used")
}

func (r *memoryRepository) Get(context.Context, uuid.UUID, uuid.UUID) (domain.Session, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.session(), nil
}

func (r *memoryRepository) AppendTurn(_ context.Context, _, _ uuid.UUID, position int32, answer string) (domain.Session, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.completed {
		return domain.Session{}, domain.ErrInterviewCompleted
	}
	slot, unchanged, err := domain.AcceptAnswer(r.turns, int32(len(r.questions)), position, answer)
	if err != nil {
		return domain.Session{}, err
	}
	if !unchanged {
		r.turns = append(r.turns, domain.Turn{
			Position: position, QuestionNumber: slot.QuestionNumber, Kind: domain.TurnCore,
			Question: r.questions[slot.QuestionNumber-1], Answer: answer,
		})
	}
	return r.session(), nil
}

func (r *memoryRepository) AppendFollowUp(_ context.Context, _, _ uuid.UUID, followUp domain.AskedQuestion, answer string, limit int32) (domain.Session, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.completed {
		return domain.Session{}, domain.ErrInterviewCompleted
	}
	slot, unchanged, err := domain.AcceptFollowUp(r.turns, limit, followUp.Position, answer)
	if err != nil {
		return domain.Session{}, err
	}
	if !unchanged {
		r.turns = append(r.turns, domain.Turn{
			Position: followUp.Position, QuestionNumber: slot.QuestionNumber, Kind: domain.TurnFollowUp, Question: followUp.Text, Answer: answer,
		})
	}
	return r.session(), nil
}

func (r *memoryRepository) Complete(context.Context, uuid.UUID, uuid.UUID) (domain.Session, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	first := !r.completed
	r.completed = true
	return r.session(), first, nil
}

func (r *memoryRepository) MarkEvaluationFailed(context.Context, uuid.UUID) error { return nil }

func (r *memoryRepository) boundary() (domain.Delivery, error) {
	if r.completed {
		return domain.Delivery{}, domain.ErrInterviewCompleted
	}
	return domain.Delivery{Type: domain.TypePractice, QuestionCount: int32(len(r.questions)), AnsweredCount: int32(len(r.turns)), ProviderVoiceID: "en-US-AvaNeural"}, nil
}

func (r *memoryRepository) Deliver(context.Context, uuid.UUID, uuid.UUID, string) (domain.Delivery, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	d, err := r.boundary()
	if err != nil {
		return d, err
	}
	if slot, ok := domain.NextCore(r.turns, int32(len(r.questions))); ok {
		d.Question = &domain.AskedQuestion{Position: slot.Position, QuestionNumber: slot.QuestionNumber, Kind: domain.TurnCore, Text: r.questions[slot.QuestionNumber-1]}
	}
	return d, nil
}

func (r *memoryRepository) FollowUpContext(_ context.Context, _, _ uuid.UUID, _ string, limit int32) (domain.Delivery, *domain.FollowUpContext, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	d, err := r.boundary()
	if err != nil {
		return d, nil, err
	}
	slot, ok := domain.FollowUpSlot(r.turns, limit)
	if !ok {
		return d, nil, nil
	}
	pending := &domain.FollowUpContext{Position: slot.Position, QuestionNumber: slot.QuestionNumber, Ordinal: slot.Ordinal, CoreQuestion: r.questions[slot.QuestionNumber-1]}
	for _, turn := range r.turns {
		if turn.QuestionNumber == slot.QuestionNumber {
			pending.Exchanges = append(pending.Exchanges, domain.Exchange{Question: turn.Question, Answer: turn.Answer})
		}
	}
	return d, pending, nil
}

func (r *memoryRepository) storedTurns() []domain.Turn {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]domain.Turn(nil), r.turns...)
}

// followUpDecider asks "Follow-up N?" for every slot unless decide is set; gate, when set, holds each call
// until the test releases it.
type followUpDecider struct {
	mu      sync.Mutex
	calls   int
	decide  func(domain.FollowUpContext) (domain.FollowUpProposal, error)
	started chan struct{}
	gate    chan struct{}
}

func (d *followUpDecider) Decide(ctx context.Context, in domain.FollowUpContext) (domain.FollowUpProposal, error) {
	d.mu.Lock()
	d.calls++
	call := d.calls
	d.mu.Unlock()
	if d.started != nil {
		d.started <- struct{}{}
	}
	if d.gate != nil {
		select {
		case <-d.gate:
		case <-ctx.Done():
			return domain.FollowUpProposal{}, ctx.Err()
		}
	}
	if d.decide != nil {
		return d.decide(in)
	}
	return domain.FollowUpProposal{FollowUp: true, Question: followUpText(call), Reason: internalReason}, nil
}

func (d *followUpDecider) count() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.calls
}

func followUpText(call int) string {
	return "Follow-up " + string(rune('0'+call)) + ": what would you change under load?"
}

func followUpHarness(t *testing.T, repo *memoryRepository, decider *followUpDecider, limit int32) *harness {
	t.Helper()
	svc, err := service.New(repo, service.Settings{FollowUpLimit: limit, FollowUpTimeout: 300 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	if decider != nil {
		svc.SetFollowUpDecider(decider)
	}
	return serve(t, svc)
}

// answer plays the open turn, answers it by text and reads up to the next turn (or completion).
func (c *client) answer(position int32, text string) (InterviewerTurn, bool) {
	c.t.Helper()
	c.listen("played-"+text, position)
	c.send(TypeAnswerText, "answer-"+text, AnswerText{Position: position, Text: text})
	c.expectState(StateProcessing)
	var accepted AnswerAccepted
	c.expect(TypeAnswerAccepted, &accepted)
	if accepted.Position != position || accepted.AnsweredCount != position || accepted.Duplicate {
		c.t.Fatalf("accepted = %+v", accepted)
	}
	c.expectState(StateTransitioning)
	var change StateChange
	c.expect(TypeState, &change)
	if change.State == StateCompleted {
		c.expect(TypeSessionCompleted, nil)
		c.expectClose(websocket.CloseNormalClosure)
		return InterviewerTurn{}, false
	}
	if change.State != StateInterviewerSpeaking {
		c.t.Fatalf("state = %s", change.State)
	}
	var turn InterviewerTurn
	c.expect(TypeInterviewerTurn, &turn)
	return turn, true
}

// assertNothingHidden checks every frame the client saw: no core question id and no decider reason ever
// reaches the Candidate. (The repository never reads unsampled bank questions; the integration tests
// cover that.)
func (c *client) assertNothingHidden() {
	c.t.Helper()
	for _, msg := range c.seen {
		for _, secret := range []string{"INTERNAL-REASON", "core_question_id", "reason"} {
			if bytes.Contains(msg.Data, []byte(secret)) {
				c.t.Fatalf("%s frame leaks %q: %s", msg.Type, secret, msg.Data)
			}
		}
	}
}

func TestFollowUpLimitZeroIsCoreQuestionsOnly(t *testing.T) {
	repo := newMemoryRepository("Q1", "Q2")
	decider := &followUpDecider{}
	h := followUpHarness(t, repo, decider, 0)
	c, sync, turn := h.connect()
	if sync.QuestionNumber == nil || *sync.QuestionNumber != 1 || turn.QuestionNumber != 1 || turn.Kind != "core" {
		t.Fatalf("sync = %+v, turn = %+v", sync, turn)
	}
	turn, _ = c.answer(1, "one")
	if turn.Position != 2 || turn.QuestionNumber != 2 || turn.Kind != "core" || turn.Text != "Q2" {
		t.Fatalf("second turn = %+v", turn)
	}
	if _, open := c.answer(2, "two"); open {
		t.Fatal("the session did not complete after the last core question")
	}
	if decider.count() != 0 {
		t.Fatalf("the decider ran %d times with limit 0", decider.count())
	}
	c.assertNothingHidden()
}

func TestFollowUpLimitOneAsksAtMostOnePerCoreQuestion(t *testing.T) {
	repo := newMemoryRepository("Q1", "Q2")
	decider := &followUpDecider{}
	h := followUpHarness(t, repo, decider, 1)
	c, _, _ := h.connect()

	turn, _ := c.answer(1, "one")
	if turn.Position != 2 || turn.QuestionNumber != 1 || turn.Kind != "follow_up" || turn.Text != followUpText(1) || turn.QuestionCount != 2 {
		t.Fatalf("follow-up = %+v", turn)
	}
	// A follow-up is spoken like a core turn.
	if turn.AudioStatus != AudioReady || turn.Audio == nil || len(turn.Visemes) == 0 || len(turn.BlendShapes) == 0 {
		t.Fatalf("follow-up speech = %s with %d visemes", turn.AudioStatus, len(turn.Visemes))
	}
	turn, _ = c.answer(2, "one-more")
	if turn.Position != 3 || turn.QuestionNumber != 2 || turn.Kind != "core" || turn.Text != "Q2" {
		t.Fatalf("after the follow-up = %+v, want the next core question", turn)
	}
	// The last core question gets its follow-up before the session completes.
	turn, _ = c.answer(3, "two")
	if turn.Position != 4 || turn.QuestionNumber != 2 || turn.Kind != "follow_up" {
		t.Fatalf("last follow-up = %+v", turn)
	}
	if _, open := c.answer(4, "two-more"); open {
		t.Fatal("the session did not complete after the last follow-up")
	}
	if decider.count() != 2 {
		t.Fatalf("the decider ran %d times, want once per core question", decider.count())
	}
	kinds := ""
	for _, stored := range repo.storedTurns() {
		kinds += string(stored.Kind) + "/" + string(rune('0'+stored.QuestionNumber)) + " "
	}
	if kinds != "core/1 follow_up/1 core/2 follow_up/2 " {
		t.Fatalf("stored turns = %s", kinds)
	}
	c.assertNothingHidden()
}

func TestADeclinedOrFailedDecisionMovesToTheNextCoreQuestion(t *testing.T) {
	tests := map[string]*followUpDecider{
		"declined": {decide: func(domain.FollowUpContext) (domain.FollowUpProposal, error) {
			return domain.FollowUpProposal{Reason: "complete"}, nil
		}},
		"provider error": {decide: func(domain.FollowUpContext) (domain.FollowUpProposal, error) {
			return domain.FollowUpProposal{}, errors.New("503")
		}},
		"timeout": {gate: make(chan struct{})},
		"invalid output": {decide: func(domain.FollowUpContext) (domain.FollowUpProposal, error) {
			return domain.FollowUpProposal{FollowUp: true, Question: "Bạn nghĩ sao?", Reason: "r"}, nil
		}},
		"two questions": {decide: func(domain.FollowUpContext) (domain.FollowUpProposal, error) {
			return domain.FollowUpProposal{FollowUp: true, Question: "Why? How?", Reason: "r"}, nil
		}},
		"missing reason": {decide: func(domain.FollowUpContext) (domain.FollowUpProposal, error) {
			return domain.FollowUpProposal{FollowUp: true, Question: "Why?"}, nil
		}},
	}
	for name, decider := range tests {
		t.Run(name, func(t *testing.T) {
			repo := newMemoryRepository("Q1", "Q2")
			h := followUpHarness(t, repo, decider, 1)
			c, _, _ := h.connect()
			started := time.Now()
			turn, _ := c.answer(1, "one")
			if turn.Position != 2 || turn.QuestionNumber != 2 || turn.Kind != "core" || turn.Text != "Q2" {
				t.Fatalf("next turn = %+v, want core question 2", turn)
			}
			if elapsed := time.Since(started); elapsed > 2*time.Second {
				t.Fatalf("the next turn took %s", elapsed)
			}
			if decider.count() != 1 {
				t.Fatalf("decider calls = %d", decider.count())
			}
		})
	}
}

// A follow-up lives only in the socket that asked it: when the socket drops during it, the reconnect asks
// the next core question at the same position, and nothing is decided again (team decision 2026-11-03).
func TestReconnectDuringAFollowUpMovesToTheNextCoreQuestion(t *testing.T) {
	repo := newMemoryRepository("Q1", "Q2")
	decider := &followUpDecider{}
	h := followUpHarness(t, repo, decider, 1)
	first, _, _ := h.connect()
	followUp, _ := first.answer(1, "one")
	if followUp.Kind != "follow_up" || followUp.Position != 2 {
		t.Fatalf("follow-up = %+v", followUp)
	}
	first.listen("played-follow-up", followUp.Position)
	_ = first.ws.NetConn().Close()

	second := h.dial()
	var sync SessionSync
	second.expect(TypeSessionSync, &sync)
	if sync.Position == nil || *sync.Position != 2 || sync.QuestionNumber == nil || *sync.QuestionNumber != 2 || sync.AnsweredCount != 1 {
		t.Fatalf("sync = %+v", sync)
	}
	var turn InterviewerTurn
	second.expect(TypeInterviewerTurn, &turn)
	if turn.Kind != "core" || turn.Position != 2 || turn.QuestionNumber != 2 || turn.Text != "Q2" {
		t.Fatalf("turn after reconnect = %+v, want core question 2", turn)
	}
	if _, open := second.answer(2, "two"); !open {
		// Q2 gets its own follow-up, so the session is still open.
		t.Fatal("the session completed before question 2's follow-up")
	}
	if decider.count() != 2 {
		t.Fatalf("decider calls = %d, want one per new core answer", decider.count())
	}
	if turns := repo.storedTurns(); len(turns) != 2 || turns[1].Kind != domain.TurnCore {
		t.Fatalf("stored turns = %+v; the dropped follow-up must leave no trace", turns)
	}
}

// A socket that drops while the follow-up is being decided: the reconnect waits for the turn lock, then
// goes on with the next core question; nothing is decided twice.
func TestReconnectWhileDecidingGoesToTheNextCoreQuestion(t *testing.T) {
	repo := newMemoryRepository("Q1", "Q2")
	decider := &followUpDecider{started: make(chan struct{}, 2), gate: make(chan struct{})}
	h := followUpHarness(t, repo, decider, 1)
	first, _, _ := h.connect()
	first.listen("p1", 1)
	first.send(TypeAnswerText, "a1", AnswerText{Position: 1, Text: "one"})
	first.expectState(StateProcessing)
	first.expect(TypeAnswerAccepted, nil)
	first.expectState(StateTransitioning)
	<-decider.started
	_ = first.ws.NetConn().Close()

	// The new socket waits for the turn lock, which the first socket holds until its decision ends.
	second := h.dial()
	close(decider.gate)
	second.expect(TypeSessionSync, nil)
	var turn InterviewerTurn
	second.expect(TypeInterviewerTurn, &turn)
	if turn.Kind != "core" || turn.Position != 2 || turn.QuestionNumber != 2 {
		t.Fatalf("turn after reconnect = %+v", turn)
	}
	if decider.count() != 1 {
		t.Fatalf("decider calls = %d", decider.count())
	}
}

func TestReplayedAnswersNeverTriggerASecondFollowUp(t *testing.T) {
	repo := newMemoryRepository("Q1", "Q2")
	decider := &followUpDecider{}
	h := followUpHarness(t, repo, decider, 1)
	c, _, _ := h.connect()
	c.answer(1, "one")

	// The same command id: acknowledged as a duplicate.
	c.send(TypeAnswerText, "answer-one", AnswerText{Position: 1, Text: "one"})
	var ack Ack
	c.expect(TypeAck, &ack)
	// The same position under a new id: a duplicate answer, nothing decided or delivered.
	c.send(TypeAnswerText, "answer-one-again", AnswerText{Position: 1, Text: "one"})
	var accepted AnswerAccepted
	c.expect(TypeAnswerAccepted, &accepted)
	if !accepted.Duplicate || accepted.AnsweredCount != 1 {
		t.Fatalf("replayed answer = %+v", accepted)
	}
	c.send(TypePing, "", nil)
	c.expect(TypePong, nil)

	// The follow-up's own answer replayed after it was stored.
	turn, _ := c.answer(2, "follow")
	if turn.Kind != "core" || turn.QuestionNumber != 2 {
		t.Fatalf("turn = %+v", turn)
	}
	c.send(TypeAnswerText, "answer-follow-again", AnswerText{Position: 2, Text: "follow"})
	c.expect(TypeAnswerAccepted, &accepted)
	if !accepted.Duplicate {
		t.Fatalf("replayed follow-up answer = %+v", accepted)
	}
	c.send(TypePing, "", nil)
	c.expect(TypePong, nil)
	if decider.count() != 1 || len(repo.storedTurns()) != 2 {
		t.Fatalf("decider calls = %d, stored turns = %d", decider.count(), len(repo.storedTurns()))
	}
}
