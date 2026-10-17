package service

import (
	"context"
	"testing"
	"time"

	"github.com/cog-forge/rolecue/api/internal/features/admin/accounts/domain"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

type accountRepository struct{ account domain.Account }

func (r *accountRepository) List(context.Context, domain.ListFilter) (domain.List, error) {
	return domain.List{}, nil
}
func (r *accountRepository) Get(context.Context, uuid.UUID) (domain.Account, error) {
	return r.account, nil
}
func (r *accountRepository) WithAccountLock(_ context.Context, _ uuid.UUID, fn func(domain.Account) error) error {
	return fn(r.account)
}

type authAdmin struct {
	repo  *accountRepository
	calls int
	input domain.LockRequest
	mail  *notifications
}

func (a *authAdmin) SetLock(_ context.Context, _ string, _ uuid.UUID, locking, notify bool, input domain.LockRequest) (domain.Transition, error) {
	a.calls++
	a.input = input
	a.repo.account.IsLocked = locking
	if locking {
		a.repo.account.LockReason = &input.Reason
		a.repo.account.LockExpiresAt = nil
		if input.ExpiresIn != nil {
			expiry := time.Now().Add(time.Duration(*input.ExpiresIn) * time.Second)
			a.repo.account.LockExpiresAt = &expiry
		}
	} else {
		a.repo.account.LockExpiresAt = nil
		a.repo.account.LockReason = nil
	}
	status := "not-needed"
	if notify {
		a.mail.calls++
		a.mail.reason = input.Reason
		status = "accepted"
		if a.mail.fail {
			status = "failed"
		}
	}
	return domain.Transition{User: a.repo.account, Notification: domain.Notification{Status: status}}, nil
}

type notifications struct {
	calls  int
	reason string
	fail   bool
}

func fixture() (*Service, *accountRepository, *authAdmin, *notifications) {
	repo := &accountRepository{account: domain.Account{ID: uuid.New(), Role: "candidate", Email: "user@example.com", Name: "Alex"}}
	mail := &notifications{}
	auth := &authAdmin{repo: repo, mail: mail}
	return New(repo, auth), repo, auth, mail
}

func TestTransitionAndNoOpNotifications(t *testing.T) {
	svc, repo, auth, mail := fixture()
	seconds := int64(3600)
	request := domain.LockRequest{Reason: "  Repeated spam  ", ExpiresIn: &seconds}
	result, err := svc.SetLock(context.Background(), "admin-cookie", repo.account.ID, true, request)
	if err != nil || !result.Changed || result.Notification.Status != "accepted" || auth.input.Reason != "Repeated spam" || mail.reason != "Repeated spam" || repo.account.LockExpiresAt == nil {
		t.Fatalf("lock: result=%+v err=%v", result, err)
	}
	result, err = svc.SetLock(context.Background(), "admin-cookie", repo.account.ID, true, request)
	if err != nil || result.Changed || auth.calls != 1 || mail.calls != 1 {
		t.Fatalf("repeated lock should be a no-op: %+v %v", result, err)
	}
	result, err = svc.SetLock(context.Background(), "admin-cookie", repo.account.ID, false, domain.LockRequest{})
	if err != nil || !result.Changed || result.User.IsLocked || auth.calls != 2 || mail.calls != 2 {
		t.Fatalf("unlock: %+v %v", result, err)
	}
	_, err = svc.SetLock(context.Background(), "admin-cookie", repo.account.ID, false, domain.LockRequest{})
	if err != nil || auth.calls != 2 || mail.calls != 2 {
		t.Fatalf("repeated unlock must not send mail")
	}
}

func TestExpiredLockCanBeRelocked(t *testing.T) {
	svc, repo, auth, mail := fixture()
	expired := time.Now().Add(-time.Hour)
	repo.account.IsLocked, repo.account.LockExpiresAt = true, &expired
	result, err := svc.SetLock(context.Background(), "cookie", repo.account.ID, true, domain.LockRequest{Reason: "New public reason"})
	if err != nil || !result.Changed || result.User.LockExpiresAt != nil || auth.calls != 1 || mail.calls != 1 {
		t.Fatalf("expired relock: %+v %v", result, err)
	}
	// Clearing an already expired raw flag does not notify an effective access transition.
	repo.account.LockExpiresAt = &expired
	result, err = svc.SetLock(context.Background(), "cookie", repo.account.ID, false, domain.LockRequest{})
	if err != nil || !result.Changed || result.Notification.Status != "not-needed" || mail.calls != 1 {
		t.Fatalf("expired cleanup: %+v %v", result, err)
	}
}

func TestPolicyAndMailFailure(t *testing.T) {
	svc, repo, auth, mail := fixture()
	repo.account.Role = "admin"
	_, err := svc.SetLock(context.Background(), "cookie", repo.account.ID, true, domain.LockRequest{Reason: "Reason"})
	if !apperror.IsCode(err, apperror.CodeForbidden) || auth.calls != 0 {
		t.Fatalf("admin target must be protected: %v", err)
	}
	repo.account.Role = "candidate"
	for _, request := range []domain.LockRequest{{Reason: " "}, {Reason: "Reason", ExpiresIn: new(int64)}} {
		_, err := svc.SetLock(context.Background(), "cookie", repo.account.ID, true, request)
		if !apperror.IsCode(err, apperror.CodeValidation) || auth.calls != 0 {
			t.Fatalf("invalid request: %v", err)
		}
	}
	mail.fail = true
	result, err := svc.SetLock(context.Background(), "cookie", repo.account.ID, true, domain.LockRequest{Reason: "Public reason"})
	if err != nil || !result.User.IsLocked || !result.Changed || result.Notification.Status != "failed" {
		t.Fatalf("mail failure must preserve lock: %+v %v", result, err)
	}
}
