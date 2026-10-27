package service

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"sort"
	"testing"

	"github.com/cog-forge/rolecue/api/internal/features/admin/interviews/domain"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

type fakeRepository struct {
	rows   []domain.InterviewRow
	total  int64
	err    error
	calls  int
	filter domain.ListFilter

	detail   domain.InterviewDetail
	getErr   error
	getCalls int
	getID    uuid.UUID
}

func (f *fakeRepository) List(_ context.Context, filter domain.ListFilter) ([]domain.InterviewRow, int64, error) {
	f.calls++
	f.filter = filter
	return f.rows, f.total, f.err
}

func (f *fakeRepository) Get(_ context.Context, id uuid.UUID) (domain.InterviewDetail, error) {
	f.getCalls++
	f.getID = id
	return f.detail, f.getErr
}

func assertCode(t *testing.T, err error, want apperror.Code) {
	t.Helper()
	var appErr *apperror.AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("want %s, got %v", want, err)
	}
	if appErr.Code != want {
		t.Fatalf("want %s, got %s", want, appErr.Code)
	}
}

func validFilter() domain.ListFilter {
	return domain.ListFilter{Page: 1, PageSize: 20}
}

func TestListRefusesNonAdmins(t *testing.T) {
	for _, role := range []string{"candidate", "recruiter", "", "Admin"} {
		repo := &fakeRepository{}
		svc, err := New(repo)
		if err != nil {
			t.Fatal(err)
		}
		_, err = svc.List(context.Background(), role, validFilter())
		assertCode(t, err, apperror.CodeForbidden)
		if repo.calls != 0 {
			t.Fatalf("role %q reached the repository", role)
		}
	}
}

func TestListRejectsInvalidFilters(t *testing.T) {
	cases := map[string]domain.ListFilter{
		"unknown type":      {Type: "mock", Page: 1, PageSize: 20},
		"page zero":         {Page: 0, PageSize: 20},
		"page size zero":    {Page: 1, PageSize: 0},
		"page size too big": {Page: 1, PageSize: 101},
		"offset overflow":   {Page: math.MaxInt32, PageSize: 100},
	}
	for name, filter := range cases {
		t.Run(name, func(t *testing.T) {
			repo := &fakeRepository{}
			svc, _ := New(repo)
			_, err := svc.List(context.Background(), "admin", filter)
			assertCode(t, err, apperror.CodeValidation)
			if repo.calls != 0 {
				t.Fatal("invalid filter reached the repository")
			}
		})
	}
}

func TestListForwardsFiltersAndPage(t *testing.T) {
	candidate := uuid.New()
	posting := uuid.New()
	repo := &fakeRepository{total: 42}
	svc, _ := New(repo)
	filter := domain.ListFilter{Type: domain.TypeRecruitment, CandidateID: &candidate, JobPostingID: &posting, Page: 3, PageSize: 10}
	page, err := svc.List(context.Background(), "admin", filter)
	if err != nil {
		t.Fatal(err)
	}
	if repo.filter.Type != domain.TypeRecruitment || *repo.filter.CandidateID != candidate || *repo.filter.JobPostingID != posting {
		t.Fatalf("filters not forwarded: %+v", repo.filter)
	}
	if repo.filter.Page != 3 || repo.filter.PageSize != 10 {
		t.Fatalf("page not forwarded: %+v", repo.filter)
	}
	if page.Total != 42 || page.Page != 3 || page.PageSize != 10 {
		t.Fatalf("unexpected page envelope: %+v", page)
	}
}

func TestListReturnsEmptyArrayNotNil(t *testing.T) {
	svc, _ := New(&fakeRepository{})
	page, err := svc.List(context.Background(), "admin", validFilter())
	if err != nil {
		t.Fatal(err)
	}
	if page.Items == nil {
		t.Fatal("items must be an empty slice, not nil")
	}
	body, _ := json.Marshal(page)
	if string(body) != `{"items":[],"total":0,"page":1,"page_size":20}` {
		t.Fatalf("unexpected body %s", body)
	}
}

func TestListPropagatesRepositoryError(t *testing.T) {
	boom := apperror.New(apperror.CodeInternal, "boom")
	svc, _ := New(&fakeRepository{err: boom})
	_, err := svc.List(context.Background(), "admin", validFilter())
	assertCode(t, err, apperror.CodeInternal)
}

// The list response is an allow-list. Serialize a row carrying every field the domain has and
// assert the exact set of keys, so a new field or a leaked path is caught here.
func TestInterviewRowJSONIsAllowListOnly(t *testing.T) {
	score := int32(4)
	row := domain.InterviewRow{
		ID:           uuid.New(),
		Type:         domain.TypeRecruitment,
		Owner:        domain.OwnerRef{ID: uuid.New(), Name: "A", Email: "a@example.com"},
		JobPosting:   &domain.JobPostingRef{ID: uuid.New(), Title: "Backend"},
		Score:        &score,
		HasReport:    true,
		HasRecording: true,
	}
	page := domain.InterviewPage{Items: []domain.InterviewRow{row}, Total: 1, Page: 1, PageSize: 20}
	body, err := json.Marshal(page)
	if err != nil {
		t.Fatal(err)
	}
	var decoded struct {
		Items []map[string]json.RawMessage `json:"items"`
	}
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatal(err)
	}
	keys := make([]string, 0, len(decoded.Items[0]))
	for key := range decoded.Items[0] {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	want := []string{"has_recording", "has_report", "id", "job_posting", "owner", "score", "type"}
	if len(keys) != len(want) {
		t.Fatalf("row keys %v, want %v", keys, want)
	}
	for i := range want {
		if keys[i] != want[i] {
			t.Fatalf("row keys %v, want %v", keys, want)
		}
	}
	for _, forbidden := range []string{"record_path", "transcript", "answer", "feedback", "question", "path", "audio", "video"} {
		if _, ok := decoded.Items[0][forbidden]; ok {
			t.Fatalf("forbidden key %q in response", forbidden)
		}
	}
}

func TestGetRefusesNonAdminsBeforeLookup(t *testing.T) {
	for _, role := range []string{"candidate", "recruiter", ""} {
		repo := &fakeRepository{}
		svc, _ := New(repo)
		_, err := svc.Get(context.Background(), role, uuid.New())
		assertCode(t, err, apperror.CodeForbidden)
		if repo.getCalls != 0 {
			t.Fatalf("role %q reached the repository", role)
		}
	}
}

func TestGetPassesIDAndReturnsDetail(t *testing.T) {
	id := uuid.New()
	detail := domain.InterviewDetail{QuestionCount: 3, TurnCount: 7}
	detail.ID = id
	repo := &fakeRepository{detail: detail}
	svc, _ := New(repo)
	got, err := svc.Get(context.Background(), "admin", id)
	if err != nil {
		t.Fatal(err)
	}
	if repo.getID != id || got.QuestionCount != 3 || got.TurnCount != 7 {
		t.Fatalf("unexpected detail %+v for id %s", got, repo.getID)
	}
}

func TestGetPropagatesNotFound(t *testing.T) {
	repo := &fakeRepository{getErr: domain.ErrInterviewNotFound}
	svc, _ := New(repo)
	_, err := svc.Get(context.Background(), "admin", uuid.New())
	assertCode(t, err, apperror.CodeInterviewNotFound)
}

// The detail response is an allow-list too. Serialize a fully populated detail and assert the exact
// key set, including the embedded list fields.
func TestInterviewDetailJSONIsAllowListOnly(t *testing.T) {
	score := int32(5)
	appID := uuid.New()
	detail := domain.InterviewDetail{
		InterviewRow: domain.InterviewRow{
			ID:           uuid.New(),
			Type:         domain.TypeRecruitment,
			Owner:        domain.OwnerRef{ID: uuid.New(), Name: "A", Email: "a@example.com"},
			JobPosting:   &domain.JobPostingRef{ID: uuid.New(), Title: "Backend"},
			Score:        &score,
			HasReport:    true,
			HasRecording: true,
		},
		RoleProfile:   &domain.RoleProfileRef{ID: uuid.New(), Title: "Go developer"},
		ApplicationID: &appID,
		Avatar:        &domain.AssetRef{ID: uuid.New(), Name: "Avatar"},
		Voice:         &domain.AssetRef{ID: uuid.New(), Name: "Voice"},
		QuestionCount: 4,
		TurnCount:     9,
	}
	body, err := json.Marshal(detail)
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]json.RawMessage
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatal(err)
	}
	keys := make([]string, 0, len(decoded))
	for key := range decoded {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	want := []string{"application_id", "avatar", "has_recording", "has_report", "id", "job_posting", "owner", "question_count", "role_profile", "score", "turn_count", "type", "voice"}
	if len(keys) != len(want) {
		t.Fatalf("detail keys %v, want %v", keys, want)
	}
	for i := range want {
		if keys[i] != want[i] {
			t.Fatalf("detail keys %v, want %v", keys, want)
		}
	}
}
