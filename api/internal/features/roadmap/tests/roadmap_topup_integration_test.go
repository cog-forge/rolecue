//go:build integration

package tests

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	interviewdomain "github.com/cog-forge/rolecue/api/internal/features/interview/domain"
	interviewrepository "github.com/cog-forge/rolecue/api/internal/features/interview/repository"
	interviewservice "github.com/cog-forge/rolecue/api/internal/features/interview/service"
	"github.com/cog-forge/rolecue/api/internal/features/roadmap/domain"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

// topUpEnv is a Confirm environment whose profile is confirmed (4 topics of 5 questions each), a wallet, and the
// real interview service with the real top-up wired in, as the application builds it.
type topUpEnv struct {
	confirmEnv
	owner   uuid.UUID
	profile uuid.UUID
	jd      uuid.UUID
	topics  []uuid.UUID
	starter *interviewservice.Service
}

func newTopUpEnv(t *testing.T) topUpEnv {
	t.Helper()
	topics := &topicsFixture{replies: []domain.TopicCandidate{goodTopics()}}
	c := newConfirmEnv(t, topics, 1)
	owner := c.user(t, "candidate")
	profile := c.reviewed(t, owner)
	if _, err := c.app.Confirm(c.ctx, owner, "candidate", profile.ID); err != nil {
		t.Fatal(err)
	}
	c.exec(t, `INSERT INTO wallets(user_id,balance) VALUES($1,50) ON CONFLICT (user_id) DO UPDATE SET balance=50`, owner)
	repo := interviewrepository.NewRepository(c.pool)
	starter, err := interviewservice.New(repo, interviewservice.Settings{QuestionLimit: 8})
	if err != nil {
		t.Fatal(err)
	}
	starter.SetTopicTopUp(repo, c.generation(t, topics, 1))
	return topUpEnv{confirmEnv: c, owner: owner, profile: profile.ID, jd: profile.JDID, topics: c.itemIDs(t, owner, profile.ID), starter: starter}
}

func (e topUpEnv) topicQuestions(t *testing.T, topic uuid.UUID) int {
	t.Helper()
	return e.count(t, `SELECT count(*) FROM core_questions WHERE jd_id=$1 AND roadmap_item_id=$2`, e.jd, topic)
}

func (e topUpEnv) start(topic uuid.UUID) (interviewdomain.Session, bool, error) {
	return e.starter.Create(e.ctx, e.owner, "candidate", interviewdomain.CreateInput{Type: interviewdomain.TypePractice, RoleProfileID: &e.profile, FocusItemID: &topic})
}

func TestAFocusedStartOfATopicWithFiveQuestionsTopsItUpOnceThenStarts(t *testing.T) {
	e := newTopUpEnv(t)
	before := e.bank.calls.Load()
	var otherBefore []int
	for _, topic := range e.topics {
		otherBefore = append(otherBefore, e.topicQuestions(t, topic))
	}
	var oldIDs []uuid.UUID
	rows, err := e.pool.Query(e.ctx, `SELECT id FROM core_questions WHERE jd_id=$1 ORDER BY id`, e.jd)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var id uuid.UUID
		if err = rows.Scan(&id); err != nil {
			t.Fatal(err)
		}
		oldIDs = append(oldIDs, id)
	}
	rows.Close()

	session, created, err := e.start(e.topics[0])
	if err != nil || !created {
		t.Fatalf("start: created=%v err=%v", created, err)
	}
	if e.bank.calls.Load() != before+1 {
		t.Fatalf("model calls for the start = %d, want exactly the one top-up", e.bank.calls.Load()-before)
	}
	if n := e.topicQuestions(t, e.topics[0]); n != 10 {
		t.Fatalf("the topic has %d questions, want 5 + a batch of 5", n)
	}
	for i := 1; i < len(e.topics); i++ {
		if e.topicQuestions(t, e.topics[i]) != otherBefore[i] {
			t.Fatalf("topic %d changed: the top-up must touch only the focused topic", i)
		}
	}
	// Every question that existed is still there with its topic: answered history keeps its meaning.
	if n := e.count(t, `SELECT count(*) FROM core_questions WHERE id = ANY($1) AND roadmap_item_id IS NOT NULL`, oldIDs); n != len(oldIDs) {
		t.Fatalf("%d of %d stored questions lost their mapping or disappeared", len(oldIDs)-n, len(oldIDs))
	}
	if n := e.count(t, `SELECT count(*) FROM interview_questions iq JOIN core_questions cq ON cq.id=iq.core_question_id WHERE iq.interview_id=$1 AND cq.roadmap_item_id=$2`, session.ID, e.topics[0]); n != 8 {
		t.Fatalf("%d of 8 session questions are of the topic", n)
	}
	var balance int
	if err = e.pool.QueryRow(e.ctx, `SELECT balance FROM wallets WHERE user_id=$1`, e.owner).Scan(&balance); err != nil || balance != 45 {
		t.Fatalf("balance = %d, want one charge of 5 (err=%v)", balance, err)
	}

	// Finish it and practise the topic again: it now has enough questions, so no model is called.
	e.exec(t, `UPDATE interviews SET completed_at=now() WHERE id=$1`, session.ID)
	again := e.bank.calls.Load()
	if _, created, err = e.start(e.topics[0]); err != nil || !created || e.bank.calls.Load() != again {
		t.Fatalf("second start: created=%v err=%v model calls=%d", created, err, e.bank.calls.Load()-again)
	}
}

func TestAFailedTopUpLeavesNoSessionNoDebitAndTheBankAsItWas(t *testing.T) {
	e := newTopUpEnv(t)
	down := errors.New("model down")
	e.bank.failWith.Store(&down)
	total := e.count(t, `SELECT count(*) FROM core_questions WHERE jd_id=$1`, e.jd)

	if _, _, err := e.start(e.topics[0]); !apperror.IsCode(err, apperror.CodeQuestionGenerationFailed) {
		t.Fatalf("start = %v, want QUESTION_GENERATION_FAILED", err)
	}
	var balance int
	if err := e.pool.QueryRow(e.ctx, `SELECT balance FROM wallets WHERE user_id=$1`, e.owner).Scan(&balance); err != nil || balance != 50 {
		t.Fatalf("balance = %d, want untouched 50", balance)
	}
	if n := e.count(t, `SELECT count(*) FROM interviews WHERE role_profile_id=$1`, e.profile); n != 0 {
		t.Fatalf("%d sessions after a failed top-up", n)
	}
	if n := e.count(t, `SELECT count(*) FROM core_questions WHERE jd_id=$1`, e.jd); n != total {
		t.Fatalf("bank size %d -> %d after a failed top-up", total, n)
	}
	if n := e.count(t, `SELECT count(*) FROM transactions WHERE "from"=$1`, e.owner.String()); n != 0 {
		t.Fatalf("%d coin rows after a failed top-up", n)
	}
	// The model comes back: the same request now works.
	e.bank.failWith.Store(nil)
	if _, created, err := e.start(e.topics[0]); err != nil || !created {
		t.Fatalf("retry: created=%v err=%v", created, err)
	}
}

func TestResumingAFocusedSessionNeverCallsTheModel(t *testing.T) {
	e := newTopUpEnv(t)
	first, _, err := e.start(e.topics[1])
	if err != nil {
		t.Fatal(err)
	}
	calls := e.bank.calls.Load()
	// Another topic of the same profile that also lacks questions: the unfinished session wins, no generation.
	resumed, created, err := e.start(e.topics[2])
	if err != nil || created || resumed.ID != first.ID || resumed.FocusItemID == nil || *resumed.FocusItemID != e.topics[1] {
		t.Fatalf("resume: id=%v created=%v focus=%v err=%v", resumed.ID, created, resumed.FocusItemID, err)
	}
	if e.bank.calls.Load() != calls || e.topicQuestions(t, e.topics[2]) != 5 {
		t.Fatal("a resumed start called the model or changed another topic")
	}
}

func TestConcurrentFocusedStartsOfAShortTopicAddOneBatchAndOneSession(t *testing.T) {
	e := newTopUpEnv(t)
	var created, resumed atomic.Int32
	var wg sync.WaitGroup
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, wasCreated, err := e.start(e.topics[0])
			if err != nil {
				t.Error(err)
				return
			}
			if wasCreated {
				created.Add(1)
			} else {
				resumed.Add(1)
			}
		}()
	}
	wg.Wait()
	if created.Load() != 1 || resumed.Load() != 3 {
		t.Fatalf("created=%d resumed=%d, want 1 and 3", created.Load(), resumed.Load())
	}
	// Concurrent requests may each draft, but only the first one's questions are added.
	if n := e.topicQuestions(t, e.topics[0]); n != 10 {
		t.Fatalf("the topic has %d questions, want exactly one top-up batch (10)", n)
	}
	var balance int
	if err := e.pool.QueryRow(e.ctx, `SELECT balance FROM wallets WHERE user_id=$1`, e.owner).Scan(&balance); err != nil || balance != 45 {
		t.Fatalf("balance = %d, want 45", balance)
	}
}

func TestATopUpAgainstARegeneratedRoadmapIsAConflictNotADanglingMapping(t *testing.T) {
	e := newTopUpEnv(t)
	oldTopic := e.topics[0]
	// The regeneration lands between the top-up's draft and its add: simulate by regenerating first and then
	// adding for the topic that no longer exists.
	topics := &topicsFixture{replies: []domain.TopicCandidate{differentTopics()}}
	if _, err := e.generation(t, topics, 1).Regenerate(e.ctx, e.owner, "candidate", e.profile); err != nil {
		t.Fatal(err)
	}
	if err := e.generation(t, topics, 1).TopUp(e.ctx, e.owner, e.profile, oldTopic, 5, 8); err == nil {
		t.Fatal("a top-up of a replaced topic was accepted")
	}
	if n := e.count(t, `SELECT count(*) FROM core_questions WHERE jd_id=$1 AND roadmap_item_id=$2`, e.jd, oldTopic); n != 0 {
		t.Fatal("questions were mapped to a topic that no longer exists")
	}
}
