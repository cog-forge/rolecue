package service

import (
	"context"
	"errors"
	"testing"

	"github.com/cog-forge/rolecue/api/internal/features/auth/domain"
	"github.com/cog-forge/rolecue/api/internal/pkg/logger"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

type roleSelectorFake struct{ err error }

func (f roleSelectorFake) SelectRole(context.Context, uuid.UUID, string) (domain.User, error) {
	return domain.User{}, f.err
}

func TestSelectRoleLogsTransitionOnlyOnSuccess(t *testing.T) {
	core, logs := observer.New(zap.DebugLevel)
	ctx := logger.ToContext(context.Background(), &logger.Logger{Logger: zap.New(core)})
	userID := uuid.New()

	if _, err := NewService(roleSelectorFake{}).SelectRole(ctx, userID, "recruiter"); err != nil {
		t.Fatalf("SelectRole: %v", err)
	}
	entries := logs.FilterMessage("onboarding role selected").All()
	if len(entries) != 1 || entries[0].ContextMap()["op"] != "onboarding.select_role" ||
		entries[0].ContextMap()["user_id"] != userID.String() || entries[0].ContextMap()["role"] != "recruiter" {
		t.Fatalf("role selection log = %v", entries)
	}

	if _, err := NewService(roleSelectorFake{err: errors.New("conflict")}).SelectRole(ctx, userID, "candidate"); err == nil {
		t.Fatal("SelectRole must return the auth error")
	}
	if n := logs.FilterMessage("onboarding role selected").Len(); n != 1 {
		t.Fatalf("a failed selection was logged; total entries = %d", n)
	}
}
