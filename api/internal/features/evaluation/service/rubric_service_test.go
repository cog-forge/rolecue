package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/cog-forge/rolecue/api/internal/features/evaluation/domain"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

// fakeRubricRepository keeps rubrics in memory and applies the same visibility rule as the SQL: an Admin sees
// system rubrics, a Recruiter sees the active system rubrics and their own. It records what the service asked
// it to store, so the tests can check what reached persistence. Locking, concurrency and transactions are the
// real repository's job and are covered by the integration tests.
type fakeRubricRepository struct {
	metrics []domain.Metric
	rubrics map[uuid.UUID]domain.Rubric

	created []domain.RubricWrite
	copies  []domain.RubricCopy
	updates []domain.RubricUpdate
	deleted []uuid.UUID
	listed  []domain.RubricListQuery

	updateErr error
	deleteErr error
}

func newFakeRubricRepository(rubrics ...domain.Rubric) *fakeRubricRepository {
	repo := &fakeRubricRepository{
		metrics: []domain.Metric{{ID: rbAccuracy, Name: "Accuracy"}, {ID: rbProblem, Name: "Problem"}},
		rubrics: map[uuid.UUID]domain.Rubric{},
	}
	for _, rubric := range rubrics {
		repo.rubrics[rubric.ID] = rubric
	}
	return repo
}

func (f *fakeRubricRepository) ListMetrics(context.Context) ([]domain.Metric, error) {
	return f.metrics, nil
}

func visibleTo(access domain.RubricAccess, r domain.Rubric) bool {
	switch access.Role {
	case domain.RubricRoleAdmin:
		return r.Scope == domain.RubricScopeSystem
	case domain.RubricRoleRecruiter:
		return (r.Scope == domain.RubricScopeSystem && r.IsActive) || (r.OwnerID != nil && *r.OwnerID == access.UserID)
	}
	return false
}

func (f *fakeRubricRepository) ListRubrics(_ context.Context, access domain.RubricAccess, q domain.RubricListQuery) ([]domain.Rubric, int64, error) {
	f.listed = append(f.listed, q)
	var out []domain.Rubric
	for _, rubric := range f.rubrics {
		if visibleTo(access, rubric) {
			out = append(out, rubric)
		}
	}
	return out, int64(len(out)), nil
}

func (f *fakeRubricRepository) GetRubric(_ context.Context, access domain.RubricAccess, id uuid.UUID) (domain.Rubric, error) {
	rubric, ok := f.rubrics[id]
	if !ok || !visibleTo(access, rubric) {
		return domain.Rubric{}, domain.ErrRubricNotFound
	}
	return rubric, nil
}

func (f *fakeRubricRepository) CreateRubric(_ context.Context, write domain.RubricWrite) (domain.Rubric, error) {
	f.created = append(f.created, write)
	return domain.Rubric{ID: uuid.New(), Name: write.Name, Scope: write.Scope, OwnerID: write.OwnerID, IsActive: true, UpdatedAt: time.Now()}, nil
}

func (f *fakeRubricRepository) CopyRubric(ctx context.Context, access domain.RubricAccess, sourceID uuid.UUID, c domain.RubricCopy) (domain.Rubric, error) {
	if _, err := f.GetRubric(ctx, access, sourceID); err != nil {
		return domain.Rubric{}, err
	}
	f.copies = append(f.copies, c)
	return domain.Rubric{ID: uuid.New(), Name: c.Name, Scope: c.Scope, OwnerID: c.OwnerID, IsActive: true, UpdatedAt: time.Now()}, nil
}

func (f *fakeRubricRepository) UpdateRubric(_ context.Context, id uuid.UUID, u domain.RubricUpdate) (domain.Rubric, error) {
	f.updates = append(f.updates, u)
	if f.updateErr != nil {
		return domain.Rubric{}, f.updateErr
	}
	rubric := f.rubrics[id]
	rubric.UpdatedAt = u.ExpectedUpdatedAt.Add(time.Second)
	return rubric, nil
}

func (f *fakeRubricRepository) DeleteRubric(_ context.Context, id uuid.UUID) error {
	f.deleted = append(f.deleted, id)
	return f.deleteErr
}

var (
	rbAdmin     = uuid.MustParse("00000000-0000-4000-8000-0000000000e1")
	rbRecruiter = uuid.MustParse("00000000-0000-4000-8000-0000000000e2")
	rbOther     = uuid.MustParse("00000000-0000-4000-8000-0000000000e3")
)

func refOf[T any](v T) *T { return &v }

func systemRubric() domain.Rubric {
	return domain.Rubric{ID: uid(0x51), Name: "System template", Scope: domain.RubricScopeSystem, IsActive: true, References: 3, UpdatedAt: time.Unix(1000, 0).UTC()}
}

func privateRubric(owner uuid.UUID, n byte) domain.Rubric {
	return domain.Rubric{ID: uid(n), Name: "Private", Scope: domain.RubricScopeRecruiter, OwnerID: &owner, IsActive: true, References: 2, UpdatedAt: time.Unix(2000, 0).UTC()}
}

func validCriterion() domain.CriterionInput {
	return domain.CriterionInput{MetricID: refOf(rbAccuracy), Name: refOf("Explains the mechanism"), MaxPoints: refOf(int32(3)), ScoringGuide: refOf("0 = wrong; 1 = partly; 2 = mostly; 3 = fully")}
}

func validCreate() domain.RubricCreate {
	return domain.RubricCreate{
		Name: refOf("Mechanism"), Description: refOf("For mechanism questions"), QuestionType: refOf("concept_explanation"),
		Criteria: []domain.CriterionInput{validCriterion()},
	}
}

func newRubrics(t *testing.T, repo *fakeRubricRepository) *Rubrics {
	t.Helper()
	svc, err := NewRubrics(repo)
	if err != nil {
		t.Fatal(err)
	}
	return svc
}

func wantCode(t *testing.T, err error, code apperror.Code) {
	t.Helper()
	var appErr *apperror.AppError
	if !errors.As(err, &appErr) || appErr.Code != code {
		t.Fatalf("error = %v, want code %s", err, code)
	}
}

func TestRubricAPIRefusesEveryOtherRole(t *testing.T) {
	repo := newFakeRubricRepository(systemRubric())
	svc := newRubrics(t, repo)
	ctx := context.Background()
	id := systemRubric().ID
	calls := map[string]func() error{
		"list": func() error {
			_, _, err := svc.List(ctx, rbRecruiter, "candidate", domain.RubricListQuery{})
			return err
		},
		"get": func() error { _, err := svc.Get(ctx, rbRecruiter, "candidate", id); return err },
		"create": func() error {
			_, err := svc.Create(ctx, rbRecruiter, "candidate", validCreate())
			return err
		},
		"copy": func() error { _, err := svc.Copy(ctx, rbRecruiter, "candidate", id, nil); return err },
		"update": func() error {
			_, err := svc.Update(ctx, rbRecruiter, "candidate", id, domain.RubricPatch{UpdatedAt: refOf(time.Now()), IsActive: refOf(false)})
			return err
		},
		"delete": func() error { return svc.Delete(ctx, rbRecruiter, "candidate", id) },
	}
	for name, call := range calls {
		t.Run(name, func(t *testing.T) { wantCode(t, call(), apperror.CodeForbidden) })
	}
	if len(repo.created)+len(repo.copies)+len(repo.updates)+len(repo.deleted)+len(repo.listed) != 0 {
		t.Fatal("the repository was reached for a role that may not use the API")
	}
}

func TestCreateFollowsTheCallersRole(t *testing.T) {
	repo := newFakeRubricRepository()
	svc := newRubrics(t, repo)

	got, err := svc.Create(context.Background(), rbRecruiter, "recruiter", validCreate())
	if err != nil {
		t.Fatal(err)
	}
	if len(repo.created) != 1 || repo.created[0].Scope != domain.RubricScopeRecruiter || repo.created[0].OwnerID == nil || *repo.created[0].OwnerID != rbRecruiter {
		t.Fatalf("recruiter create stored %+v, want a private rubric owned by the caller", repo.created)
	}
	if !got.IsOwn || !got.CanManage || got.Scope != domain.RubricScopeRecruiter {
		t.Fatalf("response flags = own %v manage %v scope %q", got.IsOwn, got.CanManage, got.Scope)
	}

	got, err = svc.Create(context.Background(), rbAdmin, "admin", validCreate())
	if err != nil {
		t.Fatal(err)
	}
	stored := repo.created[1]
	if stored.Scope != domain.RubricScopeSystem || stored.OwnerID != nil {
		t.Fatalf("admin create stored %+v, want a system rubric without an owner", stored)
	}
	if got.IsOwn || !got.CanManage {
		t.Fatalf("admin flags = own %v manage %v, want can_manage only", got.IsOwn, got.CanManage)
	}
	// Text is stored trimmed and the criteria are numbered by the repository from their order.
	if stored.Name != "Mechanism" || len(stored.Criteria) != 1 || stored.Criteria[0].MaxPoints != 3 {
		t.Fatalf("stored rubric = %+v", stored)
	}
}

func TestCreateValidation(t *testing.T) {
	long := func(n int) string { return strings.Repeat("a", n) }
	mutate := func(f func(*domain.RubricCreate)) domain.RubricCreate {
		in := validCreate()
		in.Criteria = []domain.CriterionInput{validCriterion()}
		f(&in)
		return in
	}
	cases := []struct {
		name string
		in   domain.RubricCreate
		code apperror.Code
	}{
		{"missing name", mutate(func(in *domain.RubricCreate) { in.Name = nil }), apperror.CodeValidation},
		{"blank name", mutate(func(in *domain.RubricCreate) { in.Name = refOf(" \n ") }), apperror.CodeValidation},
		{"name too long", mutate(func(in *domain.RubricCreate) { in.Name = refOf(long(domain.MaxRubricNameRunes + 1)) }), apperror.CodeValidation},
		{"description too long", mutate(func(in *domain.RubricCreate) { in.Description = refOf(long(domain.MaxRubricDescriptionRunes + 1)) }), apperror.CodeValidation},
		{"missing question type", mutate(func(in *domain.RubricCreate) { in.QuestionType = nil }), apperror.CodeValidation},
		{"question type not a slug", mutate(func(in *domain.RubricCreate) { in.QuestionType = refOf("Concept Explanation") }), apperror.CodeValidation},
		{"no criteria", mutate(func(in *domain.RubricCreate) { in.Criteria = nil }), apperror.CodeValidation},
		{"too many criteria", mutate(func(in *domain.RubricCreate) {
			in.Criteria = nil
			for i := 0; i <= domain.MaxRubricCriteria; i++ {
				in.Criteria = append(in.Criteria, validCriterion())
			}
		}), apperror.CodeValidation},
		{"criterion without metric", mutate(func(in *domain.RubricCreate) { in.Criteria[0].MetricID = nil }), apperror.CodeValidation},
		{"criterion with an unknown metric", mutate(func(in *domain.RubricCreate) { in.Criteria[0].MetricID = refOf(uuid.New()) }), apperror.CodeEvaluationMetricUnknown},
		{"criterion without points", mutate(func(in *domain.RubricCreate) { in.Criteria[0].MaxPoints = nil }), apperror.CodeValidation},
		{"zero points", mutate(func(in *domain.RubricCreate) { in.Criteria[0].MaxPoints = refOf(int32(0)) }), apperror.CodeValidation},
		{"negative points", mutate(func(in *domain.RubricCreate) { in.Criteria[0].MaxPoints = refOf(int32(-1)) }), apperror.CodeValidation},
		{"points above the limit", mutate(func(in *domain.RubricCreate) { in.Criteria[0].MaxPoints = refOf(int32(domain.MaxCriterionPoints + 1)) }), apperror.CodeValidation},
		{"blank criterion name", mutate(func(in *domain.RubricCreate) { in.Criteria[0].Name = refOf("  ") }), apperror.CodeValidation},
		{"criterion name too long", mutate(func(in *domain.RubricCreate) { in.Criteria[0].Name = refOf(long(domain.MaxCriterionNameRunes + 1)) }), apperror.CodeValidation},
		{"blank scoring guide", mutate(func(in *domain.RubricCreate) { in.Criteria[0].ScoringGuide = refOf("\n\t ") }), apperror.CodeValidation},
		{"scoring guide too long", mutate(func(in *domain.RubricCreate) {
			in.Criteria[0].ScoringGuide = refOf(long(domain.MaxScoringGuideRunes + 1))
		}), apperror.CodeValidation},
		{"invalid UTF-8", mutate(func(in *domain.RubricCreate) { in.Criteria[0].ScoringGuide = refOf("0 = \xff\xfe") }), apperror.CodeValidation},
		{"control character in a name", mutate(func(in *domain.RubricCreate) { in.Name = refOf("bad\x00name") }), apperror.CodeValidation},
		{"an id on a new rubric", mutate(func(in *domain.RubricCreate) { in.Criteria[0].ID = refOf(uuid.New()) }), apperror.CodeValidation},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := newFakeRubricRepository()
			_, err := newRubrics(t, repo).Create(context.Background(), rbRecruiter, "recruiter", tc.in)
			wantCode(t, err, tc.code)
			if len(repo.created) != 0 {
				t.Fatal("an invalid rubric reached the repository")
			}
		})
	}
	// Limits are inclusive.
	repo := newFakeRubricRepository()
	in := validCreate()
	in.Name = refOf(long(domain.MaxRubricNameRunes))
	in.Criteria = nil
	for i := 0; i < domain.MaxRubricCriteria; i++ {
		c := validCriterion()
		c.MaxPoints = refOf(int32(domain.MaxCriterionPoints))
		c.ScoringGuide = refOf(long(domain.MaxScoringGuideRunes))
		in.Criteria = append(in.Criteria, c)
	}
	if _, err := newRubrics(t, repo).Create(context.Background(), rbRecruiter, "recruiter", in); err != nil {
		t.Fatalf("a rubric at every limit was refused: %v", err)
	}
}

func TestVisibilityAndOwnershipOnRead(t *testing.T) {
	system := systemRubric()
	mine := privateRubric(rbRecruiter, 0x61)
	theirs := privateRubric(rbOther, 0x62)
	archivedSystem := systemRubric()
	archivedSystem.ID, archivedSystem.IsActive = uid(0x52), false
	svc := newRubrics(t, newFakeRubricRepository(system, mine, theirs, archivedSystem))
	ctx := context.Background()

	got, err := svc.Get(ctx, rbRecruiter, "recruiter", system.ID)
	if err != nil || got.CanManage || got.IsOwn || got.ReferenceCount != nil || got.Locked {
		t.Fatalf("recruiter reading a system rubric: %+v, %v; want read-only without usage numbers", got, err)
	}
	got, err = svc.Get(ctx, rbRecruiter, "recruiter", mine.ID)
	if err != nil || !got.CanManage || !got.IsOwn || got.ReferenceCount == nil || *got.ReferenceCount != 2 || !got.Locked {
		t.Fatalf("recruiter reading their own rubric: %+v, %v; want manageable, 2 references, locked", got, err)
	}
	got, err = svc.Get(ctx, rbAdmin, "admin", system.ID)
	if err != nil || !got.CanManage || got.ReferenceCount == nil || *got.ReferenceCount != 3 {
		t.Fatalf("admin reading a system rubric: %+v, %v; want manageable with 3 references", got, err)
	}

	// A guessed UUID of someone else's private rubric, and an archived system one, look like nothing.
	for name, call := range map[string]func() error{
		"recruiter reading a foreign private rubric": func() error { _, err := svc.Get(ctx, rbRecruiter, "recruiter", theirs.ID); return err },
		"recruiter reading an archived system rubric": func() error {
			_, err := svc.Get(ctx, rbRecruiter, "recruiter", archivedSystem.ID)
			return err
		},
		"admin reading a recruiter's private rubric": func() error { _, err := svc.Get(ctx, rbAdmin, "admin", mine.ID); return err },
		"recruiter copying a foreign private rubric": func() error {
			_, err := svc.Copy(ctx, rbRecruiter, "recruiter", theirs.ID, nil)
			return err
		},
		"recruiter editing a foreign private rubric": func() error {
			_, err := svc.Update(ctx, rbRecruiter, "recruiter", theirs.ID, domain.RubricPatch{UpdatedAt: refOf(theirs.UpdatedAt), IsActive: refOf(false)})
			return err
		},
		"recruiter deleting a foreign private rubric": func() error { return svc.Delete(ctx, rbRecruiter, "recruiter", theirs.ID) },
	} {
		t.Run(name, func(t *testing.T) { wantCode(t, call(), apperror.CodeRubricNotFound) })
	}
}

func TestOnlyTheRightPrincipalCanChangeARubric(t *testing.T) {
	system := systemRubric()
	mine := privateRubric(rbRecruiter, 0x61)
	repo := newFakeRubricRepository(system, mine)
	svc := newRubrics(t, repo)
	ctx := context.Background()
	patch := func(r domain.Rubric) domain.RubricPatch {
		return domain.RubricPatch{UpdatedAt: refOf(r.UpdatedAt), Name: refOf("Renamed")}
	}

	// A Recruiter can see a system rubric but never change it: 403, not 404.
	_, err := svc.Update(ctx, rbRecruiter, "recruiter", system.ID, patch(system))
	wantCode(t, err, apperror.CodeForbidden)
	wantCode(t, svc.Delete(ctx, rbRecruiter, "recruiter", system.ID), apperror.CodeForbidden)
	if len(repo.updates)+len(repo.deleted) != 0 {
		t.Fatal("the repository was reached for a change the caller may not make")
	}

	if _, err := svc.Update(ctx, rbAdmin, "admin", system.ID, patch(system)); err != nil {
		t.Fatalf("admin updating a system rubric: %v", err)
	}
	if _, err := svc.Update(ctx, rbRecruiter, "recruiter", mine.ID, patch(mine)); err != nil {
		t.Fatalf("owner updating their rubric: %v", err)
	}
	if err := svc.Delete(ctx, rbAdmin, "admin", system.ID); err != nil {
		t.Fatalf("admin deleting a system rubric: %v", err)
	}
	if err := svc.Delete(ctx, rbRecruiter, "recruiter", mine.ID); err != nil {
		t.Fatalf("owner deleting their rubric: %v", err)
	}
	if len(repo.updates) != 2 || len(repo.deleted) != 2 {
		t.Fatalf("updates = %d, deletes = %d, want 2 and 2", len(repo.updates), len(repo.deleted))
	}
}

func TestUpdateValidationAndConcurrencyToken(t *testing.T) {
	mine := privateRubric(rbRecruiter, 0x61)
	repo := newFakeRubricRepository(mine)
	svc := newRubrics(t, repo)
	ctx := context.Background()
	update := func(p domain.RubricPatch) error {
		_, err := svc.Update(ctx, rbRecruiter, "recruiter", mine.ID, p)
		return err
	}

	wantCode(t, update(domain.RubricPatch{Name: refOf("x")}), apperror.CodeValidation)                 // no updated_at
	wantCode(t, update(domain.RubricPatch{UpdatedAt: refOf(mine.UpdatedAt)}), apperror.CodeValidation) // nothing to change
	wantCode(t, update(domain.RubricPatch{UpdatedAt: refOf(mine.UpdatedAt), Name: refOf("  ")}), apperror.CodeValidation)
	wantCode(t, update(domain.RubricPatch{UpdatedAt: refOf(mine.UpdatedAt), Criteria: &[]domain.CriterionInput{}}), apperror.CodeValidation)
	dup := uuid.New()
	a, b := validCriterion(), validCriterion()
	a.ID, b.ID = &dup, &dup
	wantCode(t, update(domain.RubricPatch{UpdatedAt: refOf(mine.UpdatedAt), Criteria: &[]domain.CriterionInput{a, b}}), apperror.CodeValidation)
	if len(repo.updates) != 0 {
		t.Fatal("an invalid update reached the repository")
	}

	// A valid criteria replacement keeps the ids the client sent and passes the token through unchanged.
	kept := uuid.New()
	a.ID, b.ID = &kept, nil
	if err := update(domain.RubricPatch{UpdatedAt: refOf(mine.UpdatedAt), Criteria: &[]domain.CriterionInput{a, b}}); err != nil {
		t.Fatal(err)
	}
	got := repo.updates[0]
	if !got.ExpectedUpdatedAt.Equal(mine.UpdatedAt) || got.Criteria == nil || len(*got.Criteria) != 2 {
		t.Fatalf("update = %+v", got)
	}
	if (*got.Criteria)[0].ID == nil || *(*got.Criteria)[0].ID != kept || (*got.Criteria)[1].ID != nil {
		t.Fatalf("criteria ids = %+v, want the kept id first and a new criterion second", *got.Criteria)
	}

	// Repository conflicts keep their stable codes.
	repo.updateErr = domain.ErrRubricStale
	wantCode(t, update(domain.RubricPatch{UpdatedAt: refOf(mine.UpdatedAt), Name: refOf("x")}), apperror.CodeRubricStale)
	repo.updateErr = domain.ErrRubricLocked
	wantCode(t, update(domain.RubricPatch{UpdatedAt: refOf(mine.UpdatedAt), Name: refOf("x")}), apperror.CodeRubricLocked)
}

func TestArchivingNeedsNoContentValidation(t *testing.T) {
	mine := privateRubric(rbRecruiter, 0x61)
	repo := newFakeRubricRepository(mine)
	if _, err := newRubrics(t, repo).Update(context.Background(), rbRecruiter, "recruiter", mine.ID,
		domain.RubricPatch{UpdatedAt: refOf(mine.UpdatedAt), IsActive: refOf(false)}); err != nil {
		t.Fatal(err)
	}
	if got := repo.updates[0]; got.IsActive == nil || *got.IsActive || got.ContentChange() {
		t.Fatalf("archive update = %+v, want only the archive flag", got)
	}
}

func TestDeleteConflictKeepsItsCode(t *testing.T) {
	mine := privateRubric(rbRecruiter, 0x61)
	repo := newFakeRubricRepository(mine)
	repo.deleteErr = domain.ErrRubricInUse
	wantCode(t, newRubrics(t, repo).Delete(context.Background(), rbRecruiter, "recruiter", mine.ID), apperror.CodeRubricInUse)
}

func TestCopyCreatesAnIndependentRubricForTheCaller(t *testing.T) {
	system := systemRubric()
	system.Name = "Explain a mechanism"
	mine := privateRubric(rbRecruiter, 0x61)
	repo := newFakeRubricRepository(system, mine)
	svc := newRubrics(t, repo)
	ctx := context.Background()

	got, err := svc.Copy(ctx, rbRecruiter, "recruiter", system.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	c := repo.copies[0]
	if c.Scope != domain.RubricScopeRecruiter || c.OwnerID == nil || *c.OwnerID != rbRecruiter || c.Name != "Copy of Explain a mechanism" {
		t.Fatalf("recruiter copy of a system rubric = %+v", c)
	}
	if !got.IsOwn || !got.CanManage {
		t.Fatalf("the copy must belong to the caller: %+v", got)
	}

	if _, err := svc.Copy(ctx, rbRecruiter, "recruiter", mine.ID, refOf("  My own name  ")); err != nil {
		t.Fatal(err)
	}
	if repo.copies[1].Name != "My own name" {
		t.Fatalf("copy name = %q, want the trimmed override", repo.copies[1].Name)
	}

	// An Admin's copy of a system rubric is another system rubric.
	if _, err := svc.Copy(ctx, rbAdmin, "admin", system.ID, nil); err != nil {
		t.Fatal(err)
	}
	if repo.copies[2].Scope != domain.RubricScopeSystem || repo.copies[2].OwnerID != nil {
		t.Fatalf("admin copy = %+v, want a system rubric", repo.copies[2])
	}

	// The default name never exceeds the limit, and a bad override is refused before anything is copied.
	system.Name = strings.Repeat("n", domain.MaxRubricNameRunes)
	repo.rubrics[system.ID] = system
	if _, err := svc.Copy(ctx, rbRecruiter, "recruiter", system.ID, nil); err != nil {
		t.Fatal(err)
	}
	if n := len([]rune(repo.copies[3].Name)); n > domain.MaxRubricNameRunes {
		t.Fatalf("default copy name has %d runes, want at most %d", n, domain.MaxRubricNameRunes)
	}
	before := len(repo.copies)
	_, err = svc.Copy(ctx, rbRecruiter, "recruiter", system.ID, refOf(" "))
	wantCode(t, err, apperror.CodeValidation)
	if len(repo.copies) != before {
		t.Fatal("a copy with an invalid name reached the repository")
	}
}

func TestListQueryValidationAndFlags(t *testing.T) {
	repo := newFakeRubricRepository(systemRubric(), privateRubric(rbRecruiter, 0x61))
	svc := newRubrics(t, repo)
	ctx := context.Background()

	items, total, err := svc.List(ctx, rbRecruiter, "recruiter", domain.RubricListQuery{})
	if err != nil || total != 2 || len(items) != 2 {
		t.Fatalf("List() = %d items, total %d, %v", len(items), total, err)
	}
	if q := repo.listed[0]; q.Status != domain.RubricStatusActive || q.Limit != 20 {
		t.Fatalf("defaults = %+v, want active and limit 20", q)
	}
	for _, item := range items {
		if item.IsOwn != item.CanManage || (item.Scope == domain.RubricScopeSystem && item.ReferenceCount != nil) {
			t.Fatalf("item flags = %+v", item)
		}
	}
	for _, bad := range []domain.RubricListQuery{
		{Scope: "everyone"}, {Status: "deleted"}, {Limit: 101}, {Limit: -1}, {Offset: -1}, {QuestionType: "Not A Slug"},
	} {
		_, _, err := svc.List(ctx, rbRecruiter, "recruiter", bad)
		wantCode(t, err, apperror.CodeValidation)
	}
}
