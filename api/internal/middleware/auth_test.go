package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/cog-forge/rolecue/api/internal/config"
	"github.com/cog-forge/rolecue/api/internal/features/auth"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
)

type authStub struct {
	err   error
	calls int
	id    uuid.UUID
}

func (s *authStub) Authenticate(context.Context, *http.Request) (auth.User, []string, error) {
	s.calls++
	return auth.User{ID: s.id}, []string{"rolecue.session_token=a; Path=/; HttpOnly; SameSite=Lax", "rolecue.session_data=; Path=/; HttpOnly; SameSite=Lax; Max-Age=0"}, s.err
}
func TestRequireAuth(t *testing.T) {
	for _, tc := range []struct {
		name, method, origin string
		err                  error
		status, calls        int
	}{
		{"read", "GET", "", nil, 204, 1},
		{"write", "POST", "http://localhost:3000", nil, 204, 1},
		{"missing origin", "POST", "", nil, 403, 0},
		{"sibling origin", "DELETE", "https://evil.dorriss.com", nil, 403, 0},
		{"invalid session", "GET", "", auth.ErrInvalidSession, 401, 1},
		{"unavailable", "GET", "", auth.ErrUnavailable, 503, 1},
		{"locked", "GET", "", apperror.New(apperror.CodeForbidden, "locked"), 403, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			service := &authStub{err: tc.err, id: uuid.New()}
			r := gin.New()
			r.Use(RequireAuth(service, config.CORSConfig{AllowOrigins: []string{"http://localhost:3000"}}))
			r.Any("/", func(c *gin.Context) {
				id, ok := CurrentUserID(c)
				if !ok || id != service.id {
					t.Fatal("wrong authenticated owner")
				}
				c.Status(204)
			})
			req := httptest.NewRequest(tc.method, "/?user_id="+uuid.NewString(), nil)
			req.Header.Set("Origin", tc.origin)
			req.Header.Set("user_id", uuid.NewString())
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != tc.status || service.calls != tc.calls {
				t.Fatalf("status=%d calls=%d", w.Code, service.calls)
			}
			if service.calls == 1 && len(w.Header().Values("Set-Cookie")) != 2 {
				t.Fatal("cookies were merged/lost")
			}
		})
	}
}
func TestCurrentUserID(t *testing.T) {
	if _, ok := CurrentUserID(nil); ok {
		t.Fatal("nil context has identity")
	}
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Set(currentUserIDKey, "spoofed")
	if _, ok := CurrentUserID(c); ok {
		t.Fatal("string accepted as UUID")
	}
}
