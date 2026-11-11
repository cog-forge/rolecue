//go:build integration

package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	authdomain "github.com/cog-forge/rolecue/api/internal/features/auth/domain"
	"github.com/cog-forge/rolecue/api/internal/features/evaluation/domain"
	"github.com/cog-forge/rolecue/api/internal/features/evaluation/repository"
	"github.com/cog-forge/rolecue/api/internal/features/evaluation/service"
	"github.com/cog-forge/rolecue/api/internal/handler"
	"github.com/cog-forge/rolecue/api/internal/middleware"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// The rubric API end to end below the auth layer: real handler, real role middleware, real service, real
// repository and PostgreSQL. Only the logged-in user is injected (the key RequireAuth would set).

var seededMechanismRubric = uuid.MustParse("00000000-0000-4000-8000-000000000201")

type rubricAPI struct {
	t      *testing.T
	engine *gin.Engine
	seed   *seed
}

func newRubricAPI(t *testing.T, ctx context.Context, pool *pgxpool.Pool) (*rubricAPI, *seed) {
	t.Helper()
	svc, err := service.NewRubrics(repository.NewRepository(pool))
	if err != nil {
		t.Fatal(err)
	}
	h := handler.NewRubricHandler(svc)
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(func(c *gin.Context) {
		if raw := c.GetHeader("X-Test-User"); raw != "" {
			var user authdomain.User
			if err := json.Unmarshal([]byte(raw), &user); err == nil {
				c.Set("auth.middleware.currentUser", user)
			}
		}
		c.Next()
	})
	group := engine.Group("/rubrics", middleware.RequireRole("admin", "recruiter"))
	group.GET("", h.List)
	group.POST("", h.Create)
	group.GET("/:id", h.Get)
	group.PATCH("/:id", h.Update)
	group.DELETE("/:id", h.Delete)
	group.POST("/:id/copies", h.Copy)
	s := newSeed(t, ctx, pool)
	return &rubricAPI{t: t, engine: engine, seed: s}, s
}

type apiResult struct {
	status int
	body   map[string]any
}

func (r apiResult) data() map[string]any {
	d, _ := r.body["data"].(map[string]any)
	return d
}

func (r apiResult) code() string {
	e, _ := r.body["error"].(map[string]any)
	code, _ := e["code"].(string)
	return code
}

func (a *rubricAPI) call(user authdomain.User, method, path string, body any) apiResult {
	a.t.Helper()
	var reader *bytes.Reader
	if body == nil {
		reader = bytes.NewReader(nil)
	} else {
		raw, err := json.Marshal(body)
		if err != nil {
			a.t.Fatal(err)
		}
		reader = bytes.NewReader(raw)
	}
	req := httptest.NewRequest(method, path, reader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	rawUser, _ := json.Marshal(user)
	req.Header.Set("X-Test-User", string(rawUser))
	recorder := httptest.NewRecorder()
	a.engine.ServeHTTP(recorder, req)
	out := apiResult{status: recorder.Code}
	_ = json.Unmarshal(recorder.Body.Bytes(), &out.body)
	return out
}

func (a *rubricAPI) mustCall(user authdomain.User, method, path string, body any, wantStatus int) apiResult {
	a.t.Helper()
	res := a.call(user, method, path, body)
	if res.status != wantStatus {
		a.t.Fatalf("%s %s = %d %v, want %d", method, path, res.status, res.body, wantStatus)
	}
	return res
}

// track removes a rubric created through the API when the test ends.
func (a *rubricAPI) track(res apiResult) string {
	id, _ := res.data()["id"].(string)
	a.seed.later(`DELETE FROM public.rubrics WHERE id = $1::uuid`, id)
	return id
}

func rubricBody(name string) map[string]any {
	return map[string]any{
		"name": name, "description": "d", "question_type": "concept_explanation",
		"criteria": []map[string]any{
			{"metric_id": accuracyID.String(), "name": "Mechanism", "max_points": 2, "scoring_guide": "0 = no; 1 = partly; 2 = fully"},
			{"metric_id": depthID.String(), "name": "Limits", "max_points": 3, "scoring_guide": "0 = no; 1 = a bit; 2 = most; 3 = all"},
		},
	}
}

func user(id uuid.UUID, role string) authdomain.User { return authdomain.User{ID: id, Role: role} }

func criteriaOf(res apiResult) []map[string]any {
	raw, _ := res.data()["criteria"].([]any)
	out := make([]map[string]any, 0, len(raw))
	for _, item := range raw {
		out = append(out, item.(map[string]any))
	}
	return out
}

func TestRubricAPICandidatesAreRefusedAndScopeIsNotChosenByTheClient(t *testing.T) {
	ctx, pool := openPool(t)
	api, s := newRubricAPI(t, ctx, pool)
	recruiter := s.recruiter()

	if res := api.call(user(uuid.New(), "candidate"), http.MethodGet, "/rubrics", nil); res.status != http.StatusForbidden {
		t.Fatalf("candidate list = %d, want 403", res.status)
	}
	// A body that names a scope or an owner is refused, not ignored.
	body := rubricBody("scope smuggling")
	body["scope"] = "system"
	if res := api.call(user(recruiter, "recruiter"), http.MethodPost, "/rubrics", body); res.status != http.StatusBadRequest {
		t.Fatalf("create with a scope = %d %v, want 400", res.status, res.body)
	}
	res := api.mustCall(user(recruiter, "recruiter"), http.MethodPost, "/rubrics", rubricBody("Private one"), http.StatusCreated)
	api.track(res)
	if res.data()["scope"] != "recruiter" || res.data()["is_own"] != true || res.data()["can_manage"] != true {
		t.Fatalf("created rubric = %v, want a private rubric the caller owns", res.data())
	}
	if n := len(criteriaOf(res)); n != 2 {
		t.Fatalf("criteria = %d, want 2", n)
	}
}

func TestRubricAPIForeignPrivateRubricIsInvisibleEvenWithItsUUID(t *testing.T) {
	ctx, pool := openPool(t)
	api, s := newRubricAPI(t, ctx, pool)
	owner, intruder := s.recruiter(), s.recruiter()
	created := api.mustCall(user(owner, "recruiter"), http.MethodPost, "/rubrics", rubricBody("Mine alone"), http.StatusCreated)
	id := api.track(created)
	path := "/rubrics/" + id
	updatedAt := created.data()["updated_at"]

	for name, call := range map[string]func() apiResult{
		"read": func() apiResult { return api.call(user(intruder, "recruiter"), http.MethodGet, path, nil) },
		"copy": func() apiResult { return api.call(user(intruder, "recruiter"), http.MethodPost, path+"/copies", nil) },
		"edit": func() apiResult {
			return api.call(user(intruder, "recruiter"), http.MethodPatch, path, map[string]any{"updated_at": updatedAt, "is_active": false})
		},
		"delete": func() apiResult { return api.call(user(intruder, "recruiter"), http.MethodDelete, path, nil) },
		"admin read": func() apiResult {
			return api.call(user(uuid.New(), "admin"), http.MethodGet, path, nil)
		},
		"admin edit": func() apiResult {
			return api.call(user(uuid.New(), "admin"), http.MethodPatch, path, map[string]any{"updated_at": updatedAt, "is_active": false})
		},
	} {
		t.Run(name, func(t *testing.T) {
			if res := call(); res.status != http.StatusNotFound || res.code() != string(apperror.CodeRubricNotFound) {
				t.Fatalf("status = %d code = %q, want 404 RUBRIC_NOT_FOUND (%v)", res.status, res.code(), res.body)
			}
		})
	}
	// It is also absent from the other recruiter's list, and from the admin's.
	for _, viewer := range []authdomain.User{user(intruder, "recruiter"), user(uuid.New(), "admin")} {
		list := api.mustCall(viewer, http.MethodGet, "/rubrics?limit=100&status=all", nil, http.StatusOK)
		items, _ := list.data()["items"].([]any)
		for _, item := range items {
			if item.(map[string]any)["id"] == id {
				t.Fatalf("%s sees someone else's private rubric in the list", viewer.Role)
			}
		}
	}
	// The unchanged rubric is intact for its owner.
	if res := api.mustCall(user(owner, "recruiter"), http.MethodGet, path, nil, http.StatusOK); res.data()["is_active"] != true {
		t.Fatalf("owner's rubric changed: %v", res.data())
	}
	// A malformed id looks the same as an unknown one.
	if res := api.call(user(owner, "recruiter"), http.MethodGet, "/rubrics/not-a-uuid", nil); res.status != http.StatusNotFound {
		t.Fatalf("malformed id = %d, want 404", res.status)
	}
}

func TestRubricAPISystemRubricsAreReadOnlyForRecruiters(t *testing.T) {
	ctx, pool := openPool(t)
	api, s := newRubricAPI(t, ctx, pool)
	recruiter := user(s.recruiter(), "recruiter")
	path := "/rubrics/" + seededMechanismRubric.String()

	read := api.mustCall(recruiter, http.MethodGet, path, nil, http.StatusOK)
	if read.data()["can_manage"] != false || read.data()["reference_count"] != nil {
		t.Fatalf("recruiter view of a system rubric = %v, want read-only without a usage count", read.data())
	}
	updatedAt := read.data()["updated_at"]
	if res := api.call(recruiter, http.MethodPatch, path, map[string]any{"updated_at": updatedAt, "name": "hijack"}); res.status != http.StatusForbidden {
		t.Fatalf("recruiter edit of a system rubric = %d, want 403", res.status)
	}
	if res := api.call(recruiter, http.MethodDelete, path, nil); res.status != http.StatusForbidden {
		t.Fatalf("recruiter delete of a system rubric = %d, want 403", res.status)
	}
	// An Admin manages it (archive and restore, which leaves the seed as it was).
	admin := user(uuid.New(), "admin")
	archived := api.mustCall(admin, http.MethodPatch, path, map[string]any{"updated_at": updatedAt, "is_active": false}, http.StatusOK)
	s.later(`UPDATE public.rubrics SET is_active = true WHERE id = $1`, seededMechanismRubric)
	if archived.data()["is_active"] != false {
		t.Fatalf("archived = %v", archived.data())
	}
	// Now the Recruiter no longer sees it at all.
	if res := api.call(recruiter, http.MethodGet, path, nil); res.status != http.StatusNotFound {
		t.Fatalf("recruiter read of an archived system rubric = %d, want 404", res.status)
	}
	api.mustCall(admin, http.MethodPatch, path, map[string]any{"updated_at": archived.data()["updated_at"], "is_active": true}, http.StatusOK)
}

func TestRubricAPICopyIsIndependentOfItsSource(t *testing.T) {
	ctx, pool := openPool(t)
	api, s := newRubricAPI(t, ctx, pool)
	recruiter := user(s.recruiter(), "recruiter")
	sourcePath := "/rubrics/" + seededMechanismRubric.String()
	source := api.mustCall(recruiter, http.MethodGet, sourcePath, nil, http.StatusOK)

	copied := api.mustCall(recruiter, http.MethodPost, sourcePath+"/copies", map[string]any{"name": "My version"}, http.StatusCreated)
	copyID := api.track(copied)
	if copied.data()["scope"] != "recruiter" || copied.data()["is_own"] != true || copied.data()["name"] != "My version" {
		t.Fatalf("copy = %v", copied.data())
	}
	srcCriteria, copyCriteria := criteriaOf(source), criteriaOf(copied)
	if len(copyCriteria) != len(srcCriteria) || len(copyCriteria) == 0 {
		t.Fatalf("copy has %d criteria, source %d", len(copyCriteria), len(srcCriteria))
	}
	for i := range copyCriteria {
		if copyCriteria[i]["id"] == srcCriteria[i]["id"] {
			t.Fatalf("criterion %d shares its id with the source: the copy must own its rows", i)
		}
		for _, field := range []string{"metric_id", "name", "max_points", "scoring_guide", "position"} {
			if copyCriteria[i][field] != srcCriteria[i][field] {
				t.Fatalf("criterion %d %s = %v, source %v", i, field, copyCriteria[i][field], srcCriteria[i][field])
			}
		}
	}

	// Editing the copy (rename, change points, add a criterion) leaves the source exactly as it was.
	edited := make([]map[string]any, 0, len(copyCriteria)+1)
	for i, c := range copyCriteria {
		entry := map[string]any{"id": c["id"], "metric_id": c["metric_id"], "name": c["name"], "max_points": c["max_points"], "scoring_guide": c["scoring_guide"]}
		if i == 0 {
			entry["name"], entry["max_points"], entry["scoring_guide"] = "Edited", 1, "0 = no; 1 = yes"
		}
		edited = append(edited, entry)
	}
	edited = append(edited, map[string]any{"metric_id": accuracyID.String(), "name": "Brand new", "max_points": 4, "scoring_guide": "0 = a; 1 = b; 2 = c; 3 = d; 4 = e"})
	updated := api.mustCall(recruiter, http.MethodPatch, "/rubrics/"+copyID,
		map[string]any{"updated_at": copied.data()["updated_at"], "name": "Renamed copy", "criteria": edited}, http.StatusOK)
	if got := criteriaOf(updated); len(got) != len(srcCriteria)+1 || got[0]["name"] != "Edited" || got[0]["id"] != copyCriteria[0]["id"] {
		t.Fatalf("edited copy criteria = %v", got)
	}
	after := api.mustCall(recruiter, http.MethodGet, sourcePath, nil, http.StatusOK)
	if after.data()["name"] != source.data()["name"] || after.data()["updated_at"] != source.data()["updated_at"] {
		t.Fatalf("source changed after editing its copy: %v", after.data())
	}
	afterCriteria := criteriaOf(after)
	for i := range srcCriteria {
		for _, field := range []string{"id", "name", "max_points", "scoring_guide", "position"} {
			if afterCriteria[i][field] != srcCriteria[i][field] {
				t.Fatalf("source criterion %d %s changed to %v", i, field, afterCriteria[i][field])
			}
		}
	}
}

func TestRubricAPIListFiltersAndPaging(t *testing.T) {
	ctx, pool := openPool(t)
	api, s := newRubricAPI(t, ctx, pool)
	owner := user(s.recruiter(), "recruiter")
	for _, name := range []string{"B list", "A list", "C list"} {
		api.track(api.mustCall(owner, http.MethodPost, "/rubrics", rubricBody(name), http.StatusCreated))
	}
	own := api.mustCall(owner, http.MethodGet, "/rubrics?scope=own&limit=2", nil, http.StatusOK)
	items, _ := own.data()["items"].([]any)
	pagination, _ := own.data()["pagination"].(map[string]any)
	if len(items) != 2 || pagination["total"] != float64(3) || pagination["limit"] != float64(2) {
		t.Fatalf("own page 1 = %d items, pagination %v, want 2 of 3", len(items), pagination)
	}
	if first := items[0].(map[string]any); first["name"] != "A list" || first["criteria"] != nil || first["criteria_count"] != float64(2) || first["max_points"] != float64(5) {
		t.Fatalf("first item = %v, want A list without criteria, 2 criteria worth 5 points", first)
	}
	rest := api.mustCall(owner, http.MethodGet, "/rubrics?scope=own&limit=2&offset=2", nil, http.StatusOK)
	if got, _ := rest.data()["items"].([]any); len(got) != 1 {
		t.Fatalf("own page 2 = %d items, want 1", len(got))
	}
	system := api.mustCall(owner, http.MethodGet, "/rubrics?scope=system&limit=100", nil, http.StatusOK)
	seeded, _ := system.data()["items"].([]any)
	if len(seeded) < 4 {
		t.Fatalf("system list = %d, want the seeded templates", len(seeded))
	}
	for _, item := range seeded {
		if item.(map[string]any)["scope"] != "system" {
			t.Fatalf("scope=system returned %v", item)
		}
	}
	byType := api.mustCall(owner, http.MethodGet, "/rubrics?scope=system&question_type=comparison_choice", nil, http.StatusOK)
	if got, _ := byType.data()["items"].([]any); len(got) < 1 {
		t.Fatal("question_type filter found nothing")
	}
	for _, query := range []string{"?limit=0", "?limit=101", "?offset=-1", "?status=gone", "?scope=all", "?limit=1&limit=2"} {
		if res := api.call(owner, http.MethodGet, "/rubrics"+query, nil); res.status != http.StatusBadRequest {
			t.Fatalf("GET /rubrics%s = %d, want 400", query, res.status)
		}
	}
}

func TestRubricAPIStaleEditIsRefusedAndNeverOverwrites(t *testing.T) {
	ctx, pool := openPool(t)
	api, s := newRubricAPI(t, ctx, pool)
	owner := user(s.recruiter(), "recruiter")
	created := api.mustCall(owner, http.MethodPost, "/rubrics", rubricBody("Race"), http.StatusCreated)
	path := "/rubrics/" + api.track(created)
	token := created.data()["updated_at"]

	first := api.mustCall(owner, http.MethodPatch, path, map[string]any{"updated_at": token, "name": "First writer"}, http.StatusOK)
	if first.data()["updated_at"] == token {
		t.Fatal("updated_at did not move after an edit")
	}
	second := api.call(owner, http.MethodPatch, path, map[string]any{"updated_at": token, "name": "Second writer"})
	if second.status != http.StatusConflict || second.code() != string(apperror.CodeRubricStale) {
		t.Fatalf("edit with the old token = %d %q, want 409 RUBRIC_STALE", second.status, second.code())
	}
	if got := api.mustCall(owner, http.MethodGet, path, nil, http.StatusOK); got.data()["name"] != "First writer" {
		t.Fatalf("name = %v, the stale write must not land", got.data()["name"])
	}
	// The token is required.
	if res := api.call(owner, http.MethodPatch, path, map[string]any{"name": "No token"}); res.status != http.StatusBadRequest {
		t.Fatalf("edit without updated_at = %d, want 400", res.status)
	}
	// Archiving needs the token too.
	if res := api.call(owner, http.MethodPatch, path, map[string]any{"updated_at": token, "is_active": false}); res.status != http.StatusConflict {
		t.Fatalf("archive with the old token = %d, want 409", res.status)
	}
}

func TestRubricAPIConcurrentEditsOnTheSameTokenHaveOneWinner(t *testing.T) {
	ctx, pool := openPool(t)
	api, s := newRubricAPI(t, ctx, pool)
	owner := user(s.recruiter(), "recruiter")
	created := api.mustCall(owner, http.MethodPost, "/rubrics", rubricBody("Many writers"), http.StatusCreated)
	path := "/rubrics/" + api.track(created)
	token := created.data()["updated_at"]

	const writers = 8
	results := make([]apiResult, writers)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < writers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			results[i] = api.call(owner, http.MethodPatch, path, map[string]any{"updated_at": token, "description": "writer " + string(rune('a'+i))})
		}(i)
	}
	close(start)
	wg.Wait()
	won, stale := 0, 0
	for _, res := range results {
		switch {
		case res.status == http.StatusOK:
			won++
		case res.status == http.StatusConflict && res.code() == string(apperror.CodeRubricStale):
			stale++
		default:
			t.Fatalf("unexpected result %d %v", res.status, res.body)
		}
	}
	if won != 1 || stale != writers-1 {
		t.Fatalf("winners = %d, stale = %d, want 1 and %d", won, stale, writers-1)
	}
}

func TestRubricAPIReferencedRubricIsLockedButCanBeArchived(t *testing.T) {
	ctx, pool := openPool(t)
	api, s := newRubricAPI(t, ctx, pool)
	ownerID := s.recruiter()
	owner := user(ownerID, "recruiter")
	created := api.mustCall(owner, http.MethodPost, "/rubrics", rubricBody("In use"), http.StatusCreated)
	id := api.track(created)
	path := "/rubrics/" + id
	_, jdID := s.posting(ownerID, "pending")
	questionID := uuid.New()
	s.exec(`INSERT INTO public.core_questions (id, jd_id, content, rubric_id) VALUES ($1, $2, 'uses the rubric', $3::uuid)`, questionID, jdID, id)
	s.later(`DELETE FROM public.core_questions WHERE id = $1`, questionID)

	view := api.mustCall(owner, http.MethodGet, path, nil, http.StatusOK)
	if view.data()["locked"] != true || view.data()["reference_count"] != float64(1) {
		t.Fatalf("view = %v, want locked with 1 reference", view.data())
	}
	token := view.data()["updated_at"]
	for name, body := range map[string]map[string]any{
		"rename":        {"name": "nope"},
		"change points": {"criteria": []map[string]any{{"metric_id": accuracyID.String(), "name": "x", "max_points": 5, "scoring_guide": "0 = a"}}},
		"description":   {"description": "nope"},
		"question type": {"question_type": "other_type"},
	} {
		t.Run("locked "+name, func(t *testing.T) {
			body["updated_at"] = token
			res := api.call(owner, http.MethodPatch, path, body)
			if res.status != http.StatusConflict || res.code() != string(apperror.CodeRubricLocked) {
				t.Fatalf("status = %d code = %q, want 409 RUBRIC_LOCKED", res.status, res.code())
			}
		})
	}
	if res := api.call(owner, http.MethodDelete, path, nil); res.status != http.StatusConflict || res.code() != string(apperror.CodeRubricInUse) {
		t.Fatalf("delete of a referenced rubric = %d %q, want 409 RUBRIC_IN_USE", res.status, res.code())
	}

	// Archiving is not a content change: it works, keeps the reference and stops new copies by others.
	archived := api.mustCall(owner, http.MethodPatch, path, map[string]any{"updated_at": token, "is_active": false}, http.StatusOK)
	if archived.data()["is_active"] != false || archived.data()["reference_count"] != float64(1) {
		t.Fatalf("archived = %v", archived.data())
	}
	var stored uuid.UUID
	if err := pool.QueryRow(ctx, `SELECT rubric_id FROM public.core_questions WHERE id = $1`, questionID).Scan(&stored); err != nil || stored.String() != id {
		t.Fatalf("the question lost its rubric after archiving: %v, %v", stored, err)
	}
	// A copy is the way to change a locked rubric.
	copied := api.mustCall(owner, http.MethodPost, path+"/copies", nil, http.StatusCreated)
	api.track(copied)
	api.mustCall(owner, http.MethodPatch, "/rubrics/"+copied.data()["id"].(string), map[string]any{"updated_at": copied.data()["updated_at"], "name": "Editable copy"}, http.StatusOK)

	// Once nothing references it any more, it can be deleted (and is then gone).
	s.exec(`UPDATE public.core_questions SET rubric_id = NULL WHERE id = $1`, questionID)
	api.mustCall(owner, http.MethodDelete, path, nil, http.StatusNoContent)
	if res := api.call(owner, http.MethodGet, path, nil); res.status != http.StatusNotFound {
		t.Fatalf("read after delete = %d, want 404", res.status)
	}
	var criteria int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM public.rubric_criteria WHERE rubric_id = $1::uuid`, id).Scan(&criteria); err != nil || criteria != 0 {
		t.Fatalf("criteria left after delete = %d, %v", criteria, err)
	}
}

func TestRubricAPIDeleteWaitsForAQuestionThatIsBeingAssigned(t *testing.T) {
	ctx, pool := openPool(t)
	api, s := newRubricAPI(t, ctx, pool)
	ownerID := s.recruiter()
	owner := user(ownerID, "recruiter")
	created := api.mustCall(owner, http.MethodPost, "/rubrics", rubricBody("Being assigned"), http.StatusCreated)
	id := api.track(created)
	_, jdID := s.posting(ownerID, "pending")

	// A question points at the rubric in a transaction that has not committed yet.
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	questionID := uuid.New()
	if _, err := tx.Exec(ctx, `INSERT INTO public.core_questions (id, jd_id, content, rubric_id) VALUES ($1, $2, 'being assigned', $3::uuid)`, questionID, jdID, id); err != nil {
		t.Fatal(err)
	}
	s.later(`DELETE FROM public.core_questions WHERE id = $1`, questionID)

	done := make(chan apiResult, 1)
	go func() { done <- api.call(owner, http.MethodDelete, "/rubrics/"+id, nil) }()
	select {
	case res := <-done:
		t.Fatalf("delete finished (%d) while the assignment was still uncommitted; it must wait for the row lock", res.status)
	case <-time.After(400 * time.Millisecond):
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	res := <-done
	if res.status != http.StatusConflict || res.code() != string(apperror.CodeRubricInUse) {
		t.Fatalf("delete after the assignment committed = %d %q, want 409 RUBRIC_IN_USE", res.status, res.code())
	}
	api.mustCall(owner, http.MethodGet, "/rubrics/"+id, nil, http.StatusOK)
}

func TestRubricAPICriteriaSetsKeepIdsReorderAndRejectForeignIds(t *testing.T) {
	ctx, pool := openPool(t)
	api, s := newRubricAPI(t, ctx, pool)
	owner := user(s.recruiter(), "recruiter")
	created := api.mustCall(owner, http.MethodPost, "/rubrics", rubricBody("Reorder"), http.StatusCreated)
	path := "/rubrics/" + api.track(created)
	original := criteriaOf(created)

	// Swap the two criteria: ids are kept, positions follow the order sent.
	swapped := []map[string]any{keepCriterion(original[1]), keepCriterion(original[0])}
	res := api.mustCall(owner, http.MethodPatch, path, map[string]any{"updated_at": created.data()["updated_at"], "criteria": swapped}, http.StatusOK)
	got := criteriaOf(res)
	if got[0]["id"] != original[1]["id"] || got[1]["id"] != original[0]["id"] || got[0]["position"] != float64(1) || got[1]["position"] != float64(2) {
		t.Fatalf("after the swap criteria = %v", got)
	}

	// Leaving one out removes it.
	res = api.mustCall(owner, http.MethodPatch, path, map[string]any{"updated_at": res.data()["updated_at"], "criteria": []map[string]any{keepCriterion(got[0])}}, http.StatusOK)
	if left := criteriaOf(res); len(left) != 1 || left[0]["id"] != got[0]["id"] {
		t.Fatalf("after removing one, criteria = %v", left)
	}

	// An id from another rubric is refused and nothing changes (the transaction rolls back).
	foreign := criteriaOf(api.mustCall(owner, http.MethodGet, "/rubrics/"+seededMechanismRubric.String(), nil, http.StatusOK))[0]
	bad := api.call(owner, http.MethodPatch, path, map[string]any{"updated_at": res.data()["updated_at"], "criteria": []map[string]any{keepCriterion(foreign)}})
	if bad.status != http.StatusBadRequest {
		t.Fatalf("foreign criterion id = %d %v, want 400", bad.status, bad.body)
	}
	if again := criteriaOf(api.mustCall(owner, http.MethodGet, path, nil, http.StatusOK)); len(again) != 1 || again[0]["id"] != got[0]["id"] {
		t.Fatalf("a refused update changed the criteria: %v", again)
	}
	if seededAgain := criteriaOf(api.mustCall(owner, http.MethodGet, "/rubrics/"+seededMechanismRubric.String(), nil, http.StatusOK)); seededAgain[0]["id"] != foreign["id"] {
		t.Fatal("the foreign criterion was touched")
	}
}

func keepCriterion(c map[string]any) map[string]any {
	return map[string]any{"id": c["id"], "metric_id": c["metric_id"], "name": c["name"], "max_points": c["max_points"], "scoring_guide": c["scoring_guide"]}
}

func TestRubricAPIServerSideValidation(t *testing.T) {
	ctx, pool := openPool(t)
	api, s := newRubricAPI(t, ctx, pool)
	owner := user(s.recruiter(), "recruiter")
	criterion := func(mutate func(map[string]any)) map[string]any {
		body := rubricBody("Validation")
		first := body["criteria"].([]map[string]any)[0]
		mutate(first)
		body["criteria"] = []map[string]any{first}
		return body
	}
	cases := map[string]struct {
		body map[string]any
		code string
	}{
		"zero points":      {criterion(func(c map[string]any) { c["max_points"] = 0 }), string(apperror.CodeValidation)},
		"fractional":       {criterion(func(c map[string]any) { c["max_points"] = 1.5 }), string(apperror.CodeValidation)},
		"unknown metric":   {criterion(func(c map[string]any) { c["metric_id"] = uuid.NewString() }), string(apperror.CodeEvaluationMetricUnknown)},
		"blank guide":      {criterion(func(c map[string]any) { c["scoring_guide"] = "   " }), string(apperror.CodeValidation)},
		"missing guide":    {criterion(func(c map[string]any) { delete(c, "scoring_guide") }), string(apperror.CodeValidation)},
		"unknown property": {criterion(func(c map[string]any) { c["weight"] = 3 }), string(apperror.CodeValidation)},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			res := api.call(owner, http.MethodPost, "/rubrics", tc.body)
			if res.status != http.StatusBadRequest || res.code() != tc.code {
				t.Fatalf("status = %d code = %q, want 400 %s (%v)", res.status, res.code(), tc.code, res.body)
			}
		})
	}
	var leaked int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM public.rubrics WHERE name = 'Validation'`).Scan(&leaked); err != nil || leaked != 0 {
		t.Fatalf("rubrics stored from invalid requests = %d, %v", leaked, err)
	}
}

func TestRubricRepositoryCreateAndCopyAreAtomic(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	repo := repository.NewRepository(pool)
	owner := s.recruiter()
	count := func(name string) int {
		var n int
		if err := pool.QueryRow(ctx, `SELECT count(*) FROM public.rubrics WHERE name = $1`, name).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}

	// The second criterion names a metric that does not exist: the rubric row written before it rolls back.
	_, err := repo.CreateRubric(ctx, domain.RubricWrite{
		Name: "atomic create", QuestionType: "concept_explanation", Scope: domain.RubricScopeRecruiter, OwnerID: &owner,
		Criteria: []domain.CriterionWrite{
			{MetricID: accuracyID, Name: "ok", MaxPoints: 1, ScoringGuide: "0 = a; 1 = b"},
			{MetricID: uuid.New(), Name: "bad", MaxPoints: 1, ScoringGuide: "0 = a; 1 = b"},
		},
	})
	assertCode(t, err, apperror.CodeEvaluationMetricUnknown)
	if n := count("atomic create"); n != 0 {
		t.Fatalf("rubrics left after a failed create = %d, want 0", n)
	}

	// A copy that fails halfway (here: a blank name stopped by the table's check) leaves nothing behind.
	access := domain.RubricAccess{UserID: owner, Role: domain.RubricRoleRecruiter}
	var before int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM public.rubrics WHERE owner_id = $1`, owner).Scan(&before); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.CopyRubric(ctx, access, seededMechanismRubric, domain.RubricCopy{Name: "  ", Scope: domain.RubricScopeRecruiter, OwnerID: &owner}); err == nil {
		t.Fatal("a copy with a blank name succeeded")
	}
	var after int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM public.rubrics WHERE owner_id = $1`, owner).Scan(&after); err != nil || after != before {
		t.Fatalf("rubrics owned after a failed copy = %d, before %d, %v", after, before, err)
	}
}
