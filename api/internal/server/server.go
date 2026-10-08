package server

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/cog-forge/rolecue/api/internal/config"
	"github.com/cog-forge/rolecue/api/internal/pkg/logger"
)

type Server struct {
	httpServer *http.Server
	logger     *logger.Logger
}

func NewServer(cfg config.ServerConfig, engine *gin.Engine, log *logger.Logger) *Server {
	return &Server{
		httpServer: &http.Server{
			Addr:         fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
			Handler:      engine,
			ReadTimeout:  cfg.ReadTimeout,
			WriteTimeout: cfg.WriteTimeout,
			IdleTimeout:  time.Minute,
		},
		logger: log,
	}
}

func (s *Server) Start() error {
	s.logger.Info("Starting HTTP server", logger.String("addr", s.httpServer.Addr))
	return s.httpServer.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	s.logger.Info("Shutting down HTTP server...")
	return s.httpServer.Shutdown(ctx)
}

// Close terminates active connections if graceful shutdown cannot complete.
func (s *Server) Close() error {
	return s.httpServer.Close()
}
