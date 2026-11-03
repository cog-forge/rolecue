//go:build integration

package tests

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	evalrepository "github.com/cog-forge/rolecue/api/internal/features/evaluation/repository"
	"github.com/cog-forge/rolecue/api/internal/features/interview/domain"
	"github.com/cog-forge/rolecue/api/internal/features/interview/repository"
	"github.com/cog-forge/rolecue/api/internal/features/interview/runtime"
	"github.com/cog-forge/rolecue/api/internal/features/interview/service"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

// recordingDecider asks a follow-up for every slot, or fails when err is set, and keeps what it saw.
type recordingDecider struct {
	mu    sync.Mutex
	err   error
	calls []domain.FollowUpContext
}

func (d *recordingDecider) Decide(_ context.Context, in domain.FollowUpContext) (domain.FollowUpProposal, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.calls = append(d.calls, in)
	if d.err != nil {
		return domain.FollowUpProposal{}, d.err
	}
	return domain.FollowUpProposal{
		FollowUp: true, Question: "What would you change about answer " + string(rune('0'+len(d.calls))) + " under load?", Reason: "worth probing",
	}, nil
}

func (d *recordingDecider) seen() []domain.FollowUpContext {
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]domain.FollowUpContext(nil), d.calls...)
}

// requirement adds a requirement to the Job Description of a role profile.
func (s *seed) requirement(profileID uuid.UUID, name string) {
	s.t.Helper()
	s.exec(`INSERT INTO public.requirements (name, target, jd_id, type)
		SELECT $2, 'Proficient', jd_id, 'required' FROM public.role_profiles WHERE id = $1`, profileID, name)
}

// questionNumbers reads the derived question number of each stored turn, in position order.
func (s *seed) questionNumbers(interviewID uuid.UUID) []int32 {
	s.t.Helper()
	rows, err := repository.New(s.pool).ListInterviewTurns(s.ctx, interviewID)
	if err != nil {
		s.t.Fatal(err)
	}
	out := make([]int32, 0, len(rows))
	for _, row := range rows {
		out = append(out, row.QuestionNumber)
	}
	return out
}

// TestLiveFollowUpsEndToEnd runs a practice interview with one follow-up per core question through the
// real handler, engine, service and database. A follow-up is stored only with its answer; when the socket
// drops during one, the reconnect asks the next core question and nothing is decided again. The report
// and the evidence read follow-ups with their core question. The Candidate never receives an unsampled
// bank question, a core question id or the decider's reason.
func TestLiveFollowUpsEndToEnd(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	stack := newLiveStack(t, pool, service.Settings{QuestionLimit: 3, FollowUpLimit: 1, DefaultProviderVoiceID: "en-US-JennyNeural"})
	decider := &recordingDecider{}
	stack.svc.SetFollowUpDecider(decider)
	candidate := s.user("candidate")
	s.wallet(candidate, 10)
	profile, bank := s.confirmedProfile(candidate, 6)
	s.requirement(profile, "Go")
	started, _, err := stack.svc.Create(ctx, candidate, "candidate", domain.CreateInput{Type: domain.TypePractice, RoleProfileID: &profile})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	sampled := map[uuid.UUID]bool{}
	for _, id := range s.snapshot(started.ID) {
		sampled[id] = true
	}

	c, _, err := stack.dial(t, candidate, started.ID)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	c.expect(runtime.TypeSessionSync, nil)
	var turn runtime.InterviewerTurn
	c.expect(runtime.TypeInterviewerTurn, &turn)
	coreText := turn.Text
	c.answer(1, "answer-1", false)
	c.expectState(runtime.StateInterviewerSpeaking)
	c.expect(runtime.TypeInterviewerTurn, &turn)
	if turn.Position != 2 || turn.QuestionNumber != 1 || turn.Kind != "follow_up" || turn.QuestionCount != 3 || turn.AudioStatus != runtime.AudioReady {
		t.Fatalf("follow-up turn = %d/%d %s count %d audio %s", turn.Position, turn.QuestionNumber, turn.Kind, turn.QuestionCount, turn.AudioStatus)
	}
	seen := decider.seen()
	if len(seen) != 1 || seen[0].QuestionNumber != 1 || seen[0].Ordinal != 1 || seen[0].Position != 2 || seen[0].CoreQuestion != coreText ||
		len(seen[0].Exchanges) != 1 || seen[0].Exchanges[0].Answer != "typed answer answer-1" ||
		len(seen[0].Requirements) != 1 || seen[0].Requirements[0].Name != "Go" {
		t.Fatalf("decider saw %+v", seen)
	}
	if turns := s.liveTurns(started.ID); len(turns) != 1 {
		t.Fatalf("an unanswered follow-up was stored: %+v", turns)
	}

	// The socket drops during the follow-up: the reconnect asks core question 2 at the same position.
	_ = c.ws.NetConn().Close()
	c, _, err = stack.dial(t, candidate, started.ID)
	if err != nil {
		t.Fatalf("redial: %v", err)
	}
	var sync runtime.SessionSync
	c.expect(runtime.TypeSessionSync, &sync)
	c.expect(runtime.TypeInterviewerTurn, &turn)
	if sync.Position == nil || *sync.Position != 2 || sync.QuestionNumber == nil || *sync.QuestionNumber != 2 || sync.AnsweredCount != 1 ||
		turn.Kind != "core" || turn.Position != 2 || turn.QuestionNumber != 2 {
		t.Fatalf("after reconnect sync = %+v, turn = %d/%d %s", sync, turn.Position, turn.QuestionNumber, turn.Kind)
	}
	if len(decider.seen()) != 1 {
		t.Fatalf("decided %d times", len(decider.seen()))
	}

	c.answer(2, "answer-2", false)
	c.expectState(runtime.StateInterviewerSpeaking)
	c.expect(runtime.TypeInterviewerTurn, &turn)
	if turn.Position != 3 || turn.QuestionNumber != 2 || turn.Kind != "follow_up" {
		t.Fatalf("follow-up of question 2 = %+v", turn)
	}
	followUpText := turn.Text
	c.answer(3, "answer-3", false)
	c.expectState(runtime.StateInterviewerSpeaking)
	c.expect(runtime.TypeInterviewerTurn, &turn)
	if turn.Position != 4 || turn.QuestionNumber != 3 || turn.Kind != "core" {
		t.Fatalf("third core turn = %+v", turn)
	}
	c.answer(4, "answer-4", false)
	c.expectState(runtime.StateInterviewerSpeaking)
	c.expect(runtime.TypeInterviewerTurn, &turn)
	if turn.Position != 5 || turn.QuestionNumber != 3 || turn.Kind != "follow_up" {
		t.Fatalf("last follow-up = %+v", turn)
	}
	c.answer(5, "answer-5", false)
	c.expectState(runtime.StateCompleted)
	c.expect(runtime.TypeSessionCompleted, nil)
	c.expectClose(websocket.CloseNormalClosure)

	// Nothing hidden reached the Candidate.
	for _, frame := range c.frames {
		if bytes.Contains(frame, []byte("worth probing")) || bytes.Contains(frame, []byte("core_question_id")) {
			t.Fatalf("frame leaks internal data: %s", frame)
		}
		for _, id := range bank {
			if !sampled[id] && bytes.Contains(frame, []byte(id.String())) {
				t.Fatalf("frame leaks an unsampled bank question: %s", frame)
			}
		}
	}

	turns := s.liveTurns(started.ID)
	kinds := make([]string, 0, len(turns))
	for _, stored := range turns {
		kinds = append(kinds, stored.kind)
		if !stored.askedAt.Valid || !stored.answeredAt.Valid || stored.answeredAt.Time.Before(stored.askedAt.Time) {
			t.Fatalf("turn %d times asked=%v answered=%v", stored.position, stored.askedAt, stored.answeredAt)
		}
	}
	if strings.Join(kinds, ",") != "core,core,follow_up,core,follow_up" || turns[2].question != followUpText {
		t.Fatalf("stored turns = %+v", turns)
	}
	if got := s.questionNumbers(started.ID); fmt.Sprint(got) != "[1 2 2 3 3]" {
		t.Fatalf("question numbers = %v", got)
	}

	// REST progress counts core questions; the practice turns carry their kind.
	session, err := stack.svc.Get(ctx, candidate, "candidate", started.ID)
	if err != nil || session.QuestionCount != 3 || session.AnsweredCount != 3 || session.Turns == nil || len(*session.Turns) != 5 {
		t.Fatalf("session = %+v, err = %v", session, err)
	}
	if follow := (*session.Turns)[2]; follow.Kind != domain.TurnFollowUp || follow.QuestionNumber != 2 || follow.Position != 3 {
		t.Fatalf("REST follow-up turn = %+v", follow)
	}

	// The report and the Recruiter's evidence read follow-ups with their core question.
	report, err := evalrepository.NewRepository(pool).InterviewTurns(ctx, started.ID)
	if err != nil || len(report) != 5 || report[2].Kind != "follow_up" || report[2].QuestionNumber != 2 || report[4].QuestionNumber != 3 {
		t.Fatalf("report turns = %+v, err = %v", report, err)
	}
	transcript, err := repository.NewRepository(pool).ListTranscript(ctx, started.ID)
	if err != nil || len(transcript) != 5 || transcript[4].Kind != domain.TurnFollowUp || transcript[4].QuestionNumber != 3 {
		t.Fatalf("transcript = %+v, err = %v", transcript, err)
	}
}

// TestFollowUpPersistenceRules drives the repository directly: a follow-up slot opens only after a core
// answer and within the limit, a follow-up is stored with its answer and its own text, a retry changes
// nothing, and Deliver only ever returns core questions.
func TestFollowUpPersistenceRules(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	repo := repository.NewRepository(pool)
	svc := newService(t, pool, 3)
	candidate := s.user("candidate")
	s.wallet(candidate, 10)
	started, _ := practiceSession(t, ctx, s, svc, candidate, 3)
	id := started.ID

	if _, pending, err := repo.FollowUpContext(ctx, candidate, id, "", 1); err != nil || pending != nil {
		t.Fatalf("slot before any answer = %+v, %v", pending, err)
	}
	if _, err := repo.AppendTurn(ctx, candidate, id, 1, "first"); err != nil {
		t.Fatal(err)
	}
	_, pending, err := repo.FollowUpContext(ctx, candidate, id, "", 1)
	if err != nil || pending == nil || pending.Position != 2 || pending.QuestionNumber != 1 || pending.Ordinal != 1 ||
		len(pending.Exchanges) != 1 || pending.Exchanges[0].Answer != "first" {
		t.Fatalf("slot = %+v, %v", pending, err)
	}
	if _, pending, err := repo.FollowUpContext(ctx, candidate, id, "", 0); err != nil || pending != nil {
		t.Fatalf("slot with limit 0 = %+v, %v", pending, err)
	}
	// Deliver never asks a follow-up: an unanswered follow-up does not block core question 2.
	delivery, err := repo.Deliver(ctx, candidate, id, "")
	if err != nil || delivery.Question == nil || delivery.Question.Position != 2 || delivery.Question.QuestionNumber != 2 || delivery.Question.Kind != domain.TurnCore {
		t.Fatalf("Deliver = %+v, %v", delivery.Question, err)
	}

	asked := time.Now().UTC().Truncate(time.Millisecond)
	followUp := domain.AskedQuestion{Position: 2, QuestionNumber: 1, Kind: domain.TurnFollowUp, Text: "Why that index?", AskedAt: asked}
	session, err := repo.AppendFollowUp(ctx, candidate, id, followUp, "because", 1)
	if err != nil || session.AnsweredCount != 1 || session.AnsweredTurns != 2 || session.CurrentQuestion == nil || session.CurrentQuestion.Position != 3 || session.CurrentQuestion.QuestionNumber != 2 {
		t.Fatalf("AppendFollowUp = %+v, %v", session.CurrentQuestion, err)
	}
	turns := s.liveTurns(id)
	if len(turns) != 2 || turns[1].kind != "follow_up" || turns[1].question != "Why that index?" || !turns[1].askedAt.Time.Equal(asked) {
		t.Fatalf("turns = %+v", turns)
	}
	// A retry changes nothing; another answer at the same position, a second follow-up over the limit and
	// a follow-up stored as a core answer are conflicts.
	if _, err := repo.AppendFollowUp(ctx, candidate, id, followUp, "because", 1); err != nil {
		t.Fatalf("retry: %v", err)
	}
	for name, call := range map[string]func() error{
		"other answer": func() error { _, err := repo.AppendFollowUp(ctx, candidate, id, followUp, "other", 1); return err },
		"over the limit": func() error {
			next := followUp
			next.Position = 3
			_, err := repo.AppendFollowUp(ctx, candidate, id, next, "more", 1)
			return err
		},
		"core answer at the follow-up's position": func() error { _, err := repo.AppendTurn(ctx, candidate, id, 2, "because"); return err },
	} {
		if codeOf(call()) != apperror.CodeInterviewTurnConflict {
			t.Fatalf("%s was not a conflict", name)
		}
	}
	if _, pending, err := repo.FollowUpContext(ctx, candidate, id, "", 1); err != nil || pending != nil {
		t.Fatalf("slot after the limit = %+v, %v", pending, err)
	}

	delivery, err = repo.Deliver(ctx, candidate, id, "")
	if err != nil || delivery.Question == nil || delivery.Question.Position != 3 || delivery.Question.QuestionNumber != 2 {
		t.Fatalf("after the follow-up = %+v, %v", delivery.Question, err)
	}
	if _, err := repo.AppendTurn(ctx, candidate, id, 3, "second"); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.AppendTurn(ctx, candidate, id, 4, "third"); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.AppendTurn(ctx, candidate, id, 5, "beyond"); codeOf(err) != apperror.CodeInterviewTurnConflict {
		t.Fatalf("answer after the last question = %v", err)
	}
	if got := s.questionNumbers(id); fmt.Sprint(got) != "[1 1 2 3]" {
		t.Fatalf("question numbers = %v", got)
	}
	delivery, err = repo.Deliver(ctx, candidate, id, "")
	if err != nil || delivery.Question != nil || delivery.AnsweredCount != 4 || delivery.QuestionCount != 3 {
		t.Fatalf("finished delivery = %+v, %v", delivery, err)
	}
}

// A failed decision asks nothing and stores nothing; the next core question follows.
func TestFailedFollowUpDecisionMovesOn(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc, err := service.New(repository.NewRepository(pool), service.Settings{QuestionLimit: 2, FollowUpLimit: 1})
	if err != nil {
		t.Fatal(err)
	}
	svc.SetFollowUpDecider(&recordingDecider{err: errors.New("503 model overloaded")})
	candidate := s.user("candidate")
	s.wallet(candidate, 10)
	started, _ := practiceSession(t, ctx, s, svc, candidate, 2)
	if _, err := svc.AppendTurn(ctx, candidate, "candidate", started.ID, 1, "first"); err != nil {
		t.Fatal(err)
	}
	followUp, err := svc.FollowUp(ctx, candidate, "candidate", started.ID)
	if err != nil || followUp.Question != nil {
		t.Fatalf("FollowUp = %+v, %v", followUp.Question, err)
	}
	delivery, err := svc.Deliver(ctx, candidate, "candidate", started.ID)
	if err != nil || delivery.Question == nil || delivery.Question.Kind != domain.TurnCore || delivery.Question.QuestionNumber != 2 {
		t.Fatalf("Deliver = %+v, %v", delivery.Question, err)
	}
	if turns := s.liveTurns(started.ID); len(turns) != 1 {
		t.Fatalf("turns = %+v", turns)
	}
}
