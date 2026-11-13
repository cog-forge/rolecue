package service

import (
	"context"
	"errors"
	"testing"
	"time"

	questionbankdomain "github.com/cog-forge/rolecue/api/internal/features/questionbank/domain"
	"github.com/cog-forge/rolecue/api/internal/features/roadmap/domain"
	"github.com/cog-forge/rolecue/api/internal/features/roadmap/repository"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type fakeStore struct {
	item        domain.Item
	itemErr     error
	locks       int
	state       domain.ProfileState
	stateErr    error
	restores    int
	previous    domain.GenerationStatus
	roadmap     domain.Roadmap
	claimErr    error
	publishErr  error
	claims      int
	fails       int
	failCtxLive bool
	published   [][]domain.NewItem
	allowedFrom []domain.GenerationStatus
	staleBefore time.Time
}

func (f *fakeStore) EnsureForProfile(context.Context, uuid.UUID, uuid.UUID) (domain.Roadmap, error) {
	return f.roadmap, nil
}

func (f *fakeStore) GetItem(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (domain.Item, error) {
	return f.item, f.itemErr
}

func (f *fakeStore) InPracticeLock(ctx context.Context, _, _ uuid.UUID, fn func(ctx context.Context, tx pgx.Tx) error) error {
	f.locks++
	return fn(ctx, nil)
}

func (f *fakeStore) ProfileState(context.Context, uuid.UUID, uuid.UUID) (domain.ProfileState, error) {
	return f.state, f.stateErr
}

func (f *fakeStore) Restore(context.Context, uuid.UUID, domain.Fence) error {
	f.restores++
	return nil
}

func (f *fakeStore) Claim(_ context.Context, _ uuid.UUID, from []domain.GenerationStatus, stale time.Time) (domain.Fence, error) {
	f.claims++
	f.allowedFrom, f.staleBefore = from, stale
	if f.claimErr != nil {
		return domain.Fence{}, f.claimErr
	}
	f.previous = f.roadmap.GenerationStatus
	f.roadmap.GenerationStatus = domain.StatusGenerating
	return domain.Fence{UpdatedAt: time.Unix(100, 0), Previous: f.previous}, nil
}

func (f *fakeStore) Fail(ctx context.Context, _ uuid.UUID, _ domain.Fence) error {
	f.fails++
	f.failCtxLive = ctx.Err() == nil
	return nil
}

// Publish runs the hook like the real repository does, inside what would be its transaction.
func (f *fakeStore) Publish(ctx context.Context, _ uuid.UUID, _ domain.Fence, items []domain.NewItem, hook repository.PublishHook) ([]domain.Item, error) {
	if f.publishErr != nil {
		return nil, f.publishErr
	}
	if hook != nil {
		if err := hook(ctx, nil, nil); err != nil {
			return nil, err
		}
	}
	f.published = append(f.published, items)
	return nil, nil
}

type fakeTopics struct {
	calls   int
	replies []func() (domain.TopicCandidate, error)
	seen    []domain.TopicInput
}

func (f *fakeTopics) GenerateTopics(_ context.Context, in domain.TopicInput) (domain.TopicCandidate, error) {
	f.seen = append(f.seen, in)
	reply := f.replies[min(f.calls, len(f.replies)-1)]
	f.calls++
	return reply()
}

func valid() func() (domain.TopicCandidate, error) {
	return func() (domain.TopicCandidate, error) { return candidateOf("SQL", "Go", "Design", "Caching"), nil }
}

func unusable() func() (domain.TopicCandidate, error) {
	return func() (domain.TopicCandidate, error) { return candidateOf("SQL"), nil }
}

type fakeInputs struct {
	input questionbankdomain.GenerationInput
	err   error
}

func (f fakeInputs) GenerationInput(context.Context, uuid.UUID) (questionbankdomain.GenerationInput, error) {
	return f.input, f.err
}

// fakeBank is both halves of the bank: the draft (the second model call) and the store the publish hook uses.
type fakeBank struct {
	calls      int
	err        error
	seenTopics []questionbankdomain.RoadmapTopic
	storeErr   error
	storeCalls int
	storedFor  uuid.UUID

	topUpCalls    int
	topUpCount    int
	topUpTopic    questionbankdomain.RoadmapTopic
	topUpErr      error
	appendCalls   int
	appendErr     error
	appendedTotal int
	appendedTopic uuid.UUID
}

func (f *fakeBank) DraftPracticeBank(_ context.Context, _, _ uuid.UUID, topics []questionbankdomain.RoadmapTopic) ([]questionbankdomain.GeneratedQuestion, error) {
	f.calls++
	f.seenTopics = topics
	return []questionbankdomain.GeneratedQuestion{{Content: "Q?"}}, f.err
}

func (f *fakeBank) DraftTopicQuestions(_ context.Context, _, _ uuid.UUID, topic questionbankdomain.RoadmapTopic, count int) ([]questionbankdomain.GeneratedQuestion, error) {
	f.topUpCalls++
	f.topUpTopic, f.topUpCount = topic, count
	return []questionbankdomain.GeneratedQuestion{{Content: "More?"}}, f.topUpErr
}

func (f *fakeBank) AppendTopicQuestions(_ context.Context, _ pgx.Tx, _, topicID uuid.UUID, total int, _ []questionbankdomain.GeneratedQuestion) (bool, error) {
	f.appendCalls++
	f.appendedTotal = total
	f.appendedTopic = topicID
	return true, f.appendErr
}

func (f *fakeBank) ReplacePracticeBank(_ context.Context, _ pgx.Tx, jdID uuid.UUID, _ []questionbankdomain.GeneratedQuestion) error {
	f.storeCalls++
	f.storedFor = jdID
	return f.storeErr
}

func confirmed() fakeInputs {
	note := "5 years"
	return fakeInputs{input: questionbankdomain.GenerationInput{
		Seniority:      questionbankdomain.SeniorityLevel{Name: "Senior"},
		Requirements:   []questionbankdomain.ConfirmedRequirement{{Name: "Go", Target: "backend", Type: "required", EvidenceText: &note}},
		RefinementNote: "focus on caching",
	}}
}

type harness struct {
	store  *fakeStore
	topics *fakeTopics
	bank   *fakeBank
	svc    *Service
}

func newHarness(t *testing.T, status domain.GenerationStatus, retries int, inputs fakeInputs, replies ...func() (domain.TopicCandidate, error)) *harness {
	t.Helper()
	h := &harness{store: &fakeStore{roadmap: domain.Roadmap{ID: uuid.New(), GenerationStatus: status}, state: domain.ProfileState{JDID: uuid.New(), Confirmed: true}}, topics: &fakeTopics{replies: replies}, bank: &fakeBank{}}
	svc, err := New(h.store, h.topics, inputs, h.bank, h.bank, retries)
	if err != nil {
		t.Fatal(err)
	}
	h.svc = svc
	return h
}

func (h *harness) prepare(ctx context.Context) error {
	return h.svc.Prepare(ctx, uuid.New(), uuid.New(), uuid.New())
}

func TestPrepareClaimsGeneratesBothStepsAndPublishesOnce(t *testing.T) {
	h := newHarness(t, domain.StatusPending, 1, confirmed(), valid())
	if err := h.prepare(context.Background()); err != nil {
		t.Fatal(err)
	}
	if h.store.claims != 1 || h.topics.calls != 1 || h.bank.calls != 1 || len(h.store.published) != 1 || h.store.fails != 0 {
		t.Fatalf("claims=%d topics=%d bank=%d published=%d fails=%d, want 1 1 1 1 0", h.store.claims, h.topics.calls, h.bank.calls, len(h.store.published), h.store.fails)
	}
	if len(h.store.allowedFrom) != 2 || h.store.allowedFrom[0] != domain.StatusPending || h.store.allowedFrom[1] != domain.StatusFailed {
		t.Fatalf("claim allowed from %v, want pending and failed", h.store.allowedFrom)
	}
	if !h.store.staleBefore.Before(time.Now().Add(-time.Minute)) {
		t.Fatal("a claim must only take over generations silent for the lease")
	}
	in := h.topics.seen[0]
	if in.SeniorityName != "Senior" || len(in.Requirements) != 1 || in.Requirements[0].Name != "Go" || in.RefinementNote != "focus on caching" {
		t.Fatalf("topic input = %+v", in)
	}
}

func TestPrepareOfAReadyRoadmapCallsNothing(t *testing.T) {
	h := newHarness(t, domain.StatusReady, 1, confirmed(), valid())
	if err := h.prepare(context.Background()); err != nil {
		t.Fatal(err)
	}
	if h.store.claims+h.topics.calls+h.bank.calls != 0 {
		t.Fatal("a ready roadmap must not be generated again")
	}
}

func TestPrepareOfAGeneratingRoadmapIsAConflict(t *testing.T) {
	h := newHarness(t, domain.StatusGenerating, 1, confirmed(), valid())
	h.store.claimErr = repository.ErrNotClaimable
	err := h.prepare(context.Background())
	if !apperror.IsCode(err, apperror.CodeRoadmapGenerating) {
		t.Fatalf("err = %v, want ROADMAP_GENERATING", err)
	}
	if h.topics.calls+h.bank.calls != 0 || h.store.fails != 0 {
		t.Fatal("a refused claim must call no model and mark nothing failed")
	}
}

func TestPrepareRetriesUnusableTopicsOnceThenFails(t *testing.T) {
	h := newHarness(t, domain.StatusPending, 1, confirmed(), unusable())
	err := h.prepare(context.Background())
	if !apperror.IsCode(err, apperror.CodeInvalidRoadmapGenerationOutput) {
		t.Fatalf("err = %v, want INVALID_ROADMAP_GENERATION_OUTPUT", err)
	}
	if h.topics.calls != 2 || h.bank.calls != 0 || h.store.fails != 1 || len(h.store.published) != 0 {
		t.Fatalf("topics=%d bank=%d fails=%d published=%d, want 2 0 1 0", h.topics.calls, h.bank.calls, h.store.fails, len(h.store.published))
	}
}

func TestPrepareAcceptsASecondAttempt(t *testing.T) {
	h := newHarness(t, domain.StatusFailed, 1, confirmed(), unusable(), valid())
	if err := h.prepare(context.Background()); err != nil {
		t.Fatal(err)
	}
	if h.topics.calls != 2 || h.bank.calls != 1 || len(h.store.published) != 1 {
		t.Fatalf("topics=%d bank=%d published=%d", h.topics.calls, h.bank.calls, len(h.store.published))
	}
}

func TestPrepareDoesNotRetryWhenRetriesAreOff(t *testing.T) {
	h := newHarness(t, domain.StatusPending, 0, confirmed(), unusable())
	if err := h.prepare(context.Background()); err == nil || h.topics.calls != 1 {
		t.Fatalf("calls=%d err=%v, want one call and an error", h.topics.calls, err)
	}
}

func TestPrepareMapsAProviderErrorAndNeverRetriesIt(t *testing.T) {
	boom := func() (domain.TopicCandidate, error) { return domain.TopicCandidate{}, errors.New("provider down") }
	h := newHarness(t, domain.StatusPending, 1, confirmed(), boom)
	err := h.prepare(context.Background())
	if !apperror.IsCode(err, apperror.CodeRoadmapGenerationFailed) || h.topics.calls != 1 || h.store.fails != 1 {
		t.Fatalf("err=%v calls=%d fails=%d", err, h.topics.calls, h.store.fails)
	}
}

func TestPrepareMarksFailedWhenTheBankFailsAndPublishesNothing(t *testing.T) {
	h := newHarness(t, domain.StatusPending, 1, confirmed(), valid())
	h.bank.err = apperror.New(apperror.CodeQuestionGenerationFailed, "bank failed")
	err := h.prepare(context.Background())
	if !apperror.IsCode(err, apperror.CodeQuestionGenerationFailed) || h.store.fails != 1 || len(h.store.published) != 0 {
		t.Fatalf("err=%v fails=%d published=%d", err, h.store.fails, len(h.store.published))
	}
}

func TestPrepareGeneratesTheBankForTheTopicsItJustPlannedAndPublishesItWithThem(t *testing.T) {
	h := newHarness(t, domain.StatusPending, 1, confirmed(), valid())
	if err := h.prepare(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(h.bank.seenTopics) != 4 || len(h.store.published[0]) != 4 {
		t.Fatalf("bank topics=%d published=%d, want 4 and 4", len(h.bank.seenTopics), len(h.store.published[0]))
	}
	for i, published := range h.store.published[0] {
		if published.ID == uuid.Nil || published.ID != h.bank.seenTopics[i].ID || published.Title != h.bank.seenTopics[i].Title {
			t.Fatalf("topic %d: published %+v but the bank was generated for %+v", i, published, h.bank.seenTopics[i])
		}
	}
	if h.bank.storeCalls != 1 {
		t.Fatalf("bank stored %d times in the publish, want 1", h.bank.storeCalls)
	}
}

func TestPrepareStoresNoBankWhenThePublishFails(t *testing.T) {
	h := newHarness(t, domain.StatusPending, 1, confirmed(), valid())
	h.store.publishErr = repository.ErrFenceLost
	_ = h.prepare(context.Background())
	if h.bank.storeCalls != 0 {
		t.Fatal("the bank must only be written by a publish that holds the fence")
	}
}

func TestPrepareMapsABankInUseToRegenerateNotAllowed(t *testing.T) {
	h := newHarness(t, domain.StatusPending, 1, confirmed(), valid())
	h.bank.storeErr = questionbankdomain.ErrBankInUse
	err := h.prepare(context.Background())
	if !apperror.IsCode(err, apperror.CodeRoadmapRegenerateNotAllowed) || h.store.fails != 1 || len(h.store.published) != 0 {
		t.Fatalf("err=%v fails=%d published=%d", err, h.store.fails, len(h.store.published))
	}
}

func TestPrepareWithoutRequirementsCallsNoModel(t *testing.T) {
	h := newHarness(t, domain.StatusPending, 1, fakeInputs{}, valid())
	err := h.prepare(context.Background())
	if !apperror.IsCode(err, apperror.CodeRequirementsNotConfirmed) || h.topics.calls+h.bank.calls != 0 {
		t.Fatalf("err=%v topics=%d bank=%d", err, h.topics.calls, h.bank.calls)
	}
}

func TestPrepareReportsALostFenceAsAConflictAndRecordsNothingElse(t *testing.T) {
	h := newHarness(t, domain.StatusPending, 1, confirmed(), valid())
	h.store.publishErr = repository.ErrFenceLost
	if err := h.prepare(context.Background()); !apperror.IsCode(err, apperror.CodeRoadmapGenerating) {
		t.Fatalf("err = %v, want ROADMAP_GENERATING", err)
	}
}

func TestPrepareRecordsFailureEvenWhenTheRequestWasCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancelling := func() (domain.TopicCandidate, error) {
		cancel()
		return domain.TopicCandidate{}, context.Canceled
	}
	h := newHarness(t, domain.StatusPending, 1, confirmed(), cancelling)
	if err := h.prepare(ctx); !apperror.IsCode(err, apperror.CodeRoadmapGenerationFailed) {
		t.Fatalf("err = %v, want ROADMAP_GENERATION_FAILED", err)
	}
	if h.store.fails != 1 || !h.store.failCtxLive {
		t.Fatalf("fails=%d liveContext=%v: the failure must be written on a context that outlives the request", h.store.fails, h.store.failCtxLive)
	}
}
