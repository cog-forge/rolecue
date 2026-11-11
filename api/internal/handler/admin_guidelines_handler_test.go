package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cog-forge/rolecue/api/internal/features/admin/guidelines/domain"
	authdomain "github.com/cog-forge/rolecue/api/internal/features/auth/domain"
	interviewdomain "github.com/cog-forge/rolecue/api/internal/features/interview/domain"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type fakeGuidelines struct {
	calls    int
	role     string
	admin    uuid.UUID
	expected int32
	values   interviewdomain.BehaviorGuideline
	err      error
}

func (f *fakeGuidelines) Get(_ context.Context, role string) (domain.Settings, error) {
	f.calls++
	f.role = role
	return domain.Settings{Values: interviewdomain.DefaultBehaviorGuideline()}, f.err
}

func (f *fakeGuidelines) Save(_ context.Context, admin uuid.UUID, role string, expected int32, values interviewdomain.BehaviorGuideline) (domain.Settings, error) {
	f.calls++
	f.admin, f.role, f.expected, f.values = admin, role, expected, values
	return domain.Settings{Version: expected + 1, Values: values}, f.err
}

func callGuidelines(svc AdminGuidelinesService, method, contentType, body string, user *authdomain.User) (*httptest.ResponseRecorder, map[string]any) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(func(c *gin.Context) {
		if user != nil {
			c.Set("auth.middleware.currentUser", *user)
		}
		c.Next()
	})
	h := NewAdminGuidelinesHandler(svc)
	engine.GET("/admin/interview-guidelines", h.Get)
	engine.PUT("/admin/interview-guidelines", h.Put)
	req := httptest.NewRequest(method, "/admin/interview-guidelines", strings.NewReader(body))
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, req)
	var decoded map[string]any
	_ = json.Unmarshal(recorder.Body.Bytes(), &decoded)
	return recorder, decoded
}

const validGuidelineBody = `{"expected_version":2,"values":{"tone":"formal","question_length":"concise","follow_up_max":1,"follow_up_focus":"thorough","avoid_topics":["compensation"]}}`

func TestGuidelinesSavePassesTheAdminVersionAndValues(t *testing.T) {
	admin := uuid.New()
	svc := &fakeGuidelines{}
	recorder, _ := callGuidelines(svc, http.MethodPut, "application/json", validGuidelineBody, &authdomain.User{ID: admin, Role: "admin"})
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", recorder.Code, recorder.Body.String())
	}
	if svc.admin != admin || svc.role != "admin" || svc.expected != 2 {
		t.Fatalf("service saw admin %s role %s expected %d", svc.admin, svc.role, svc.expected)
	}
	if svc.values.Tone != "formal" || svc.values.QuestionLength != "concise" || svc.values.FollowUpFocus != "thorough" ||
		svc.values.FollowUpMax == nil || *svc.values.FollowUpMax != 1 || len(svc.values.AvoidTopics) != 1 {
		t.Fatalf("values = %+v", svc.values)
	}
}

func TestGuidelinesSaveTreatsNullFollowUpMaxAsUnset(t *testing.T) {
	svc := &fakeGuidelines{}
	body := `{"expected_version":0,"values":{"tone":"neutral","question_length":"standard","follow_up_max":null,"follow_up_focus":"standard","avoid_topics":[]}}`
	recorder, _ := callGuidelines(svc, http.MethodPut, "application/json", body, &authdomain.User{ID: uuid.New(), Role: "admin"})
	if recorder.Code != http.StatusOK || svc.values.FollowUpMax != nil {
		t.Fatalf("status = %d max = %v", recorder.Code, svc.values.FollowUpMax)
	}
}

func TestGuidelinesSaveRefusesMalformedBodiesBeforeTheService(t *testing.T) {
	admin := &authdomain.User{ID: uuid.New(), Role: "admin"}
	cases := []struct{ name, contentType, body string }{
		{"not json content type", "text/plain", validGuidelineBody},
		{"no content type", "", validGuidelineBody},
		{"empty body", "application/json", ``},
		{"not an object", "application/json", `[]`},
		{"missing expected_version", "application/json", `{"values":{"tone":"formal","question_length":"standard","follow_up_focus":"standard"}}`},
		{"missing values", "application/json", `{"expected_version":0}`},
		{"unknown top-level field", "application/json", `{"expected_version":0,"values":{},"system_prompt":"x"}`},
		{"unknown value field", "application/json", `{"expected_version":0,"values":{"tone":"formal","question_length":"standard","follow_up_focus":"standard","prompt":"be rude"}}`},
		{"text where a number belongs", "application/json", `{"expected_version":"1","values":{}}`},
		{"trailing data", "application/json", validGuidelineBody + `{}`},
		{"oversize body", "application/json", `{"expected_version":0,"values":{"tone":"` + strings.Repeat("a", 20<<10) + `"}}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc := &fakeGuidelines{}
			recorder, _ := callGuidelines(svc, http.MethodPut, tc.contentType, tc.body, admin)
			if recorder.Code != http.StatusBadRequest || svc.calls != 0 {
				t.Fatalf("status = %d, service calls = %d, want 400 and none (%s)", recorder.Code, svc.calls, recorder.Body.String())
			}
		})
	}
}

func TestGuidelinesRoutesMapServiceErrors(t *testing.T) {
	admin := &authdomain.User{ID: uuid.New(), Role: "admin"}
	for _, tc := range []struct {
		err  error
		want int
	}{
		{apperror.New(apperror.CodeForbidden, "no"), http.StatusForbidden},
		{apperror.New(apperror.CodeValidation, "tone must be one of x"), http.StatusBadRequest},
		{apperror.New(apperror.CodeGuidelineVersionConflict, "stale"), http.StatusConflict},
		{errors.New("database unavailable"), http.StatusInternalServerError},
	} {
		recorder, _ := callGuidelines(&fakeGuidelines{err: tc.err}, http.MethodPut, "application/json", validGuidelineBody, admin)
		if recorder.Code != tc.want {
			t.Errorf("save %v: status = %d, want %d", tc.err, recorder.Code, tc.want)
		}
		recorder, _ = callGuidelines(&fakeGuidelines{err: tc.err}, http.MethodGet, "", "", admin)
		if recorder.Code != tc.want {
			t.Errorf("get %v: status = %d, want %d", tc.err, recorder.Code, tc.want)
		}
	}
}

func TestGuidelinesRoutesNeedAnAuthenticatedUser(t *testing.T) {
	for _, method := range []string{http.MethodGet, http.MethodPut} {
		svc := &fakeGuidelines{}
		recorder, _ := callGuidelines(svc, method, "application/json", validGuidelineBody, nil)
		if recorder.Code != http.StatusUnauthorized || svc.calls != 0 {
			t.Fatalf("%s: status = %d, calls = %d", method, recorder.Code, svc.calls)
		}
	}
}

func TestGuidelinesGetReturnsValuesDefaultsAndAllowedVocabulary(t *testing.T) {
	svc := &fakeGuidelines{}
	recorder, body := callGuidelines(svc, http.MethodGet, "", "", &authdomain.User{ID: uuid.New(), Role: "admin"})
	if recorder.Code != http.StatusOK || svc.role != "admin" {
		t.Fatalf("status = %d role = %q", recorder.Code, svc.role)
	}
	data, _ := body["data"].(map[string]any)
	for _, key := range []string{"version", "values", "defaults", "allowed", "effective_follow_up_ceiling", "updated_at", "updated_by_name"} {
		if _, ok := data[key]; !ok {
			t.Errorf("response data lacks %q: %v", key, data)
		}
	}
	values, _ := data["values"].(map[string]any)
	for _, key := range []string{"tone", "question_length", "follow_up_max", "follow_up_focus", "avoid_topics"} {
		if _, ok := values[key]; !ok {
			t.Errorf("values lack %q: %v", key, values)
		}
	}
}
