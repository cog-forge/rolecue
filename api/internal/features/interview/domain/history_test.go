package domain

import (
	"encoding/json"
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestDeriveSessionStatus(t *testing.T) {
	done := time.Date(2026, 10, 2, 10, 0, 0, 0, time.UTC)
	tests := []struct {
		name        string
		completedAt *time.Time
		evaluation  EvaluationStatus
		want        SessionStatus
	}{
		{name: "running, evaluation not started", evaluation: EvaluationPending, want: StatusInProgress},
		{name: "running ignores a stale evaluation", evaluation: EvaluationCompleted, want: StatusInProgress},
		{name: "completed, evaluation pending", completedAt: &done, evaluation: EvaluationPending, want: StatusPending},
		{name: "completed, evaluated", completedAt: &done, evaluation: EvaluationCompleted, want: StatusCompleted},
		{name: "completed, evaluation failed", completedAt: &done, evaluation: EvaluationFailed, want: StatusFailed},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DeriveSessionStatus(tt.completedAt, tt.evaluation); got != tt.want {
				t.Fatalf("status = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestHistoryPageAllowList pins the JSON keys of the history page and its items. A new field must be
// added here on purpose, so questions, turns, feedback and the recording path cannot leak in by accident.
func TestHistoryPageAllowList(t *testing.T) {
	title := "Backend Engineer"
	score := int32(81)
	page := HistoryPage{
		Items: []HistoryItem{{
			ID: uuid.New(), Type: TypeRecruitment, StartedAt: time.Now(), EvaluationStatus: EvaluationCompleted,
			Status: StatusCompleted, Score: &score, RoleProfileTitle: "Backend", JobPostingTitle: &title,
		}},
		Total: 1, Page: 1, PageSize: 20,
	}
	raw, err := json.Marshal(page)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var body struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	var envelope map[string]any
	if err := json.Unmarshal(raw, &envelope); err != nil {
		t.Fatalf("unmarshal envelope: %v", err)
	}
	wantPage := []string{"items", "page", "page_size", "total"}
	wantItem := []string{
		"application_id", "completed_at", "evaluation_status", "id", "job_posting_title",
		"role_profile_title", "score", "started_at", "status", "type",
	}
	if got := sortedKeys(envelope); !reflect.DeepEqual(got, wantPage) {
		t.Fatalf("page keys = %v, want %v", got, wantPage)
	}
	if got := sortedKeys(body.Items[0]); !reflect.DeepEqual(got, wantItem) {
		t.Fatalf("item keys = %v, want %v", got, wantItem)
	}
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
