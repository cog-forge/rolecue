package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/cog-forge/rolecue/api/internal/features/interview/domain"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

// fakeRepository records what the service passes in and returns the configured answers.
type fakeRepository struct {
	createParams  domain.CreateParams
	createSession domain.Session
	createCreated bool
	createErr     error
	createCalls   int

	getSession domain.Session
	getErr     error
	getCalls   int

	appendCandidate uuid.UUID
	appendPosition  int32
	appendAnswer    string
	appendSession   domain.Session
	appendErr       error
	appendCalls     int

	completeSession domain.Session
	completeFirst   bool
	completeErr     error
	completeCalls   int
}

func (r *fakeRepository) Create(_ context.Context, params domain.CreateParams) (domain.Session, bool, error) {
	r.createCalls++
	r.createParams = params
	return r.createSession, r.createCreated, r.createErr
}

func (r *fakeRepository) Get(_ context.Context, _, _ uuid.UUID) (domain.Session, error) {
	r.getCalls++
	return r.getSession, r.getErr
}

func (r *fakeRepository) AppendTurn(_ context.Context, candidateID, _ uuid.UUID, position int32, answer string) (domain.Session, error) {
	r.appendCalls++
	r.appendCandidate, r.appendPosition, r.appendAnswer = candidateID, position, answer
	return r.appendSession, r.appendErr
}

func (r *fakeRepository) Complete(_ context.Context, _, _ uuid.UUID) (domain.Session, bool, error) {
	r.completeCalls++
	return r.completeSession, r.completeFirst, r.completeErr
}

// fakeEvaluator reports each call on a channel, so a test waits for the background goroutine without
// sleeping.
type fakeEvaluator struct {
	calls chan uuid.UUID
}

func newFakeEvaluator() *fakeEvaluator {
	return &fakeEvaluator{calls: make(chan uuid.UUID, 4)}
}

func (e *fakeEvaluator) Evaluate(_ context.Context, interviewID uuid.UUID) error {
	e.calls <- interviewID
	return nil
}

var (
	candidateID   = uuid.MustParse("00000000-0000-4000-8000-000000000001")
	sessionID     = uuid.MustParse("00000000-0000-4000-8000-000000000010")
	profileID     = uuid.MustParse("00000000-0000-4000-8000-000000000020")
	applicationID = uuid.MustParse("00000000-0000-4000-8000-000000000030")
)

func newTestService(t *testing.T, repo *fakeRepository, settings Settings) *Service {
	t.Helper()
	svc, err := New(repo, settings)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return svc
}

func assertCode(t *testing.T, err error, want apperror.Code) {
	t.Helper()
	if !apperror.IsCode(err, want) {
		t.Fatalf("error = %v, want code %s", err, want)
	}
}

func practiceSession() domain.Session {
	turns := []domain.Turn{{Position: 1, Question: "Q1", Answer: "A1"}}
	return domain.Session{ID: sessionID, Type: domain.TypePractice, RoleProfileID: profileID, Turns: &turns}
}

func recruitmentSession() domain.Session {
	turns := []domain.Turn{{Position: 1, Question: "Q1", Answer: "A1"}}
	application := applicationID
	return domain.Session{
		ID: sessionID, Type: domain.TypeRecruitment, RoleProfileID: profileID, ApplicationID: &application,
		Turns: &turns, CurrentQuestion: &domain.CurrentQuestion{Position: 2, Content: "Q2"},
	}
}

func TestNewRequiresRepository(t *testing.T) {
	if _, err := New(nil, Settings{}); err == nil {
		t.Fatal("New(nil) returned no error")
	}
}

func TestNewDefaultsQuestionLimit(t *testing.T) {
	repo := &fakeRepository{}
	svc := newTestService(t, repo, Settings{PracticeCoins: 5})
	if svc.settings.QuestionLimit != defaultQuestionLimit {
		t.Fatalf("QuestionLimit = %d, want %d", svc.settings.QuestionLimit, defaultQuestionLimit)
	}
}

func TestRoleIsRequiredForEveryAction(t *testing.T) {
	repo := &fakeRepository{}
	svc := newTestService(t, repo, Settings{})
	ctx := context.Background()
	if _, _, err := svc.Create(ctx, candidateID, "recruiter", domain.CreateInput{Type: domain.TypePractice}); err == nil {
		t.Fatal("Create by recruiter succeeded")
	} else {
		assertCode(t, err, apperror.CodeForbidden)
	}
	if _, err := svc.Get(ctx, candidateID, "admin", sessionID); err == nil {
		t.Fatal("Get by admin succeeded")
	} else {
		assertCode(t, err, apperror.CodeForbidden)
	}
	if _, err := svc.Resume(ctx, candidateID, "recruiter", sessionID); err == nil {
		t.Fatal("Resume by recruiter succeeded")
	} else {
		assertCode(t, err, apperror.CodeForbidden)
	}
	if _, err := svc.AppendTurn(ctx, candidateID, "recruiter", sessionID, 1, "A"); err == nil {
		t.Fatal("AppendTurn by recruiter succeeded")
	} else {
		assertCode(t, err, apperror.CodeForbidden)
	}
	if _, err := svc.Complete(ctx, candidateID, "recruiter", sessionID); err == nil {
		t.Fatal("Complete by recruiter succeeded")
	} else {
		assertCode(t, err, apperror.CodeForbidden)
	}
	if repo.createCalls+repo.getCalls+repo.appendCalls+repo.completeCalls != 0 {
		t.Fatal("repository was called for a forbidden role")
	}
}

func TestCreateValidatesTypeAndReferences(t *testing.T) {
	application := applicationID
	profile := profileID
	tests := []struct {
		name  string
		input domain.CreateInput
	}{
		{name: "unknown type", input: domain.CreateInput{Type: "mock"}},
		{name: "empty type", input: domain.CreateInput{}},
		{name: "practice without role profile", input: domain.CreateInput{Type: domain.TypePractice}},
		{name: "practice with application", input: domain.CreateInput{Type: domain.TypePractice, RoleProfileID: &profile, ApplicationID: &application}},
		{name: "recruitment without application", input: domain.CreateInput{Type: domain.TypeRecruitment, RoleProfileID: &profile}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &fakeRepository{}
			svc := newTestService(t, repo, Settings{})
			_, _, err := svc.Create(context.Background(), candidateID, "candidate", tt.input)
			assertCode(t, err, apperror.CodeValidation)
			if repo.createCalls != 0 {
				t.Fatal("repository was called for an invalid request")
			}
		})
	}
}

func TestCreatePracticeChargesConfiguredPrice(t *testing.T) {
	repo := &fakeRepository{createSession: practiceSession(), createCreated: true}
	svc := newTestService(t, repo, Settings{PracticeCoins: 5, QuestionLimit: 3})
	profile := profileID
	session, created, err := svc.Create(context.Background(), candidateID, "candidate", domain.CreateInput{
		Type: domain.TypePractice, RoleProfileID: &profile,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if !created {
		t.Fatal("created = false, want true")
	}
	if repo.createParams.PracticeCoins != 5 || repo.createParams.QuestionLimit != 3 {
		t.Fatalf("params = %+v, want coins 5 and limit 3", repo.createParams)
	}
	if repo.createParams.CandidateID != candidateID {
		t.Fatalf("CandidateID = %s, want %s", repo.createParams.CandidateID, candidateID)
	}
	if session.Turns == nil {
		t.Fatal("practice session lost its turns")
	}
}

func TestCreateRecruitmentIsNeverCharged(t *testing.T) {
	repo := &fakeRepository{createSession: recruitmentSession(), createCreated: true}
	svc := newTestService(t, repo, Settings{PracticeCoins: 5})
	application := applicationID
	if _, _, err := svc.Create(context.Background(), candidateID, "candidate", domain.CreateInput{
		Type: domain.TypeRecruitment, ApplicationID: &application,
	}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if repo.createParams.PracticeCoins != 0 {
		t.Fatalf("PracticeCoins = %d for a recruitment session, want 0", repo.createParams.PracticeCoins)
	}
}

func TestCreateExistingRecruitmentReturnsNotCreated(t *testing.T) {
	repo := &fakeRepository{createSession: recruitmentSession(), createCreated: false}
	svc := newTestService(t, repo, Settings{})
	application := applicationID
	_, created, err := svc.Create(context.Background(), candidateID, "candidate", domain.CreateInput{
		Type: domain.TypeRecruitment, ApplicationID: &application,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created {
		t.Fatal("created = true for an existing session, want false")
	}
}

func TestCreatePassesRepositoryErrorsThrough(t *testing.T) {
	repo := &fakeRepository{createErr: apperror.New(apperror.CodeInsufficientBalance, "insufficient coin balance")}
	svc := newTestService(t, repo, Settings{PracticeCoins: 5})
	profile := profileID
	_, _, err := svc.Create(context.Background(), candidateID, "candidate", domain.CreateInput{
		Type: domain.TypePractice, RoleProfileID: &profile,
	})
	assertCode(t, err, apperror.CodeInsufficientBalance)
}

func TestRecruitmentSessionNeverReturnsTurns(t *testing.T) {
	repo := &fakeRepository{getSession: recruitmentSession()}
	svc := newTestService(t, repo, Settings{})
	session, err := svc.Get(context.Background(), candidateID, "candidate", sessionID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if session.Turns != nil {
		t.Fatalf("recruitment Turns = %v, want nil", *session.Turns)
	}
	body, err := json.Marshal(session)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if strings.Contains(string(body), `"turns"`) {
		t.Fatalf("recruitment JSON has a turns key: %s", body)
	}
	if session.CurrentQuestion == nil || session.CurrentQuestion.Content != "Q2" {
		t.Fatalf("CurrentQuestion = %+v, want the current question", session.CurrentQuestion)
	}
}

func TestPracticeSessionReturnsTurnsWhenEmpty(t *testing.T) {
	empty := []domain.Turn{}
	practice := domain.Session{ID: sessionID, Type: domain.TypePractice, Turns: &empty}
	repo := &fakeRepository{getSession: practice}
	svc := newTestService(t, repo, Settings{})
	session, err := svc.Get(context.Background(), candidateID, "candidate", sessionID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	body, err := json.Marshal(session)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if !strings.Contains(string(body), `"turns":[]`) {
		t.Fatalf("practice JSON = %s, want turns []", body)
	}
}

func TestResumeReturnsSessionWithoutWriting(t *testing.T) {
	repo := &fakeRepository{getSession: practiceSession()}
	svc := newTestService(t, repo, Settings{PracticeCoins: 5})
	session, err := svc.Resume(context.Background(), candidateID, "candidate", sessionID)
	if err != nil {
		t.Fatalf("Resume: %v", err)
	}
	if session.ID != sessionID {
		t.Fatalf("ID = %s, want %s", session.ID, sessionID)
	}
	if repo.createCalls+repo.appendCalls+repo.completeCalls != 0 {
		t.Fatal("resume wrote through the repository")
	}
}

func TestResumeRefusesCompletedSession(t *testing.T) {
	completed := practiceSession()
	now := time.Now()
	completed.CompletedAt = &now
	repo := &fakeRepository{getSession: completed}
	svc := newTestService(t, repo, Settings{})
	_, err := svc.Resume(context.Background(), candidateID, "candidate", sessionID)
	assertCode(t, err, apperror.CodeInterviewCompleted)
}

func TestAppendTurnNormalizesAnswer(t *testing.T) {
	repo := &fakeRepository{appendSession: practiceSession()}
	svc := newTestService(t, repo, Settings{})
	if _, err := svc.AppendTurn(context.Background(), candidateID, "candidate", sessionID, 2, "  my answer \n"); err != nil {
		t.Fatalf("AppendTurn: %v", err)
	}
	if repo.appendAnswer != "my answer" {
		t.Fatalf("answer = %q, want the trimmed text", repo.appendAnswer)
	}
	if repo.appendPosition != 2 || repo.appendCandidate != candidateID {
		t.Fatalf("position = %d candidate = %s, want 2 and the caller", repo.appendPosition, repo.appendCandidate)
	}
}

func TestAppendTurnRejectsInvalidInput(t *testing.T) {
	tests := []struct {
		name     string
		position int32
		answer   string
	}{
		{name: "position zero", position: 0, answer: "A"},
		{name: "blank answer", position: 1, answer: "   \t\n"},
		{name: "answer too long", position: 1, answer: strings.Repeat("a", 8001)},
		{name: "invalid UTF-8", position: 1, answer: string([]byte{0xff, 0xfe})},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &fakeRepository{}
			svc := newTestService(t, repo, Settings{})
			_, err := svc.AppendTurn(context.Background(), candidateID, "candidate", sessionID, tt.position, tt.answer)
			assertCode(t, err, apperror.CodeValidation)
			if repo.appendCalls != 0 {
				t.Fatal("repository was called for invalid input")
			}
		})
	}
}

func TestAppendTurnAcceptsAnswerAtLimit(t *testing.T) {
	repo := &fakeRepository{appendSession: practiceSession()}
	svc := newTestService(t, repo, Settings{})
	if _, err := svc.AppendTurn(context.Background(), candidateID, "candidate", sessionID, 1, strings.Repeat("a", 8000)); err != nil {
		t.Fatalf("AppendTurn at the limit: %v", err)
	}
}

func TestAppendTurnPassesConflictThrough(t *testing.T) {
	repo := &fakeRepository{appendErr: domain.ErrTurnConflict}
	svc := newTestService(t, repo, Settings{})
	_, err := svc.AppendTurn(context.Background(), candidateID, "candidate", sessionID, 2, "A")
	assertCode(t, err, apperror.CodeInterviewTurnConflict)
}

func TestCompleteStartsEvaluationOnceAfterFirstCompletion(t *testing.T) {
	evaluator := newFakeEvaluator()
	repo := &fakeRepository{completeSession: practiceSession(), completeFirst: true}
	svc := newTestService(t, repo, Settings{})
	svc.SetEvaluator(evaluator)

	if _, err := svc.Complete(context.Background(), candidateID, "candidate", sessionID); err != nil {
		t.Fatalf("first Complete: %v", err)
	}
	select {
	case got := <-evaluator.calls:
		if got != sessionID {
			t.Fatalf("evaluated %s, want %s", got, sessionID)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("evaluator was not started after the first completion")
	}

	repo.completeFirst = false
	if _, err := svc.Complete(context.Background(), candidateID, "candidate", sessionID); err != nil {
		t.Fatalf("repeat Complete: %v", err)
	}
	if len(evaluator.calls) != 0 {
		t.Fatalf("evaluator started %d more times on a repeat completion", len(evaluator.calls))
	}
}

func TestCompleteWithoutEvaluatorDoesNotFail(t *testing.T) {
	repo := &fakeRepository{completeSession: practiceSession(), completeFirst: true}
	svc := newTestService(t, repo, Settings{})
	if _, err := svc.Complete(context.Background(), candidateID, "candidate", sessionID); err != nil {
		t.Fatalf("Complete without evaluator: %v", err)
	}
}

func TestCompletePassesRepositoryErrorsThrough(t *testing.T) {
	evaluator := newFakeEvaluator()
	repo := &fakeRepository{completeErr: domain.ErrInterviewNotFound}
	svc := newTestService(t, repo, Settings{})
	svc.SetEvaluator(evaluator)
	_, err := svc.Complete(context.Background(), candidateID, "candidate", sessionID)
	assertCode(t, err, apperror.CodeInterviewNotFound)
	if len(evaluator.calls) != 0 {
		t.Fatal("evaluator started after a failed completion")
	}
}
