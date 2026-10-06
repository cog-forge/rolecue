package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/swp391-group3/ai-interview-practice/api/internal/pkg/logger"
)

// @title AI Interview Practice API
// @version 1.0.0
// @description Application API endpoints protected by Better Auth session cookies. Health is liveness only.
// @BasePath /
// @securityDefinitions.apikey SessionCookie
// @in header
// @name Cookie
// @description Better Auth HttpOnly session cookie. Production uses __Secure-rolecue-authentication.session_token; browser sends cookies automatically.
func main() {
	var configPath string
	flag.StringVar(&configPath, "config", "", "path to config file")
	flag.Parse()
	if err := run(configPath); err != nil {
		fmt.Fprintf(os.Stderr, "API stopped: %v\n", err)
		os.Exit(1)
	}
}

func run(configPath string) error {
	app, cleanup, err := InitializeApplication(configPath)
	if err != nil {
		return fmt.Errorf("initialize application: %w", err)
	}
	defer cleanup()

	app.Logger.Info("Starting AI Interview Practice API",
		logger.String("environment", app.Config.App.Environment),
		logger.String("version", app.Config.App.Version),
	)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	return serve(ctx, app.Server, app.Config.Server.ShutdownTimeout)
}

type httpServer interface {
	Start() error
	Shutdown(context.Context) error
	Close() error
}

func serve(ctx context.Context, server httpServer, timeout time.Duration) error {
	serverErrors := make(chan error, 1)

	go func() {
		serverErrors <- server.Start()
	}()

	select {
	case <-ctx.Done():
		// Normal shutdown path.
	case startErr := <-serverErrors:
		if errors.Is(startErr, http.ErrServerClosed) {
			return nil
		}
		return startErr
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	shutdownErr := server.Shutdown(shutdownCtx)
	if shutdownErr != nil {
		shutdownErr = errors.Join(shutdownErr, server.Close())
	}

	startErr := <-serverErrors
	if errors.Is(startErr, http.ErrServerClosed) {
		startErr = nil
	}

	return errors.Join(startErr, shutdownErr)
}
