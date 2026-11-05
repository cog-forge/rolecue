package service

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/cog-forge/rolecue/api/internal/features/interview/domain"
	"github.com/cog-forge/rolecue/api/internal/pkg/logger"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

const sentinelAnswer = "SENTINEL-ANSWER-7f3a I would shard the table by tenant."

// sentinelPanicEvaluator panics with the answer text, as a scorer bug could.
type sentinelPanicEvaluator struct{}

func (sentinelPanicEvaluator) Evaluate(context.Context, uuid.UUID) error {
	panic(sentinelAnswer)
}

func observedContext() (context.Context, *observer.ObservedLogs) {
	core, logs := observer.New(zap.DebugLevel)
	return logger.ToContext(context.Background(), &logger.Logger{Logger: zap.New(core)}), logs
}

// loggedText renders every message and field, so a sentinel anywhere in the output is found.
func loggedText(logs *observer.ObservedLogs) string {
	var out strings.Builder
	for _, entry := range logs.All() {
		fmt.Fprintf(&out, "%s %v\n", entry.Message, entry.ContextMap())
	}
	return out.String()
}

func onlyEntry(t *testing.T, logs *observer.ObservedLogs, message string) observer.LoggedEntry {
	t.Helper()
	entries := logs.FilterMessage(message).All()
	if len(entries) != 1 {
		t.Fatalf("%q logged %d times, want 1", message, len(entries))
	}
	return entries[0]
}

func TestCreateLogsStartedInterview(t *testing.T) {
	ctx, logs := observedContext()
	repo := &fakeRepository{createSession: practiceSession(), createCreated: true}
	svc := newTestService(t, repo, Settings{})
	if _, _, err := svc.Create(ctx, candidateID, "candidate", domain.CreateInput{Type: domain.TypePractice, RoleProfileID: &profileID}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	fields := onlyEntry(t, logs, "interview started").ContextMap()
	if fields["op"] != "interview.start" || fields["interview_id"] != sessionID.String() ||
		fields["user_id"] != candidateID.String() || fields["type"] != "practice" {
		t.Fatalf("interview started fields = %v", fields)
	}
}

func TestRepeatedStartLogsNoStartedEvent(t *testing.T) {
	ctx, logs := observedContext()
	repo := &fakeRepository{createSession: practiceSession(), createCreated: false}
	svc := newTestService(t, repo, Settings{})
	if _, _, err := svc.Create(ctx, candidateID, "candidate", domain.CreateInput{Type: domain.TypePractice, RoleProfileID: &profileID}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if n := logs.FilterMessage("interview started").Len(); n != 0 {
		t.Fatalf("a reused session logged %d started events", n)
	}
}

func TestAppendTurnLogsProgressNeverAnswerText(t *testing.T) {
	ctx, logs := observedContext()
	session := practiceSession()
	session.AnsweredCount, session.QuestionCount = 1, 8
	repo := &fakeRepository{appendSession: session}
	svc := newTestService(t, repo, Settings{})
	if _, err := svc.AppendTurn(ctx, candidateID, "candidate", sessionID, 1, sentinelAnswer); err != nil {
		t.Fatalf("AppendTurn: %v", err)
	}
	fields := onlyEntry(t, logs, "interview answer recorded").ContextMap()
	if fields["question_index"] != int64(1) || fields["answered_count"] != int64(1) || fields["question_count"] != int64(8) {
		t.Fatalf("answer recorded fields = %v", fields)
	}
	if strings.Contains(loggedText(logs), "SENTINEL-ANSWER") {
		t.Fatal("answer text reached the log")
	}
}

func TestCompleteLogsTheEndWithReasonAndDuration(t *testing.T) {
	ctx, logs := observedContext()
	evaluator := newFakeEvaluator()
	session := practiceSession()
	started := time.Now().Add(-time.Minute)
	completed := started.Add(time.Minute)
	left := domain.EndLeft
	session.StartedAt, session.CompletedAt, session.EndReason = started, &completed, &left
	repo := &fakeRepository{completeSession: session, completeFirst: true, completeEnded: true}
	svc := newTestService(t, repo, Settings{})
	svc.SetEvaluator(evaluator)
	if _, err := svc.Complete(ctx, candidateID, "candidate", sessionID); err != nil {
		t.Fatalf("Complete: %v", err)
	}
	<-evaluator.calls
	fields := onlyEntry(t, logs, "interview ended").ContextMap()
	if _, ok := fields["duration"]; !ok || fields["op"] != "interview.end" || fields["interview_id"] != sessionID.String() ||
		fields["end_reason"] != "left" {
		t.Fatalf("interview ended fields = %v", fields)
	}
}

func TestCompleteWarnsWhenEvaluationCannotStart(t *testing.T) {
	ctx, logs := observedContext()
	repo := &fakeRepository{completeSession: practiceSession(), completeFirst: true, completeEnded: true}
	svc := newTestService(t, repo, Settings{})
	if _, err := svc.Complete(ctx, candidateID, "candidate", sessionID); err != nil {
		t.Fatalf("Complete: %v", err)
	}
	entry := onlyEntry(t, logs, "interview evaluation not started")
	if entry.Level != zapcore.WarnLevel || entry.ContextMap()["reason"] != "no evaluator configured" {
		t.Fatalf("evaluation skip = %v at %v", entry.ContextMap(), entry.Level)
	}
}

func TestEvaluationPanicLogsWithoutPanicValue(t *testing.T) {
	ctx, logs := observedContext()
	repo := &fakeRepository{completeSession: practiceSession(), completeFirst: true, failed: make(chan uuid.UUID, 1)}
	svc := newTestService(t, repo, Settings{})
	svc.SetEvaluator(sentinelPanicEvaluator{})
	if _, err := svc.Complete(ctx, candidateID, "candidate", sessionID); err != nil {
		t.Fatalf("Complete: %v", err)
	}
	select {
	case <-repo.failed:
	case <-time.After(5 * time.Second):
		t.Fatal("a panicking evaluation was not marked failed")
	}
	fields := onlyEntry(t, logs, "interview evaluation panicked").ContextMap()
	if fields["panic_type"] != "string" || fields["op"] != "interview.evaluate" {
		t.Fatalf("panic fields = %v", fields)
	}
	if strings.Contains(loggedText(logs), "SENTINEL-ANSWER") {
		t.Fatal("the panic value reached the log")
	}
}

func TestRecordingStatusChangeIsLogged(t *testing.T) {
	ctx, logs := observedContext()
	repo := &fakeRecordingRepository{
		state:    recruitmentRecordingState(domain.RecordUploading, recordingPath(testRecordingKey)),
		resultOK: true,
	}
	store := &fakeRecordingStore{headInfo: webmInfo(`"v1"`), prefix: webmHead}
	svc := newTestRecordingService(t, repo, store)
	if _, err := svc.CompleteUpload(ctx, uuid.New(), "candidate", uuid.New()); err != nil {
		t.Fatalf("CompleteUpload: %v", err)
	}
	fields := onlyEntry(t, logs, "recording status changed").ContextMap()
	if fields["from_status"] != "uploading" || fields["to_status"] != "ready" || fields["bytes"] != int64(10) {
		t.Fatalf("recording status fields = %v", fields)
	}
	if strings.Contains(loggedText(logs), testRecordingKey) {
		t.Fatal("the storage key reached the log")
	}
}
