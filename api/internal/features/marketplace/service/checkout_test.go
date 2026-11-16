package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/cog-forge/rolecue/api/internal/features/marketplace/domain"
	"github.com/cog-forge/rolecue/api/internal/features/marketplace/service"
	"github.com/cog-forge/rolecue/api/internal/pkg/ledger"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
)

// checkoutRepo embeds the interface so only Checkout needs a fake; any other call panics on nil.
type checkoutRepo struct {
	service.Repository
	calls  int
	errors []error
	keys   []string
}

func (r *checkoutRepo) Checkout(_ context.Context, req domain.PurchaseRequest) (domain.PurchaseResult, error) {
	r.calls++
	r.keys = append(r.keys, req.KeyText)
	if r.calls <= len(r.errors) && r.errors[r.calls-1] != nil {
		return domain.PurchaseResult{}, r.errors[r.calls-1]
	}
	return domain.PurchaseResult{Balance: 30}, nil
}

func pgFailure(code string) error {
	return apperror.Wrap(apperror.CodeInternal, "unable to save", &pgconn.PgError{Code: code})
}

func purchase(t *testing.T, repo *checkoutRepo) (domain.PurchaseResult, error) {
	t.Helper()
	listing := uuid.New()
	key, err := ledger.NewMarketplaceKey(listing, 20, "q7Zt3KfA9xLmP2Vd")
	if err != nil {
		t.Fatal(err)
	}
	// Purchase never signs a URL, so the preview signer is not needed here.
	return service.New(repo, nil).Purchase(context.Background(), uuid.New(), listing, 20, key)
}

func TestPurchaseRetriesWholeTransactionOnDeadlock(t *testing.T) {
	for _, code := range []string{"40P01", "40001"} {
		repo := &checkoutRepo{errors: []error{pgFailure(code), pgFailure(code)}}
		res, err := purchase(t, repo)
		if err != nil || res.Balance != 30 || repo.calls != 3 {
			t.Fatalf("%s: err=%v calls=%d", code, err, repo.calls)
		}
		if repo.keys[0] != repo.keys[1] || repo.keys[1] != repo.keys[2] {
			t.Errorf("every attempt must use the same key: %v", repo.keys)
		}
	}
}

func TestPurchaseGivesUpAfterThreeAttempts(t *testing.T) {
	repo := &checkoutRepo{errors: []error{pgFailure("40P01"), pgFailure("40P01"), pgFailure("40P01"), pgFailure("40P01")}}
	_, err := purchase(t, repo)
	if !ledger.IsRetryable(err) || repo.calls != domain.MaxCheckoutAttempts {
		t.Fatalf("err=%v calls=%d", err, repo.calls)
	}
}

func TestPurchaseDoesNotRetryOtherErrors(t *testing.T) {
	refusal := apperror.New(apperror.CodeInsufficientBalance, "no coins")
	repo := &checkoutRepo{errors: []error{refusal}}
	if _, err := purchase(t, repo); !errors.Is(err, refusal) || repo.calls != 1 {
		t.Fatalf("err=%v calls=%d", err, repo.calls)
	}
}

func TestPurchaseRejectsBadRequestBeforeStorage(t *testing.T) {
	repo := &checkoutRepo{}
	svc := service.New(repo, nil)
	listing := uuid.New()
	for name, key := range map[string]string{"empty": "", "slot key": "slot-purchase-0001"} {
		if _, err := svc.Purchase(context.Background(), uuid.New(), listing, 20, key); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
	if repo.calls != 0 {
		t.Errorf("storage reached %d times", repo.calls)
	}
}
