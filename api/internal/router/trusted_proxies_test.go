package router

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/cog-forge/rolecue/api/internal/config"
	"github.com/cog-forge/rolecue/api/internal/pkg/logger"
)

func TestTrustedProxiesGovernClientIP(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		name    string
		proxies []string
		peer    string
		want    string
	}{
		{"untrusted peer cannot spoof", []string{"127.0.0.1"}, "203.0.113.9:4000", "203.0.113.9"},
		{"trusted peer is believed", []string{"127.0.0.1"}, "127.0.0.1:4000", "198.51.100.7"},
		{"trusted CIDR is believed", []string{"10.0.0.0/8"}, "10.1.2.3:4000", "198.51.100.7"},
		{"no proxy is trusted by default-empty list", nil, "127.0.0.1:4000", "127.0.0.1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &config.Config{Server: config.ServerConfig{TrustedProxies: tc.proxies}}
			r := &Router{cfg: cfg, logger: &logger.Logger{Logger: zap.NewNop()}}
			engine := gin.New()
			r.applyTrustedProxies(engine)
			engine.GET("/ip", func(c *gin.Context) { c.String(http.StatusOK, c.ClientIP()) })

			req := httptest.NewRequest(http.MethodGet, "/ip", nil)
			req.RemoteAddr = tc.peer
			req.Header.Set("X-Forwarded-For", "198.51.100.7")
			w := httptest.NewRecorder()
			engine.ServeHTTP(w, req)
			if got := w.Body.String(); got != tc.want {
				t.Fatalf("ClientIP = %q, want %q", got, tc.want)
			}
		})
	}
}
