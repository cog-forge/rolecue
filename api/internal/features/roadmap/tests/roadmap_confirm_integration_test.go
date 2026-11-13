//go:build integration

package tests

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	questionbankdomain "github.com/cog-forge/rolecue/api/internal/features/questionbank/domain"
	questionbankrepository "github.com/cog-forge/rolecue/api/internal/features/questionbank/repository"
	questionbankservice "github.com/cog-forge/rolecue/api/internal/features/questionbank/service"
	"github.com/cog-forge/rolecue/api/internal/features/roadmap/domain"
	roadmapservice "github.com/cog-forge/rolecue/api/internal/features/roadmap/service"
	rpdomain "github.com/cog-forge/rolecue/api/internal/features/roleprofile/domain"
	rpservice "github.com/cog-forge/rolecue/api/internal/features/roleprofile/service"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

type extractionFixture struct{}

func (extractionFixture) Extract(context.Context, string) (rpdomain.ExtractionCandidate, error) {
	level := rpdomain.Senior
	return rpdomain.ExtractionCandidate{Title: "Backend Engineer", SeniorityLevel: &level,
		Skills:       []rpdomain.ExtractedSkill{{Name: "Go", Category: rpdomain.ProgrammingLanguage, Target: "Proficient", Requirement: rpdomain.Required}},
		Technologies: []string{}, DomainKnowledge: []string{}}, nil
}

type bankFixture struct {
	calls atomic.Int32
	// failWith makes every following call fail (the model is down).
	failWith atomic.Pointer[error]
}

// Generate writes each topic exactly its quota of distinct questions, the way a compliant model does.
func (b *bankFixture) Generate(_ context.Context, in questionbankdomain.GenerationInput, count int) (questionbankdomain.GenerationCandidate, error) {
	b.calls.Add(1)
	if err := b.failWith.Load(); err != nil {
		return questionbankdomain.GenerationCandidate{}, *err
	}
	candidate := questionbankdomain.GenerationCandidate{Questions: []questionbankdomain.GeneratedQuestion{}}
	for _, topic := range in.Topics {
		for i := 0; i < topic.Quota; i++ {
			id := topic.ID
			// A top-up gets the stored questions as existing: new ones are numbered after them.
			candidate.Questions = append(candidate.Questions, questionbankdomain.GeneratedQuestion{
				Content: fmt.Sprintf("Question %d about %s?", len(in.Existing)+i, topic.Title), TopicID: &id})
		}
	}
	if len(candidate.Questions) != count {
		return candidate, fmt.Errorf("fixture asked for %d questions but the quotas add up to %d", count, len(candidate.Questions))
	}
	return candidate, nil
}

type topicsFixture struct {
	calls   atomic.Int32
	replies []domain.TopicCandidate
	delay   time.Duration
}

func (f *topicsFixture) GenerateTopics(context.Context, domain.TopicInput) (domain.TopicCandidate, error) {
	n := int(f.calls.Add(1)) - 1
	time.Sleep(f.delay)
	return f.replies[min(n, len(f.replies)-1)], nil
}

func goodTopics() domain.TopicCandidate {
	return domain.TopicCandidate{Topics: []domain.GeneratedTopic{{Title: "SQL", Description: "Joins and indexes"}, {Title: "Go concurrency"},
		{Title: "System design"}, {Title: "Caching"}}}
}

type confirmEnv struct {
	env
	app    *rpservice.RoleProfileApplication
	topics *topicsFixture
	bank   *bankFixture
}

func newConfirmEnv(t *testing.T, topics *topicsFixture, retries int) confirmEnv {
	t.Helper()
	return newConfirmEnvWith(t, topics, &bankFixture{}, 0, retries)
}

// newConfirmEnvWith wires the Confirm flow with a given bank generator and retry budgets.
func newConfirmEnvWith(t *testing.T, topics *topicsFixture, bank questionbankservice.Generator, bankRetries, retries int) confirmEnv {
	t.Helper()
	e := newEnv(t)
	bankRepo := questionbankrepository.NewRepository(e.pool)
	questionBank, err := questionbankservice.New(bankRepo, bank, bankRetries, 5)
	if err != nil {
		t.Fatal(err)
	}
	generation, err := roadmapservice.New(e.roadmaps, topics, bankRepo, questionBank, bankRepo, retries)
	if err != nil {
		t.Fatal(err)
	}
	extraction, err := rpservice.NewExtractionService(extractionFixture{}, 0)
	if err != nil {
		t.Fatal(err)
	}
	app, err := rpservice.NewRoleProfileApplication(extraction, e.profiles, questionBank, 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	fixture, _ := bank.(*bankFixture)
	return confirmEnv{env: e, app: app.WithRoadmap(generation), topics: topics, bank: fixture}
}

// reviewed creates a profile and takes it to the review state, ready to be confirmed.
func (c confirmEnv) reviewed(t *testing.T, owner uuid.UUID) rpdomain.RoleProfile {
	t.Helper()
	raw := strings.Repeat("Senior backend engineer using Go, concurrency, caching and PostgreSQL. ", 4)
	profile, err := c.app.Create(c.ctx, owner, "candidate", raw)
	if err != nil {
		t.Fatal("create", err)
	}
	t.Cleanup(func() { c.removeProfile(profile.ID, owner) })
	if profile, err = c.app.Analyze(c.ctx, owner, "candidate", profile.ID); err != nil || profile.Status != rpdomain.Review {
		t.Fatal("analyze", err)
	}
	return profile
}

func TestConfirmGeneratesTopicsAndTheBankAndMarksTheRoadmapReady(t *testing.T) {
	c := newConfirmEnv(t, &topicsFixture{replies: []domain.TopicCandidate{goodTopics()}}, 1)
	owner := c.user(t, "candidate")
	profile := c.reviewed(t, owner)
	if got := c.roadmap(t, owner, profile.ID).GenerationStatus; got != domain.StatusPending {
		t.Fatalf("before confirm = %s, want pending (nothing is generated from unconfirmed requirements)", got)
	}
	if c.topics.calls.Load() != 0 {
		t.Fatal("topics were generated before the requirements were confirmed")
	}

	confirmed, err := c.app.Confirm(c.ctx, owner, "candidate", profile.ID)
	if err != nil || confirmed.Status != rpdomain.Confirmed {
		t.Fatal("confirm", err)
	}
	if got := c.roadmap(t, owner, profile.ID).GenerationStatus; got != domain.StatusReady {
		t.Fatalf("after confirm = %s, want ready", got)
	}
	items, err := c.roadmaps.ListItems(c.ctx, owner, profile.ID)
	if err != nil || len(items) != 4 || items[0].Title != "SQL" || items[0].Position != 0 || items[3].Position != 3 {
		t.Fatalf("items = %+v (err=%v)", items, err)
	}
	// The configured size (5) cannot hold five questions for each of four topics: the bank is raised to 20.
	if n := c.count(t, `SELECT count(*) FROM core_questions WHERE jd_id=$1`, profile.JDID); n != 20 {
		t.Fatalf("bank size = %d, want 20", n)
	}
	if n := c.count(t, `SELECT count(*) FROM core_questions WHERE jd_id=$1 AND roadmap_item_id IS NULL`, profile.JDID); n != 0 {
		t.Fatalf("%d bank questions have no topic", n)
	}
	rows, err := c.pool.Query(c.ctx, `SELECT roadmap_item_id, count(*) FROM core_questions WHERE jd_id=$1 GROUP BY 1`, profile.JDID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	perTopic := 0
	for rows.Next() {
		var item uuid.UUID
		var n int
		if err = rows.Scan(&item, &n); err != nil || n < 5 {
			t.Fatalf("topic %s has %d questions (err=%v), want at least 5", item, n, err)
		}
		perTopic++
	}
	if perTopic != 4 {
		t.Fatalf("questions cover %d topics, want 4", perTopic)
	}
	// A retried confirmation changes nothing and calls no model again.
	if _, err = c.app.Confirm(c.ctx, owner, "candidate", profile.ID); err != nil {
		t.Fatal("repeat confirm", err)
	}
	if c.topics.calls.Load() != 1 || c.bank.calls.Load() != 1 {
		t.Fatalf("model calls topics=%d bank=%d, want 1 and 1", c.topics.calls.Load(), c.bank.calls.Load())
	}
}

func TestConfirmFailureLeavesTheProfileInReviewAndTheRoadmapFailedThenRetries(t *testing.T) {
	unusable := domain.TopicCandidate{Topics: []domain.GeneratedTopic{{Title: "SQL"}}}
	c := newConfirmEnv(t, &topicsFixture{replies: []domain.TopicCandidate{unusable, unusable, goodTopics()}}, 1)
	owner := c.user(t, "candidate")
	profile := c.reviewed(t, owner)

	_, err := c.app.Confirm(c.ctx, owner, "candidate", profile.ID)
	if !apperror.IsCode(err, apperror.CodeInvalidRoadmapGenerationOutput) {
		t.Fatalf("confirm = %v, want INVALID_ROADMAP_GENERATION_OUTPUT", err)
	}
	if got := c.roadmap(t, owner, profile.ID).GenerationStatus; got != domain.StatusFailed {
		t.Fatalf("status = %s, want failed", got)
	}
	if items, _ := c.roadmaps.ListItems(c.ctx, owner, profile.ID); len(items) != 0 {
		t.Fatalf("a failed generation stored %d topics", len(items))
	}
	if n := c.count(t, `SELECT count(*) FROM core_questions WHERE jd_id=$1`, profile.JDID); n != 0 {
		t.Fatalf("a failed generation stored %d questions", n)
	}
	if got, _ := c.app.Get(c.ctx, owner, "candidate", profile.ID); got.Status != rpdomain.Review {
		t.Fatalf("profile status = %s, want review", got.Status)
	}
	if c.bank.calls.Load() != 0 {
		t.Fatal("the bank was generated for unusable topics")
	}

	if _, err = c.app.Confirm(c.ctx, owner, "candidate", profile.ID); err != nil {
		t.Fatal("confirm after failure", err)
	}
	if got := c.roadmap(t, owner, profile.ID).GenerationStatus; got != domain.StatusReady {
		t.Fatalf("status = %s, want ready", got)
	}
}

func TestConcurrentConfirmationsGenerateOnce(t *testing.T) {
	c := newConfirmEnv(t, &topicsFixture{replies: []domain.TopicCandidate{goodTopics()}, delay: 300 * time.Millisecond}, 1)
	owner := c.user(t, "candidate")
	profile := c.reviewed(t, owner)

	var ok, conflict atomic.Int32
	var wg sync.WaitGroup
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := c.app.Confirm(c.ctx, owner, "candidate", profile.ID)
			switch {
			case err == nil:
				ok.Add(1)
			case apperror.IsCode(err, apperror.CodeRoadmapGenerating):
				conflict.Add(1)
			default:
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if c.topics.calls.Load() != 1 || c.bank.calls.Load() != 1 {
		t.Fatalf("model calls topics=%d bank=%d, want exactly one generation", c.topics.calls.Load(), c.bank.calls.Load())
	}
	if ok.Load() < 1 || ok.Load()+conflict.Load() != 4 {
		t.Fatalf("ok=%d conflict=%d", ok.Load(), conflict.Load())
	}
	if n := c.count(t, `SELECT count(*) FROM roadmap_items WHERE roadmap_id=$1`, c.roadmap(t, owner, profile.ID).ID); n != 4 {
		t.Fatalf("topics = %d, want 4", n)
	}
}

func TestSilentGenerationIsTakenOverAfterTheLease(t *testing.T) {
	c := newConfirmEnv(t, &topicsFixture{replies: []domain.TopicCandidate{goodTopics()}}, 1)
	owner := c.user(t, "candidate")
	profile := c.reviewed(t, owner)
	roadmap := c.roadmap(t, owner, profile.ID)
	// A worker claimed the roadmap long ago and died.
	c.exec(t, `UPDATE roadmaps SET generation_status='generating', updated_at=now()-interval '10 minutes' WHERE id=$1`, roadmap.ID)
	if _, err := c.app.Confirm(c.ctx, owner, "candidate", profile.ID); err != nil {
		t.Fatal("confirm over a dead generation", err)
	}
	if got := c.roadmap(t, owner, profile.ID).GenerationStatus; got != domain.StatusReady {
		t.Fatalf("status = %s, want ready", got)
	}
	// A generation that is still fresh is not taken over.
	other := c.reviewed(t, owner)
	c.exec(t, `UPDATE roadmaps SET generation_status='generating', updated_at=now() WHERE role_profile_id=$1`, other.ID)
	if _, err := c.app.Confirm(c.ctx, owner, "candidate", other.ID); !apperror.IsCode(err, apperror.CodeRoadmapGenerating) {
		t.Fatalf("confirm over a live generation = %v, want ROADMAP_GENERATING", err)
	}
}

// A model that ignores the topic quotas fails the distribution rule on every attempt: nothing is published,
// not even the topics, and the roadmap can be retried.
type lopsidedBank struct{ calls atomic.Int32 }

func (b *lopsidedBank) Generate(_ context.Context, in questionbankdomain.GenerationInput, count int) (questionbankdomain.GenerationCandidate, error) {
	b.calls.Add(1)
	id := in.Topics[0].ID
	candidate := questionbankdomain.GenerationCandidate{Questions: []questionbankdomain.GeneratedQuestion{}}
	for i := 0; i < count; i++ {
		candidate.Questions = append(candidate.Questions, questionbankdomain.GeneratedQuestion{Content: fmt.Sprintf("Question %d?", i), TopicID: &id})
	}
	return candidate, nil
}

func TestAnUnusableDistributionPublishesNothingAndLeavesTheRoadmapFailed(t *testing.T) {
	lopsided := &lopsidedBank{}
	c := newConfirmEnvWith(t, &topicsFixture{replies: []domain.TopicCandidate{goodTopics()}}, lopsided, 1, 1)
	owner := c.user(t, "candidate")
	profile := c.reviewed(t, owner)
	roadmap := c.roadmap(t, owner, profile.ID)

	_, err := c.app.Confirm(c.ctx, owner, "candidate", profile.ID)
	if !apperror.IsCode(err, apperror.CodeInvalidQuestionGenerationOutput) {
		t.Fatalf("confirm = %v, want INVALID_QUESTION_GENERATION_OUTPUT", err)
	}
	if lopsided.calls.Load() != 2 {
		t.Fatalf("bank attempts = %d, want the first and one retry", lopsided.calls.Load())
	}
	if got := c.roadmap(t, owner, profile.ID).GenerationStatus; got != domain.StatusFailed {
		t.Fatalf("status = %s, want failed with a retry available", got)
	}
	if n := c.count(t, `SELECT (SELECT count(*) FROM roadmap_items WHERE roadmap_id=$1) + (SELECT count(*) FROM core_questions WHERE jd_id=$2)`, roadmap.ID, profile.JDID); n != 0 {
		t.Fatalf("%d topic or bank rows were published for an unusable distribution", n)
	}
	if got, _ := c.app.Get(c.ctx, owner, "candidate", profile.ID); got.Status != rpdomain.Review {
		t.Fatalf("profile status = %s, want review", got.Status)
	}
}

func TestTheBankOfAProfileWithASessionIsNeverReplaced(t *testing.T) {
	c := newConfirmEnv(t, &topicsFixture{replies: []domain.TopicCandidate{goodTopics()}}, 1)
	owner := c.user(t, "candidate")
	profile := c.reviewed(t, owner)
	if _, err := c.app.Confirm(c.ctx, owner, "candidate", profile.ID); err != nil {
		t.Fatal(err)
	}
	items, _ := c.roadmaps.ListItems(c.ctx, owner, profile.ID)
	var question uuid.UUID
	if err := c.pool.QueryRow(c.ctx, `SELECT id FROM core_questions WHERE roadmap_item_id=$1 LIMIT 1`, items[0].ID).Scan(&question); err != nil {
		t.Fatal(err)
	}
	c.interview(t, profile.ID, question, uuid.Nil)
	before := c.count(t, `SELECT count(*) FROM core_questions WHERE jd_id=$1`, profile.JDID)
	c.exec(t, `UPDATE roadmaps SET generation_status='failed' WHERE role_profile_id=$1`, profile.ID)

	generation, err := roadmapservice.New(c.roadmaps, &topicsFixture{replies: []domain.TopicCandidate{goodTopics()}},
		questionbankrepository.NewRepository(c.pool), sessionBank(t, c), questionbankrepository.NewRepository(c.pool), 1)
	if err != nil {
		t.Fatal(err)
	}
	if err = generation.Prepare(c.ctx, owner, profile.ID, profile.JDID); !apperror.IsCode(err, apperror.CodeRoadmapRegenerateNotAllowed) {
		t.Fatalf("prepare = %v, want ROADMAP_REGENERATE_NOT_ALLOWED", err)
	}
	after, _ := c.roadmaps.ListItems(c.ctx, owner, profile.ID)
	if len(after) != 4 || after[0].ID != items[0].ID {
		t.Fatalf("topics changed under a session: %+v", after)
	}
	if n := c.count(t, `SELECT count(*) FROM core_questions WHERE jd_id=$1`, profile.JDID); n != before {
		t.Fatalf("bank size %d -> %d under a session", before, n)
	}
	if n := c.count(t, `SELECT count(*) FROM interview_questions WHERE core_question_id=$1`, question); n != 1 {
		t.Fatal("the session lost its question")
	}
}

func sessionBank(t *testing.T, c confirmEnv) *questionbankservice.Service {
	t.Helper()
	svc, err := questionbankservice.New(questionbankrepository.NewRepository(c.pool), c.bank, 0, 5)
	if err != nil {
		t.Fatal(err)
	}
	return svc
}
