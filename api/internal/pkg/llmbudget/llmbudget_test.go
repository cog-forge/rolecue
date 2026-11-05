package llmbudget

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

// memStore mirrors the SQL upsert: count per user and day, refuse at the limit, 0 = unlimited.
type memStore struct {
	mu   sync.Mutex
	rows map[string]int
	err  error
}

func (m *memStore) Consume(_ context.Context, userID uuid.UUID, day time.Time, limit int) (int, bool, error) {
	if m.err != nil {
		return 0, false, m.err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.rows == nil {
		m.rows = map[string]int{}
	}
	key := userID.String() + day.Format("2006-01-02")
	if limit > 0 && m.rows[key] >= limit {
		return limit, false, nil
	}
	m.rows[key]++
	return m.rows[key], true, nil
}

func budgetAt(store Store, limit int, clock *time.Time) *Budget {
	b := New(store, limit)
	b.now = func() time.Time { return *clock }
	return b
}

func TestBudgetRefusesAfterTheDailyLimit(t *testing.T) {
	now := time.Date(2026, 11, 5, 10, 0, 0, 0, time.UTC)
	b := budgetAt(&memStore{}, 3, &now)
	user := uuid.New()
	for i := 0; i < 3; i++ {
		if err := b.Consume(context.Background(), user); err != nil {
			t.Fatalf("call %d refused: %v", i+1, err)
		}
	}
	err := b.Consume(context.Background(), user)
	if !IsExceeded(err) {
		t.Fatalf("4th call = %v, want LLM_BUDGET_EXCEEDED", err)
	}
	// Retrying a refused call does not count and does not get through.
	for i := 0; i < 5; i++ {
		if err := b.Consume(context.Background(), user); !IsExceeded(err) {
			t.Fatalf("retry %d = %v, want LLM_BUDGET_EXCEEDED", i+1, err)
		}
	}
	// Another user has their own budget.
	if err := b.Consume(context.Background(), uuid.New()); err != nil {
		t.Fatalf("other user refused: %v", err)
	}
}

func TestBudgetResetsOnTheNextUTCDay(t *testing.T) {
	now := time.Date(2026, 11, 5, 23, 59, 0, 0, time.UTC)
	b := budgetAt(&memStore{}, 1, &now)
	user := uuid.New()
	if err := b.Consume(context.Background(), user); err != nil {
		t.Fatal(err)
	}
	if !IsExceeded(b.Consume(context.Background(), user)) {
		t.Fatal("second call on the same day was allowed")
	}
	now = now.Add(2 * time.Minute) // 00:01 UTC the next day
	if err := b.Consume(context.Background(), user); err != nil {
		t.Fatalf("call on the next day refused: %v", err)
	}
}

func TestBudgetDayIsUTCNotLocal(t *testing.T) {
	zone := time.FixedZone("UTC+7", 7*3600)
	store := &memStore{}
	// 06:30 local on Nov 6 is 23:30 UTC on Nov 5.
	now := time.Date(2026, 11, 6, 6, 30, 0, 0, zone)
	b := budgetAt(store, 1, &now)
	user := uuid.New()
	if err := b.Consume(context.Background(), user); err != nil {
		t.Fatal(err)
	}
	if store.rows[user.String()+"2026-11-05"] != 1 {
		t.Fatalf("rows = %v, want the call counted on 2026-11-05", store.rows)
	}
}

func TestBudgetZeroIsUnlimited(t *testing.T) {
	now := time.Date(2026, 11, 5, 10, 0, 0, 0, time.UTC)
	b := budgetAt(&memStore{}, 0, &now)
	user := uuid.New()
	for i := 0; i < 1000; i++ {
		if err := b.Consume(context.Background(), user); err != nil {
			t.Fatalf("call %d refused with an unlimited budget: %v", i+1, err)
		}
	}
}

func TestBudgetHoldsUnderConcurrency(t *testing.T) {
	now := time.Date(2026, 11, 5, 10, 0, 0, 0, time.UTC)
	b := budgetAt(&memStore{}, 10, &now)
	user := uuid.New()
	var wg sync.WaitGroup
	var mu sync.Mutex
	allowed := 0
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if b.Consume(context.Background(), user) == nil {
				mu.Lock()
				allowed++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if allowed != 10 {
		t.Fatalf("allowed = %d, want exactly 10", allowed)
	}
}

func TestBudgetStoreFailureIsAnInternalErrorNotABudgetError(t *testing.T) {
	now := time.Date(2026, 11, 5, 10, 0, 0, 0, time.UTC)
	b := budgetAt(&memStore{err: errors.New("db down")}, 5, &now)
	err := b.Consume(context.Background(), uuid.New())
	var appErr *apperror.AppError
	if !errors.As(err, &appErr) || appErr.Code != apperror.CodeInternal {
		t.Fatalf("err = %v, want INTERNAL", err)
	}
	if IsExceeded(err) {
		t.Fatal("a store failure was reported as a spent budget")
	}
}
