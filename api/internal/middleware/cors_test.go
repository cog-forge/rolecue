package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/cog-forge/rolecue/api/internal/config"
)

func TestCORS(t *testing.T) {
	for _, tc := range []struct {
		name               string
		origins            []string
		credentials        bool
		origin, wantOrigin string
		status             int
	}{
		{"explicit", []string{"http://localhost:3000"}, true, "http://localhost:3000", "http://localhost:3000", 204},
		{"unknown", []string{"http://localhost:3000"}, true, "https://unknown.example", "", 403},
		{"wildcard", []string{"*"}, false, "https://unknown.example", "*", 204},
		{"unsafe wildcard", []string{"*"}, true, "https://unknown.example", "", 403},
		{"unsafe mixed wildcard", []string{"http://localhost:3000", "*"}, true, "http://localhost:3000", "", 403},
		{"no origin", []string{"http://localhost:3000"}, true, "", "", 204},
	} {
		for _, method := range []string{http.MethodGet, http.MethodOptions} {
			t.Run(tc.name+method, func(t *testing.T) {
				r := gin.New()
				r.Use(CORSMiddleware(config.CORSConfig{AllowOrigins: tc.origins, AllowCredentials: tc.credentials, AllowMethods: []string{"GET", "POST"}, AllowHeaders: []string{"Authorization"}, MaxAge: 60}))
				called := false
				r.Any("/test", func(c *gin.Context) { called = true; c.Status(204) })
				req := httptest.NewRequest(method, "/test", nil)
				req.Header.Set("Origin", tc.origin)
				req.Header.Set("Access-Control-Request-Method", "POST")
				rec := httptest.NewRecorder()
				r.ServeHTTP(rec, req)
				if rec.Code != tc.status || rec.Header().Get("Access-Control-Allow-Origin") != tc.wantOrigin {
					t.Fatalf("status=%d headers=%v", rec.Code, rec.Header())
				}
				wantCredentials := ""
				if tc.wantOrigin != "" && tc.credentials {
					wantCredentials = "true"
				}
				if rec.Header().Get("Access-Control-Allow-Credentials") != wantCredentials {
					t.Fatal("unsafe credential header")
				}
				if tc.status == 403 && called {
					t.Fatal("rejected request reached handler")
				}
				if method == http.MethodOptions && tc.wantOrigin != "" {
					if called || rec.Header().Get("Access-Control-Allow-Methods") == "" || rec.Header().Get("Access-Control-Allow-Headers") == "" {
						t.Fatal("preflight broken")
					}
				}
			})
		}
	}
}
