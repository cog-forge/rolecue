package ledger

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/cog-forge/rolecue/api/internal/pkg/logger"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

const sentinelDescription = "SENTINEL-POSTING-TITLE-TEXT"

func observedContext() (context.Context, *observer.ObservedLogs) {
	core, logs := observer.New(zap.DebugLevel)
	return logger.ToContext(context.Background(), &logger.Logger{Logger: zap.New(core)}), logs
}

// renderLogs flattens every message and field into one string, so a test can look for a secret anywhere.
func renderLogs(logs *observer.ObservedLogs) string {
	var b strings.Builder
	for _, e := range logs.All() {
		fmt.Fprintf(&b, "%s %v\n", e.Message, e.ContextMap())
	}
	return b.String()
}

func onlyLog(t *testing.T, logs *observer.ObservedLogs, message string) map[string]any {
	t.Helper()
	found := logs.FilterMessage(message).All()
	if len(found) != 1 {
		t.Fatalf("%q logged %d times, want once; logs:\n%s", message, len(found), renderLogs(logs))
	}
	return found[0].ContextMap()
}

func TestDebitLogsTheMovementWithoutTheDescription(t *testing.T) {
	ctx, logs := observedContext()
	db := &fakeDB{wallet: walletBalance(100)}
	if err := Debit(ctx, db, userID, 5, sentinelDescription); err != nil {
		t.Fatal(err)
	}
	got := onlyLog(t, logs, "wallet debit applied in transaction")
	for key, want := range map[string]string{
		"op": "wallet.debit", "user_id": userID.String(), "amount": "5", "balance_after": "95", "currency": Currency,
	} {
		if fmt.Sprint(got[key]) != want {
			t.Errorf("field %s = %v, want %s", key, got[key], want)
		}
	}
	if strings.Contains(renderLogs(logs), sentinelDescription) {
		t.Fatalf("the ledger description reached the log:\n%s", renderLogs(logs))
	}
}

func TestRejectedDebitLogsNothingAsDebited(t *testing.T) {
	ctx, logs := observedContext()
	db := &fakeDB{wallet: walletBalance(1)}
	if err := Debit(ctx, db, userID, 5, "over balance"); err == nil {
		t.Fatal("Debit() error = nil, want insufficient balance")
	}
	if n := logs.FilterMessage("wallet debit applied in transaction").Len(); n != 0 {
		t.Fatalf("a refused debit logged as debited %d times", n)
	}
}

func TestCreditForOrderLogsTheBalanceAfterAndTheReason(t *testing.T) {
	ctx, logs := observedContext()
	db := &fakeDB{wallet: walletBalance(10)}
	if err := CreditForOrder(ctx, db, userID, 40); err != nil {
		t.Fatal(err)
	}
	got := onlyLog(t, logs, "wallet credit applied in transaction")
	for key, want := range map[string]string{
		"op": "wallet.credit_order", "reason": "paid_order", "amount": "40", "balance_after": "50",
	} {
		if fmt.Sprint(got[key]) != want {
			t.Errorf("field %s = %v, want %s", key, got[key], want)
		}
	}
}

func TestFailedCreditLogsNothing(t *testing.T) {
	ctx, logs := observedContext()
	db := &fakeDB{wallet: walletBalance(0), execErr: errors.New("connection reset")}
	if err := Credit(ctx, db, userID, 5, sentinelDescription); err == nil {
		t.Fatal("Credit() error = nil, want a persistence error")
	}
	if n := logs.Len(); n != 0 {
		t.Fatalf("a failed credit logged %d lines: %s", n, renderLogs(logs))
	}
}
