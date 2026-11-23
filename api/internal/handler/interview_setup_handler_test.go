package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	authdomain "github.com/cog-forge/rolecue/api/internal/features/auth/domain"
	"github.com/cog-forge/rolecue/api/internal/features/interviewsetup/domain"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type setupStub struct {
	setup    domain.Setup
	err      error
	practice domain.PracticeInput
	envSeen  *string
	saves    int
}

func (s *setupStub) Practice(context.Context, uuid.UUID, string, uuid.UUID) (domain.Setup, error) {
	return s.setup, s.err
}

func (s *setupStub) SavePractice(_ context.Context, _ uuid.UUID, _ string, _ uuid.UUID, in domain.PracticeInput) (domain.Setup, error) {
	s.saves++
	s.practice = in
	return s.setup, s.err
}

func (s *setupStub) Recruitment(context.Context, uuid.UUID, string, uuid.UUID) (domain.Setup, error) {
	return s.setup, s.err
}

func (s *setupStub) SaveRecruitment(_ context.Context, _ uuid.UUID, _ string, _ uuid.UUID, env *string) (domain.Setup, error) {
	s.saves++
	s.envSeen = env
	return s.setup, s.err
}

func setupRouter(stub *setupStub) *gin.Engine {
	gin.SetMode(gin.TestMode)
	h := NewInterviewSetupHandler(stub)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("auth.middleware.currentUser", authdomain.User{ID: uuid.New(), Role: "candidate"})
	})
	r.GET("/practice/:id", h.GetPractice)
	r.PUT("/practice/:id", h.PutPractice)
	r.GET("/recruitment/:id", h.GetRecruitment)
	r.PUT("/recruitment/:id", h.PutRecruitment)
	return r
}

func setupDo(r *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(method, path, strings.NewReader(body)))
	return w
}

func TestSetupRequestsAreStrict(t *testing.T) {
	id := uuid.NewString()
	for name, tc := range map[string]struct{ path, body string }{
		"unknown practice field":         {"/practice/" + id, `{"environment":"cafe","difficulty":"hard"}`},
		"duration is not a field":        {"/practice/" + id, `{"duration_minutes":30}`},
		"locked avatar sent":             {"/recruitment/" + id, `{"environment":"cafe","avatar_id":"` + uuid.NewString() + `"}`},
		"locked voice sent":              {"/recruitment/" + id, `{"voice_id":"` + uuid.NewString() + `"}`},
		"avatar is not a uuid":           {"/practice/" + id, `{"avatar_id":"nope"}`},
		"environment is not a string":    {"/practice/" + id, `{"environment":3}`},
		"two documents":                  {"/practice/" + id, `{} {}`},
		"empty body":                     {"/practice/" + id, ``},
		"bad id in the path":             {"/practice/not-a-uuid", `{}`},
		"bad application id in the path": {"/recruitment/not-a-uuid", `{}`},
	} {
		t.Run(name, func(t *testing.T) {
			stub := &setupStub{}
			w := setupDo(setupRouter(stub), http.MethodPut, tc.path, tc.body)
			if w.Code != http.StatusBadRequest || stub.saves != 0 {
				t.Fatalf("status = %d, saves = %d, body %s", w.Code, stub.saves, w.Body.String())
			}
		})
	}
}

func TestPutPracticeMapsNullAndMissingToNotChosen(t *testing.T) {
	avatar := uuid.New()
	stub := &setupStub{}
	r := setupRouter(stub)
	id := uuid.NewString()
	body := `{"avatar_id":"` + avatar.String() + `","voice_id":null,"environment":"office-3d"}`
	if w := setupDo(r, http.MethodPut, "/practice/"+id, body); w.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", w.Code, w.Body.String())
	}
	if stub.practice.AvatarID == nil || *stub.practice.AvatarID != avatar || stub.practice.VoiceID != nil || *stub.practice.Environment != "office-3d" {
		t.Fatalf("input = %+v", stub.practice)
	}
	if w := setupDo(r, http.MethodPut, "/practice/"+id, `{}`); w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	if stub.practice.AvatarID != nil || stub.practice.VoiceID != nil || stub.practice.Environment != nil {
		t.Fatalf("a missing field means not chosen yet: %+v", stub.practice)
	}
}

func TestSetupResponseShape(t *testing.T) {
	profile, avatar := uuid.New(), uuid.New()
	updated := time.Date(2026, 11, 18, 10, 0, 0, 0, time.UTC)
	env := "cafe"
	stub := &setupStub{setup: domain.Setup{
		Mode: domain.ModePractice, RoleProfileID: &profile, TargetTitle: "Backend",
		Avatar: &domain.Choice{ID: avatar, Name: "Minh", Usable: true}, Environment: &env, LockedFields: []string{},
		Saved: true, UpdatedAt: &updated,
	}.WithBlockers([]domain.Blocker{{Code: domain.BlockerVoiceRequired, Field: "voice"}})}
	w := setupDo(setupRouter(stub), http.MethodGet, "/practice/"+uuid.NewString(), "")
	if w.Code != http.StatusOK || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("status = %d, cache = %q", w.Code, w.Header().Get("Cache-Control"))
	}
	var body struct {
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"mode", "role_profile_id", "target_title", "avatar", "voice", "environment", "locked_fields", "saved", "updated_at", "ready", "blockers"} {
		if _, ok := body.Data[field]; !ok {
			t.Errorf("response has no %q: %v", field, body.Data)
		}
	}
	if body.Data["voice"] != nil || body.Data["ready"] != false {
		t.Fatalf("voice = %v, ready = %v", body.Data["voice"], body.Data["ready"])
	}
	// Nothing about the bank, the questions or the weights ever leaves, and no difficulty or duration exists.
	for _, forbidden := range []string{"bank", "question", "weight", "difficulty", "duration", "usable", "model"} {
		if strings.Contains(strings.ToLower(w.Body.String()), forbidden) {
			t.Errorf("response mentions %q: %s", forbidden, w.Body.String())
		}
	}
}

func TestSetupErrorsKeepTheirCodes(t *testing.T) {
	for _, err := range []*apperror.AppError{domain.ErrRoleProfileNotFound, domain.ErrApplicationNotEligible, domain.ErrAvatarNotFound} {
		stub := &setupStub{err: err}
		w := setupDo(setupRouter(stub), http.MethodPut, "/practice/"+uuid.NewString(), `{}`)
		var body struct {
			Error struct {
				Code string `json:"code"`
			} `json:"error"`
		}
		if json.Unmarshal(w.Body.Bytes(), &body) != nil || body.Error.Code != string(err.Code) {
			t.Fatalf("code = %q, want %s (%s)", body.Error.Code, err.Code, w.Body.String())
		}
	}
}
