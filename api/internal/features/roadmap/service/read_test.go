package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/cog-forge/rolecue/api/internal/features/roadmap/domain"
	"github.com/cog-forge/rolecue/api/internal/features/roadmap/service"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
)

type fakeRepo struct {
	rec     domain.RoadmapRecord
	rows    []domain.ResultRow
	err     error
	calls   int
	gotUser uuid.UUID
	gotProf uuid.UUID
}

func (f *fakeRepo) LoadReadModel(_ context.Context, candidate, profile uuid.UUID) (domain.RoadmapRecord, []domain.ResultRow, error) {
	f.calls++
	f.gotUser, f.gotProf = candidate, profile
	return f.rec, f.rows, f.err
}

func wantCode(t *testing.T, err error, code apperror.Code) {
	t.Helper()
	var appErr *apperror.AppError
	if !errors.As(err, &appErr) || appErr.Code != code {
		t.Fatalf("error = %v, want code %s", err, code)
	}
}

func uid(n byte) uuid.UUID {
	var id uuid.UUID
	id[15] = n
	return id
}

func repoWithTopic() *fakeRepo {
	done := time.Date(2026, 11, 1, 9, 0, 0, 0, time.UTC)
	score := 90.0
	return &fakeRepo{
		rec: domain.RoadmapRecord{ID: uid(1), RoleProfileID: uid(2), GenerationStatus: domain.StatusReady, PracticeStarted: true,
			Items: []domain.ItemRecord{{ID: uid(3), Title: "SQL", Position: 1}}},
		rows: []domain.ResultRow{{ItemID: uid(3), CoreQuestionID: uid(4), InterviewID: uid(5), Position: 1, InterviewType: "practice",
			CompletedAt: &done, EvaluationStatus: "completed", Delivered: true, Answered: true, QuestionScore: &score, Linked: true}},
	}
}

func TestOnlyCandidatesReadARoadmap(t *testing.T) {
	for _, role := range []string{"recruiter", "admin", ""} {
		repo := repoWithTopic()
		svc := service.NewReadService(repo)
		_, err := svc.Get(context.Background(), uid(9), role, uid(2))
		wantCode(t, err, apperror.CodeForbidden)
		_, _, err = svc.ItemHistory(context.Background(), uid(9), role, uid(2), uid(3), 20, 0)
		wantCode(t, err, apperror.CodeForbidden)
		if repo.calls != 0 {
			t.Fatalf("role %q reached the repository", role)
		}
	}
}

func TestGetPassesTheOwnerToTheRepository(t *testing.T) {
	repo := repoWithTopic()
	view, err := service.NewReadService(repo).Get(context.Background(), uid(9), "candidate", uid(2))
	if err != nil {
		t.Fatal(err)
	}
	if repo.gotUser != uid(9) || repo.gotProf != uid(2) {
		t.Fatalf("repository got %s/%s", repo.gotUser, repo.gotProf)
	}
	if len(view.Items) != 1 || view.Items[0].Score == nil || *view.Items[0].Score != 90 || view.Items[0].EvidenceQuestionCount != 1 {
		t.Fatalf("view = %+v", view)
	}
}

func TestForeignOrMissingProfileIsPassedThrough(t *testing.T) {
	repo := &fakeRepo{err: apperror.New(apperror.CodeJDNotFound, "Role Profile not found.")}
	_, err := service.NewReadService(repo).Get(context.Background(), uid(9), "candidate", uid(2))
	wantCode(t, err, apperror.CodeJDNotFound)
	_, _, err = service.NewReadService(repo).ItemHistory(context.Background(), uid(9), "candidate", uid(2), uid(3), 20, 0)
	wantCode(t, err, apperror.CodeJDNotFound)
}

func TestHistoryUnknownItemIsNotFound(t *testing.T) {
	repo := repoWithTopic()
	_, _, err := service.NewReadService(repo).ItemHistory(context.Background(), uid(9), "candidate", uid(2), uid(99), 20, 0)
	wantCode(t, err, apperror.CodeRoadmapItemNotFound)
}

func TestHistoryPagesAndTotals(t *testing.T) {
	repo := repoWithTopic()
	base := repo.rows[0]
	for i := 0; i < 6; i++ {
		row := base
		row.CoreQuestionID = uid(byte(20 + i))
		row.InterviewID = uid(byte(30 + i))
		done := base.CompletedAt.Add(time.Duration(i+1) * time.Hour)
		row.CompletedAt = &done
		repo.rows = append(repo.rows, row)
	}
	svc := service.NewReadService(repo)
	page, total, err := svc.ItemHistory(context.Background(), uid(9), "candidate", uid(2), uid(3), 3, 0)
	if err != nil || total != 7 || len(page) != 3 {
		t.Fatalf("page = %d entries, total %d, err %v", len(page), total, err)
	}
	page2, _, err := svc.ItemHistory(context.Background(), uid(9), "candidate", uid(2), uid(3), 3, 6)
	if err != nil || len(page2) != 1 {
		t.Fatalf("last page = %d entries, err %v", len(page2), err)
	}
	// Newest first: the first row of page 1 is the newest evaluation.
	if !page[0].EvaluatedAt.After(page2[0].EvaluatedAt) {
		t.Fatal("history is not newest first across pages")
	}
}

func TestHistoryRejectsBadPaging(t *testing.T) {
	svc := service.NewReadService(repoWithTopic())
	for _, tc := range []struct{ limit, offset int32 }{{0, 0}, {101, 0}, {20, -1}, {-5, 0}} {
		_, _, err := svc.ItemHistory(context.Background(), uid(9), "candidate", uid(2), uid(3), tc.limit, tc.offset)
		wantCode(t, err, apperror.CodeValidation)
	}
}
