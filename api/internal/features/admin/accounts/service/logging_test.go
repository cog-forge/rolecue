package service

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/cog-forge/rolecue/api/internal/features/admin/accounts/domain"
	"github.com/cog-forge/rolecue/api/internal/pkg/logger"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func TestLockDecisionsAreLoggedWithoutReasonText(t *testing.T) {
	svc, repo, _, _ := fixture()
	core, logs := observer.New(zap.DebugLevel)
	ctx := logger.ToContext(context.Background(), &logger.Logger{Logger: zap.New(core)})
	seconds := int64(3600)
	reason := "SENTINEL-LOCK-REASON-7f3a repeated spam"
	request := domain.LockRequest{Reason: reason, ExpiresIn: &seconds}

	if _, err := svc.SetLock(ctx, "admin-cookie", repo.account.ID, true, request); err != nil {
		t.Fatalf("lock: %v", err)
	}
	changed := logs.FilterMessage("account lock changed").All()
	if len(changed) != 1 {
		t.Fatalf("lock logged %d changes, want 1", len(changed))
	}
	fields := changed[0].ContextMap()
	if fields["op"] != "admin.set_account_lock" || fields["to_status"] != "locked" ||
		fields["target_user_id"] != repo.account.ID.String() || fields["reason_length"] != int64(len(reason)) ||
		fields["expires_in"] != int64(3600) {
		t.Fatalf("lock fields = %v", fields)
	}

	if _, err := svc.SetLock(ctx, "admin-cookie", repo.account.ID, true, request); err != nil {
		t.Fatalf("repeated lock: %v", err)
	}
	unchanged := logs.FilterMessage("account lock unchanged").All()
	if len(unchanged) != 1 || unchanged[0].ContextMap()["reason"] != "already_locked" {
		t.Fatalf("repeated lock log = %v", unchanged)
	}

	for _, entry := range logs.All() {
		if text := fmt.Sprint(entry.Message, entry.ContextMap()); strings.Contains(text, "SENTINEL") {
			t.Fatalf("lock reason reached the log: %s", text)
		}
	}
}
