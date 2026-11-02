package service

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/cog-forge/rolecue/api/internal/features/admin/moderation/domain"
	"github.com/google/uuid"
)

func TestApproveLogsDecisionWithoutReason(t *testing.T) {
	repo := postingIn(domain.PostingStatusPending)
	svc, logs := newObservedService(t, repo)
	adminID := uuid.New()
	if _, err := svc.Approve(context.Background(), adminID, "admin", repo.posting.ID); err != nil {
		t.Fatalf("Approve: %v", err)
	}
	entries := logs.FilterMessage("job posting moderated").All()
	if len(entries) != 1 {
		t.Fatalf("decision logged %d times, want 1", len(entries))
	}
	fields := entries[0].ContextMap()
	if fields["op"] != "moderation.decide" || fields["admin_id"] != adminID.String() || fields["status"] != "open" {
		t.Fatalf("decision fields = %v", fields)
	}
	if _, ok := fields["reason_length"]; ok {
		t.Fatal("an approval must not carry a reason length")
	}
}

func TestRejectLogsReasonLengthNeverText(t *testing.T) {
	repo := postingIn(domain.PostingStatusPending)
	svc, logs := newObservedService(t, repo)
	reason := "SENTINEL-REASON-7f3a: the salary range is missing"
	if _, err := svc.Reject(context.Background(), uuid.New(), "admin", repo.posting.ID, reason); err != nil {
		t.Fatalf("Reject: %v", err)
	}
	entries := logs.FilterMessage("job posting moderated").All()
	if len(entries) != 1 {
		t.Fatalf("decision logged %d times, want 1", len(entries))
	}
	fields := entries[0].ContextMap()
	if fields["status"] != "rejected" || fields["reason_length"] != int64(utf8.RuneCountInString(reason)) {
		t.Fatalf("rejection fields = %v", fields)
	}
	for _, entry := range logs.All() {
		if text := fmt.Sprint(entry.Message, entry.ContextMap()); strings.Contains(text, "SENTINEL") || strings.Contains(text, "salary") {
			t.Fatalf("reject reason reached the log: %s", text)
		}
	}
}
