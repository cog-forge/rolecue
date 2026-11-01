package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/cog-forge/rolecue/api/internal/features/application/domain"
	"github.com/cog-forge/rolecue/api/internal/pkg/logger"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

const sentinelReason = "SENTINEL-REJECT-REASON-TEXT"

// observedContext returns a context whose request logger writes to an in-memory observer.
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

// onlyLog returns the fields of the single entry with message, and fails when there is not exactly one.
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

func TestApproveCVLogsTheTransitionWithoutSecrets(t *testing.T) {
	ctx, logs := observedContext()
	repo := &fakeRepository{screening: domain.Screening{ID: applicationID, JobPostingID: postingID,
		Status: domain.StatusInterviewEligible, JobPostingStatus: "closed"}}
	if _, err := newService(t, repo).ApproveCV(ctx, recruiterID, recruiterRole, postingID, applicationID); err != nil {
		t.Fatal(err)
	}
	got := onlyLog(t, logs, "application cv approved")
	assertFields(t, got, map[string]string{
		"op": "application.approve_cv", "application_id": applicationID.String(), "posting_id": postingID.String(),
		"user_id": recruiterID.String(), "from_status": "pending", "to_status": string(domain.StatusInterviewEligible),
		"posting_status": "closed",
	})
}

func TestRejectCVLogsOnlyWhetherAReasonWasGiven(t *testing.T) {
	ctx, logs := observedContext()
	repo := &fakeRepository{screening: domain.Screening{ID: applicationID, JobPostingID: postingID, Status: domain.StatusRejected}}
	reason := sentinelReason
	if _, err := newService(t, repo).RejectCV(ctx, recruiterID, recruiterRole, postingID, applicationID, &reason); err != nil {
		t.Fatal(err)
	}
	got := onlyLog(t, logs, "application cv rejected")
	assertFields(t, got, map[string]string{
		"op": "application.reject_cv", "application_id": applicationID.String(), "from_status": "pending",
		"to_status": string(domain.StatusRejected), "has_reason": "true",
	})
	if strings.Contains(renderLogs(logs), sentinelReason) {
		t.Fatalf("the rejection reason text reached the log:\n%s", renderLogs(logs))
	}
}

func TestRejectedCVLogsNothing(t *testing.T) {
	ctx, logs := observedContext()
	repo := &fakeRepository{err: errors.New("application is not pending")}
	if _, err := newService(t, repo).RejectCV(ctx, recruiterID, recruiterRole, postingID, applicationID, nil); err == nil {
		t.Fatal("RejectCV() error = nil, want the repository error")
	}
	if n := logs.FilterMessage("application cv rejected").Len(); n != 0 {
		t.Fatalf("a failed rejection logged a success line %d times", n)
	}
}

func TestDecideLogsTheDecisionWithoutTheReason(t *testing.T) {
	ctx, logs := observedContext()
	repo := &decisionFake{fakeRepository: fakeRepository{detail: interviewedDetail()}}
	svc, err := New(repo, &fakeCVLinker{})
	if err != nil {
		t.Fatal(err)
	}
	reason := sentinelReason
	if _, err := svc.Decide(ctx, recruiterID, recruiterRole, postingID, applicationID, domain.DecisionReject, &reason); err != nil {
		t.Fatal(err)
	}
	got := onlyLog(t, logs, "application decided")
	assertFields(t, got, map[string]string{
		"op": "application.decide", "application_id": applicationID.String(), "posting_id": postingID.String(),
		"from_status": "interviewed", "to_status": string(domain.DecisionReject.Status()), "decision": "reject",
	})
	if strings.Contains(renderLogs(logs), sentinelReason) {
		t.Fatalf("the rejection reason text reached the log:\n%s", renderLogs(logs))
	}
}

func TestApplyLogsTheSubmissionWithoutTheCV(t *testing.T) {
	ctx, logs := observedContext()
	repo := newApplyRepository()
	submissionID := uuid.New()
	repo.submission = domain.Submission{ID: submissionID, JobPostingID: postingID, Status: domain.StatusPending}
	store := newApplyCVStore()
	cv := append(validCV(), []byte(sentinelReason)...)
	if _, err := applyService(t, repo, store).Apply(ctx, candidateID, candidateRole, postingID, cv); err != nil {
		t.Fatal(err)
	}
	got := onlyLog(t, logs, "application submitted")
	assertFields(t, got, map[string]string{
		"op": "application.apply", "application_id": submissionID.String(), "posting_id": postingID.String(),
		"user_id": candidateID.String(), "status": string(domain.StatusPending),
	})
	if strings.Contains(renderLogs(logs), sentinelReason) {
		t.Fatalf("CV content reached the log:\n%s", renderLogs(logs))
	}
}

// failingDeleteCVStore cannot remove the object again, the orphan case the service must report.
type failingDeleteCVStore struct {
	*applyCVStore
}

func (s *failingDeleteCVStore) Delete(context.Context, string) error {
	return errors.New("storage delete refused")
}

func TestApplyWarnsWhenTheOrphanedCVCannotBeDeleted(t *testing.T) {
	ctx, logs := observedContext()
	repo := newApplyRepository()
	repo.insertErr = errors.New("insert refused")
	store := &failingDeleteCVStore{applyCVStore: newApplyCVStore()}
	if _, err := applyService(t, repo, store).Apply(ctx, candidateID, candidateRole, postingID, validCV()); err == nil {
		t.Fatal("Apply() error = nil, want the insert error")
	}
	warn := logs.FilterMessage("application CV cleanup failed").All()
	if len(warn) != 1 || warn[0].Level != zap.WarnLevel {
		t.Fatalf("cleanup failure logs = %v, want one warn", warn)
	}
	assertFields(t, warn[0].ContextMap(), map[string]string{"op": "application.apply", "reason": "cv_object_orphaned"})
	if n := logs.FilterMessage("application submitted").Len(); n != 0 {
		t.Fatalf("a failed submission logged as submitted")
	}
}

// countingJob is an expiry pass that rejects a fixed number of Applications, or fails.
type countingJob struct {
	rejected int
	err      error
}

func (j countingJob) AutoRejectExpired(context.Context, time.Time) (int, error) {
	return j.rejected, j.err
}

func TestExpiryPassLogsTheCountOnlyWhenSomethingWasRejected(t *testing.T) {
	ctx, logs := observedContext()
	runner, err := NewExpiryRunner(countingJob{rejected: 3})
	if err != nil {
		t.Fatal(err)
	}
	runner.runOnce(ctx, time.Now)
	got := onlyLog(t, logs, "expired applications rejected")
	assertFields(t, got, map[string]string{"op": "application.expiry", "count": "3"})

	quiet, quietLogs := observedContext()
	idle, err := NewExpiryRunner(countingJob{})
	if err != nil {
		t.Fatal(err)
	}
	idle.runOnce(quiet, time.Now)
	if n := quietLogs.Len(); n != 0 {
		t.Fatalf("a pass that rejected nothing logged %d lines", n)
	}
}

func TestExpiryPassFailureIsLoggedAtErrorWithItsCause(t *testing.T) {
	ctx, logs := observedContext()
	runner, err := NewExpiryRunner(countingJob{err: errors.New("expiry lock lost")})
	if err != nil {
		t.Fatal(err)
	}
	runner.runOnce(ctx, time.Now)
	failed := logs.FilterMessage("application expiry pass failed").All()
	if len(failed) != 1 || failed[0].Level != zap.ErrorLevel {
		t.Fatalf("failure logs = %v, want one error", failed)
	}
	if _, ok := failed[0].ContextMap()["cause_chain"]; !ok {
		t.Fatalf("failure has no cause_chain: %v", failed[0].ContextMap())
	}
}
