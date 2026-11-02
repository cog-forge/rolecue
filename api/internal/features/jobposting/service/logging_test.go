package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/cog-forge/rolecue/api/internal/features/jobposting/domain"
	roleprofiledomain "github.com/cog-forge/rolecue/api/internal/features/roleprofile/domain"
	"github.com/cog-forge/rolecue/api/internal/pkg/logger"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

const sentinelJDText = "SENTINEL-JD-TEXT-BODY"

func observedContext() (context.Context, *observer.ObservedLogs) {
	core, logs := observer.New(zap.DebugLevel)
	return logger.ToContext(context.Background(), &logger.Logger{Logger: zap.New(core)}), logs
}

// renderLogs flattens every message and field into one string, so a test can look for a secret anywhere.
func renderLogs(logs *observer.ObservedLogs) string {
	var b strings.Builder
	for _, e := range logs.All() {
		fmt.Fprintf(&b, "%s %v\n", e.Message, e.ContextMap())
	}
	return b.String()
}

func onlyLog(t *testing.T, logs *observer.ObservedLogs, message string) map[string]any {
	t.Helper()
	found := logs.FilterMessage(message).All()
	if len(found) != 1 {
		t.Fatalf("%q logged %d times, want once; logs:\n%s", message, len(found), renderLogs(logs))
	}
	return found[0].ContextMap()
}

func assertFields(t *testing.T, got map[string]any, want map[string]string) {
	t.Helper()
	for key, value := range want {
		if fmt.Sprint(got[key]) != value {
			t.Errorf("field %s = %v, want %s; all fields: %v", key, got[key], value, got)
		}
	}
}

func TestCreateLogsThePostingWithoutTheJDText(t *testing.T) {
	ctx, logs := observedContext()
	store := newStore()
	documentID := store.putPDF(sentinelJDText + " " + validText())
	repo := &fakeRepository{}
	posting, err := newService(t, repo, store).Create(ctx, uuid.New(), recruiterRole, validCreate(documentID))
	if err != nil {
		t.Fatal(err)
	}
	got := onlyLog(t, logs, "job posting created")
	assertFields(t, got, map[string]string{
		"op": "jobposting.create", "posting_id": posting.ID.String(), "status": "pending", "count": "2",
	})
	if strings.Contains(renderLogs(logs), sentinelJDText) {
		t.Fatalf("JD text reached the log:\n%s", renderLogs(logs))
	}
}

func TestSubmitAndResubmitLogTheStatusMove(t *testing.T) {
	text := validText()
	id, recruiter := uuid.New(), uuid.New()

	ctx, logs := observedContext()
	repo := &fakeRepository{record: domain.Record{StoredText: &text, JobPosting: domain.JobPosting{ID: id, Status: domain.StatusPending}}}
	if _, err := newService(t, repo, newStore()).Submit(ctx, recruiter, recruiterRole, id); err != nil {
		t.Fatal(err)
	}
	assertFields(t, onlyLog(t, logs, "job posting submitted"), map[string]string{
		"op": "jobposting.submit", "posting_id": id.String(), "user_id": recruiter.String(),
		"from_status": "pending", "to_status": "pending",
	})

	ctx, logs = observedContext()
	repo = &fakeRepository{record: domain.Record{StoredText: &text, JobPosting: domain.JobPosting{ID: id, Status: domain.StatusRejected}}}
	if _, err := newService(t, repo, newStore()).Resubmit(ctx, recruiter, recruiterRole, id); err != nil {
		t.Fatal(err)
	}
	assertFields(t, onlyLog(t, logs, "job posting resubmitted"), map[string]string{
		"op": "jobposting.resubmit", "from_status": "rejected", "to_status": "pending",
	})
}

func TestSuggestLogsTheStepButNeverTheJDText(t *testing.T) {
	raw := sentinelJDText + " " + validText()

	ctx, logs := observedContext()
	ext, err := NewExtraction(&fakeExtractor{result: roleprofiledomain.StructuredJD{}}, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ext.Suggest(ctx, recruiterRole, raw); err != nil {
		t.Fatal(err)
	}
	ok := logs.FilterMessage("step ok").FilterField(zap.String("op", "jobposting.suggest")).All()
	if len(ok) != 1 {
		t.Fatalf("suggest step ok logs = %d, want 1; logs:\n%s", len(ok), renderLogs(logs))
	}

	failCtx, failLogs := observedContext()
	failing, err := NewExtraction(&fakeExtractor{err: errors.New("model unavailable")}, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := failing.Suggest(failCtx, recruiterRole, raw); err == nil {
		t.Fatal("Suggest() error = nil, want the extractor error")
	}
	failed := failLogs.FilterMessage("step failed").FilterField(zap.String("op", "jobposting.suggest")).All()
	if len(failed) != 1 || failed[0].Level != zap.WarnLevel {
		t.Fatalf("suggest failure logs = %v, want one warn", failed)
	}
	if strings.Contains(renderLogs(failLogs), sentinelJDText) || strings.Contains(renderLogs(logs), sentinelJDText) {
		t.Fatal("JD text reached the log")
	}
}
