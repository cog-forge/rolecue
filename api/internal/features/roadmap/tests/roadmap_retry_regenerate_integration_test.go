//go:build integration

package tests

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	interviewdomain "github.com/cog-forge/rolecue/api/internal/features/interview/domain"
	interviewrepository "github.com/cog-forge/rolecue/api/internal/features/interview/repository"
	interviewservice "github.com/cog-forge/rolecue/api/internal/features/interview/service"
	questionbankdomain "github.com/cog-forge/rolecue/api/internal/features/questionbank/domain"
	questionbankrepository "github.com/cog-forge/rolecue/api/internal/features/questionbank/repository"
	questionbankservice "github.com/cog-forge/rolecue/api/internal/features/questionbank/service"
	"github.com/cog-forge/rolecue/api/internal/features/roadmap/domain"
	roadmapservice "github.com/cog-forge/rolecue/api/internal/features/roadmap/service"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// generation builds the Roadmap generation service of a Confirm environment, for Retry and Regenerate.
func (c confirmEnv) generation(t *testing.T, topics *topicsFixture, retries int) *roadmapservice.Service {
	t.Helper()
	bankRepo := questionbankrepository.NewRepository(c.pool)
	bank, err := questionbankservice.New(bankRepo, c.bank, 0, 5)
	if err != nil {
		t.Fatal(err)
	}
	generation, err := roadmapservice.New(c.roadmaps, topics, bankRepo, bank, bankRepo, retries)
	if err != nil {
		t.Fatal(err)
	}
	return generation
}

// legacyConfirmed leaves the profile confirmed with its Roadmap still pending: a profile confirmed before the
// Roadmap existed.
func (c confirmEnv) legacyConfirmed(t *testing.T, profile uuid.UUID) {
	t.Helper()
	c.exec(t, `UPDATE role_profiles SET status='confirmed' WHERE id=$1`, profile)
}

func (c confirmEnv) itemIDs(t *testing.T, owner, profile uuid.UUID) []uuid.UUID {
	t.Helper()
	items, err := c.roadmaps.ListItems(c.ctx, owner, profile)
	if err != nil {
		t.Fatal(err)
	}
	ids := make([]uuid.UUID, len(items))
	for i, item := range items {
		ids[i] = item.ID
	}
	return ids
}

func differentTopics() domain.TopicCandidate {
	return domain.TopicCandidate{Topics: []domain.GeneratedTopic{{Title: "Kafka"}, {Title: "Kubernetes"}, {Title: "Observability"}, {Title: "Security"}, {Title: "Testing"}}}
}

func TestConcurrentRetriesRunOneGeneration(t *testing.T) {
	topics := &topicsFixture{replies: []domain.TopicCandidate{goodTopics()}, delay: 300 * time.Millisecond}
	c := newConfirmEnv(t, topics, 1)
	owner := c.user(t, "candidate")
	profile := c.reviewed(t, owner)
	c.legacyConfirmed(t, profile.ID)
	generation := c.generation(t, topics, 1)

	var ok, conflict atomic.Int32
	var wg sync.WaitGroup
	for range 6 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			roadmap, err := generation.Retry(c.ctx, owner, "candidate", profile.ID)
			switch {
			case err == nil && roadmap.GenerationStatus == domain.StatusReady:
				ok.Add(1)
			case apperror.IsCode(err, apperror.CodeRoadmapGenerating):
				conflict.Add(1)
			default:
				t.Errorf("retry = %+v, %v", roadmap, err)
			}
		}()
	}
	wg.Wait()
	if topics.calls.Load() != 1 || c.bank.calls.Load() != 1 {
		t.Fatalf("model calls topics=%d bank=%d, want one generation for six concurrent retries", topics.calls.Load(), c.bank.calls.Load())
	}
	if ok.Load() < 1 || ok.Load()+conflict.Load() != 6 {
		t.Fatalf("ok=%d conflict=%d", ok.Load(), conflict.Load())
	}
	// A late retry of the finished roadmap is a no-op, not a second generation and not an error.
	if roadmap, err := generation.Retry(c.ctx, owner, "candidate", profile.ID); err != nil || roadmap.GenerationStatus != domain.StatusReady || topics.calls.Load() != 1 {
		t.Fatalf("late retry = %+v, %v, topic calls %d", roadmap, err, topics.calls.Load())
	}
	if n := c.count(t, `SELECT count(*) FROM roadmap_items WHERE roadmap_id=$1`, c.roadmap(t, owner, profile.ID).ID); n != 4 {
		t.Fatalf("topics = %d, want 4", n)
	}
}

func TestRetryNeverResetsAReadyRoadmapWithSessions(t *testing.T) {
	topics := &topicsFixture{replies: []domain.TopicCandidate{goodTopics()}}
	c := newConfirmEnv(t, topics, 1)
	owner := c.user(t, "candidate")
	profile := c.reviewed(t, owner)
	if _, err := c.app.Confirm(c.ctx, owner, "candidate", profile.ID); err != nil {
		t.Fatal(err)
	}
	before := c.itemIDs(t, owner, profile.ID)
	var question uuid.UUID
	if err := c.pool.QueryRow(c.ctx, `SELECT id FROM core_questions WHERE jd_id=$1 LIMIT 1`, profile.JDID).Scan(&question); err != nil {
		t.Fatal(err)
	}
	c.interview(t, profile.ID, question, uuid.Nil)

	generation := c.generation(t, topics, 1)
	roadmap, err := generation.Retry(c.ctx, owner, "candidate", profile.ID)
	if err != nil || roadmap.GenerationStatus != domain.StatusReady {
		t.Fatalf("retry = %+v, %v", roadmap, err)
	}
	after := c.itemIDs(t, owner, profile.ID)
	if len(after) != len(before) || after[0] != before[0] || topics.calls.Load() != 1 {
		t.Fatal("a retry reset a ready roadmap that already has sessions")
	}
}

func TestRetryOfALegacyProfileWithSessionsIsNotRetryable(t *testing.T) {
	topics := &topicsFixture{replies: []domain.TopicCandidate{goodTopics()}}
	c := newConfirmEnv(t, topics, 1)
	owner := c.user(t, "candidate")
	profile := c.reviewed(t, owner)
	c.legacyConfirmed(t, profile.ID)
	c.exec(t, `INSERT INTO core_questions(jd_id,content) VALUES($1,'Old question?')`, profile.JDID)
	var question uuid.UUID
	if err := c.pool.QueryRow(c.ctx, `SELECT id FROM core_questions WHERE jd_id=$1`, profile.JDID).Scan(&question); err != nil {
		t.Fatal(err)
	}
	c.interview(t, profile.ID, question, uuid.Nil)
	if _, err := c.generation(t, topics, 1).Retry(c.ctx, owner, "candidate", profile.ID); !apperror.IsCode(err, apperror.CodeRoadmapNotRetryable) {
		t.Fatalf("retry = %v, want ROADMAP_NOT_RETRYABLE", err)
	}
	if topics.calls.Load() != 0 {
		t.Fatal("a refused retry called the model")
	}
}

func TestRegenerateKeepsTheRoadmapIDAndReplacesTopicsAndBankTogether(t *testing.T) {
	topics := &topicsFixture{replies: []domain.TopicCandidate{goodTopics(), differentTopics()}}
	c := newConfirmEnv(t, topics, 1)
	owner := c.user(t, "candidate")
	profile := c.reviewed(t, owner)
	if _, err := c.app.Confirm(c.ctx, owner, "candidate", profile.ID); err != nil {
		t.Fatal(err)
	}
	roadmap := c.roadmap(t, owner, profile.ID)
	oldItems := c.itemIDs(t, owner, profile.ID)
	oldQuestions := c.count(t, `SELECT count(*) FROM core_questions WHERE roadmap_item_id = ANY($1)`, oldItems)

	regenerated, err := c.generation(t, topics, 1).Regenerate(c.ctx, owner, "candidate", profile.ID)
	if err != nil || regenerated.ID != roadmap.ID || regenerated.GenerationStatus != domain.StatusReady {
		t.Fatalf("regenerate = %+v, %v", regenerated, err)
	}
	newItems := c.itemIDs(t, owner, profile.ID)
	if len(newItems) != 5 {
		t.Fatalf("topics = %d, want the 5 new ones", len(newItems))
	}
	for _, id := range newItems {
		for _, old := range oldItems {
			if id == old {
				t.Fatal("a topic id survived a regeneration: the set was not replaced")
			}
		}
	}
	if n := c.count(t, `SELECT count(*) FROM core_questions WHERE roadmap_item_id = ANY($1)`, oldItems); n != 0 || oldQuestions == 0 {
		t.Fatalf("%d questions still point at replaced topics", n)
	}
	// One current bank: 25 questions (5 topics x 5), all mapped to the new topics.
	if n := c.count(t, `SELECT count(*) FROM core_questions WHERE jd_id=$1`, profile.JDID); n != 25 {
		t.Fatalf("bank = %d questions, want exactly one current bank of 25", n)
	}
	if n := c.count(t, `SELECT count(*) FROM core_questions WHERE jd_id=$1 AND roadmap_item_id = ANY($2)`, profile.JDID, newItems); n != 25 {
		t.Fatalf("%d of 25 bank questions are mapped to the new topics", n)
	}
}

func TestRegenerateIsRefusedAfterAnyKindOfSession(t *testing.T) {
	for name, update := range map[string]string{
		"unfinished":             ``,
		"finished without score": `UPDATE interviews SET completed_at=now(), end_reason='completed' WHERE id=$1`,
		"abandoned":              `UPDATE interviews SET completed_at=now(), end_reason='abandoned' WHERE id=$1`,
	} {
		t.Run(name, func(t *testing.T) {
			topics := &topicsFixture{replies: []domain.TopicCandidate{goodTopics(), differentTopics()}}
			c := newConfirmEnv(t, topics, 1)
			owner := c.user(t, "candidate")
			profile := c.reviewed(t, owner)
			if _, err := c.app.Confirm(c.ctx, owner, "candidate", profile.ID); err != nil {
				t.Fatal(err)
			}
			items := c.itemIDs(t, owner, profile.ID)
			var question uuid.UUID
			if err := c.pool.QueryRow(c.ctx, `SELECT id FROM core_questions WHERE jd_id=$1 LIMIT 1`, profile.JDID).Scan(&question); err != nil {
				t.Fatal(err)
			}
			interview := c.interview(t, profile.ID, question, uuid.Nil)
			if update != "" {
				c.exec(t, update, interview)
			}
			if _, err := c.generation(t, topics, 1).Regenerate(c.ctx, owner, "candidate", profile.ID); !apperror.IsCode(err, apperror.CodeRoadmapRegenerateNotAllowed) {
				t.Fatalf("regenerate = %v, want ROADMAP_REGENERATE_NOT_ALLOWED", err)
			}
			if topics.calls.Load() != 1 {
				t.Fatal("a refused regeneration called the model")
			}
			after := c.itemIDs(t, owner, profile.ID)
			if len(after) != len(items) || after[0] != items[0] {
				t.Fatal("topics changed under a session")
			}
		})
	}
}

func TestAFailedRegenerationLeavesTheOldSetAndTheRoadmapReady(t *testing.T) {
	unusable := domain.TopicCandidate{Topics: []domain.GeneratedTopic{{Title: "Only one"}}}
	topics := &topicsFixture{replies: []domain.TopicCandidate{goodTopics(), unusable}}
	c := newConfirmEnv(t, topics, 0)
	owner := c.user(t, "candidate")
	profile := c.reviewed(t, owner)
	if _, err := c.app.Confirm(c.ctx, owner, "candidate", profile.ID); err != nil {
		t.Fatal(err)
	}
	oldItems := c.itemIDs(t, owner, profile.ID)
	if _, err := c.generation(t, topics, 0).Regenerate(c.ctx, owner, "candidate", profile.ID); !apperror.IsCode(err, apperror.CodeInvalidRoadmapGenerationOutput) {
		t.Fatalf("regenerate = %v, want INVALID_ROADMAP_GENERATION_OUTPUT", err)
	}
	if got := c.roadmap(t, owner, profile.ID).GenerationStatus; got != domain.StatusReady {
		t.Fatalf("status = %s, want ready: the old set is intact", got)
	}
	after := c.itemIDs(t, owner, profile.ID)
	if len(after) != 4 || after[0] != oldItems[0] {
		t.Fatalf("old topics were not kept: %v", after)
	}
	if n := c.count(t, `SELECT count(*) FROM core_questions WHERE jd_id=$1 AND roadmap_item_id = ANY($2)`, profile.JDID, oldItems); n != 20 {
		t.Fatalf("old bank has %d questions, want 20", n)
	}
}

// A real practice start and a regeneration of the same profile run at the same time, round after round.
func TestRegenerateAndStartExcludeEachOther(t *testing.T) {
	var startedFirst, regeneratedFirst int
	for round := 0; round < 12; round++ {
		topics := &topicsFixture{replies: []domain.TopicCandidate{goodTopics(), differentTopics()}}
		c := newConfirmEnv(t, topics, 1)
		owner := c.user(t, "candidate")
		profile := c.reviewed(t, owner)
		if _, err := c.app.Confirm(c.ctx, owner, "candidate", profile.ID); err != nil {
			t.Fatal(err)
		}
		c.exec(t, `INSERT INTO wallets(user_id,balance) VALUES($1,20) ON CONFLICT (user_id) DO UPDATE SET balance=20`, owner)
		starter, err := interviewservice.New(interviewrepository.NewRepository(c.pool), interviewservice.Settings{QuestionLimit: 3})
		if err != nil {
			t.Fatal(err)
		}
		generation := c.generation(t, topics, 1)
		// The start usually wins this race (a regeneration spends two model calls before it publishes); the
		// opposite order is held open on purpose by the next test.

		var startErr, regenErr error
		var wg sync.WaitGroup
		wg.Add(2)
		release := make(chan struct{})
		go func() {
			defer wg.Done()
			<-release
			_, _, startErr = starter.Create(c.ctx, owner, "candidate", interviewdomain.CreateInput{Type: interviewdomain.TypePractice, RoleProfileID: &profile.ID})
		}()
		go func() {
			defer wg.Done()
			<-release
			_, regenErr = generation.Regenerate(c.ctx, owner, "candidate", profile.ID)
		}()
		close(release)
		wg.Wait()

		if startErr != nil {
			t.Fatalf("round %d: start = %v", round, startErr)
		}
		// Whatever the order, no session may point at a question that is not in the current bank, nor at a
		// topic that is not a current topic of the roadmap.
		current := c.itemIDs(t, owner, profile.ID)
		var dangling int
		err = c.pool.QueryRow(c.ctx, `SELECT count(*) FROM interview_questions iq JOIN interviews i ON i.id=iq.interview_id
 LEFT JOIN core_questions cq ON cq.id=iq.core_question_id AND cq.jd_id=$2 AND cq.roadmap_item_id = ANY($3)
 WHERE i.role_profile_id=$1 AND cq.id IS NULL`, profile.ID, profile.JDID, current).Scan(&dangling)
		if err != nil || dangling != 0 {
			t.Fatalf("round %d: %d session questions reference a replaced question or topic (err=%v)", round, dangling, err)
		}
		if n := c.count(t, `SELECT count(*) FROM interviews WHERE role_profile_id=$1`, profile.ID); n != 1 {
			t.Fatalf("round %d: %d sessions, want exactly one", round, n)
		}
		switch {
		case regenErr == nil:
			regeneratedFirst++ // the regeneration committed before the start sampled: the session uses the new bank
			if len(current) != 5 {
				t.Fatalf("round %d: regenerated but the roadmap has %d topics", round, len(current))
			}
		case apperror.IsCode(regenErr, apperror.CodeRoadmapRegenerateNotAllowed):
			startedFirst++ // the start won: the old set is kept for the session
			if len(current) != 4 {
				t.Fatalf("round %d: refused regeneration but the topics changed (%d)", round, len(current))
			}
		default:
			t.Fatalf("round %d: regenerate = %v, want success or ROADMAP_REGENERATE_NOT_ALLOWED", round, regenErr)
		}
		if got := c.roadmap(t, owner, profile.ID).GenerationStatus; got != domain.StatusReady {
			t.Fatalf("round %d: status = %s, want ready", round, got)
		}
	}
	t.Logf("12 rounds: start won %d, regeneration won %d", startedFirst, regeneratedFirst)
}

// pausingStore holds the publish transaction open, after the bank is replaced and before the commit, so a start
// that arrives meanwhile must queue on the profile's practice start lock.
type pausingStore struct {
	inner    *questionbankrepository.Repository
	inTx     chan struct{}
	holdFor  time.Duration
	released atomic.Bool
}

func (p *pausingStore) ReplacePracticeBank(ctx context.Context, tx pgx.Tx, jdID uuid.UUID, questions []questionbankdomain.GeneratedQuestion) error {
	if err := p.inner.ReplacePracticeBank(ctx, tx, jdID, questions); err != nil {
		return err
	}
	if p.released.CompareAndSwap(false, true) {
		close(p.inTx)
	}
	time.Sleep(p.holdFor)
	return nil
}

func (p *pausingStore) AppendTopicQuestions(ctx context.Context, tx pgx.Tx, jdID, topicID uuid.UUID, total int, questions []questionbankdomain.GeneratedQuestion) (bool, error) {
	return p.inner.AppendTopicQuestions(ctx, tx, jdID, topicID, total, questions)
}

func TestAStartThatArrivesDuringARegenerationWaitsAndSamplesTheNewBank(t *testing.T) {
	topics := &topicsFixture{replies: []domain.TopicCandidate{goodTopics(), differentTopics()}}
	c := newConfirmEnv(t, topics, 1)
	owner := c.user(t, "candidate")
	profile := c.reviewed(t, owner)
	if _, err := c.app.Confirm(c.ctx, owner, "candidate", profile.ID); err != nil {
		t.Fatal(err)
	}
	c.exec(t, `INSERT INTO wallets(user_id,balance) VALUES($1,20) ON CONFLICT (user_id) DO UPDATE SET balance=20`, owner)
	starter, err := interviewservice.New(interviewrepository.NewRepository(c.pool), interviewservice.Settings{QuestionLimit: 3})
	if err != nil {
		t.Fatal(err)
	}
	bankRepo := questionbankrepository.NewRepository(c.pool)
	bank, err := questionbankservice.New(bankRepo, c.bank, 0, 5)
	if err != nil {
		t.Fatal(err)
	}
	pausing := &pausingStore{inner: bankRepo, inTx: make(chan struct{}), holdFor: 400 * time.Millisecond}
	generation, err := roadmapservice.New(c.roadmaps, topics, bankRepo, bank, pausing, 1)
	if err != nil {
		t.Fatal(err)
	}

	regenerated := make(chan error, 1)
	go func() {
		_, err := generation.Regenerate(c.ctx, owner, "candidate", profile.ID)
		regenerated <- err
	}()
	<-pausing.inTx // the new bank is written and uncommitted, the lock is held
	begin := time.Now()
	session, _, err := starter.Create(c.ctx, owner, "candidate", interviewdomain.CreateInput{Type: interviewdomain.TypePractice, RoleProfileID: &profile.ID})
	waited := time.Since(begin)
	if err != nil {
		t.Fatal("start", err)
	}
	if err = <-regenerated; err != nil {
		t.Fatal("regenerate", err)
	}
	if waited < 200*time.Millisecond {
		t.Fatalf("the start took %v: it did not wait for the regeneration's transaction", waited)
	}
	current := c.itemIDs(t, owner, profile.ID)
	if len(current) != 5 {
		t.Fatalf("topics = %d, want the regenerated 5", len(current))
	}
	var fromNewBank int
	if err = c.pool.QueryRow(c.ctx, `SELECT count(*) FROM interview_questions iq JOIN core_questions cq ON cq.id=iq.core_question_id
 WHERE iq.interview_id=$1 AND cq.jd_id=$2 AND cq.roadmap_item_id = ANY($3)`, session.ID, profile.JDID, current).Scan(&fromNewBank); err != nil || fromNewBank != 3 {
		t.Fatalf("%d of 3 session questions come from the new bank (err=%v)", fromNewBank, err)
	}
}
