package service

import (
	"context"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/cog-forge/rolecue/api/internal/features/admin/accounts/domain"
	"github.com/cog-forge/rolecue/api/internal/pkg/logger"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// logLockDecision records an Admin lock decision. The reason is typed by the Admin, so only its length
// is logged. The Admin's own ID is not passed to this service, so the audit line names the target only.
func logLockDecision(ctx context.Context, targetID uuid.UUID, locking bool, result domain.Transition, request domain.LockRequest) {
	status := "unlocked"
	if locking {
		status = "locked"
	}
	fields := []zap.Field{
		logger.Op("admin.set_account_lock"),
		logger.String("target_user_id", targetID.String()),
		logger.String("to_status", status),
		logger.String("notification_status", result.Notification.Status),
	}
	if !result.Changed {
		logger.FromContext(ctx).Info("account lock unchanged", append(fields, logger.String("reason", "already_"+status))...)
		return
	}
	if locking {
		fields = append(fields, logger.Int("reason_length", utf8.RuneCountInString(request.Reason)))
		if request.ExpiresIn != nil {
			fields = append(fields, zap.Int64("expires_in", *request.ExpiresIn))
		}
	}
	logger.FromContext(ctx).Info("account lock changed", fields...)
}

type Repository interface {
	List(context.Context, domain.ListFilter) (domain.List, error)
	Get(context.Context, uuid.UUID) (domain.Account, error)
	WithAccountLock(context.Context, uuid.UUID, func(domain.Account) error) error
}

type AuthAdmin interface {
	SetLock(context.Context, string, uuid.UUID, bool, bool, domain.LockRequest) (domain.Transition, error)
}

type Service struct {
	repository Repository
	auth       AuthAdmin
}

func New(repository Repository, auth AuthAdmin) *Service {
	return &Service{repository: repository, auth: auth}
}

func (s *Service) List(ctx context.Context, filter domain.ListFilter) (domain.List, error) {
	filter.Search = strings.TrimSpace(filter.Search)
	if utf8.RuneCountInString(filter.Search) > 500 || filter.Limit < 1 || filter.Limit > 100 || filter.Offset < 0 {
		return domain.List{}, apperror.New(apperror.CodeValidation, "invalid account search or pagination")
	}
	switch filter.Filter {
	case "all", "locked", "candidate", "recruiter", "admin":
	default:
		return domain.List{}, apperror.New(apperror.CodeValidation, "invalid account filter")
	}
	return s.repository.List(ctx, filter)
}

func (s *Service) Get(ctx context.Context, id uuid.UUID) (domain.Account, error) {
	return s.repository.Get(ctx, id)
}

func (s *Service) SetLock(ctx context.Context, cookie string, id uuid.UUID, locking bool, request domain.LockRequest) (domain.Transition, error) {
	if locking {
		request.Reason = strings.TrimSpace(request.Reason)
		if request.Reason == "" || utf8.RuneCountInString(request.Reason) > 500 {
			return domain.Transition{}, apperror.New(apperror.CodeValidation, "enter a user-visible reason between 1 and 500 characters")
		}
		// Expiry must fit the API's RFC3339 timestamp contract.
		if request.ExpiresIn != nil && (*request.ExpiresIn <= 0 || *request.ExpiresIn > time.Date(9999, 12, 31, 23, 59, 59, 0, time.UTC).Unix()-time.Now().Unix()) {
			return domain.Transition{}, apperror.New(apperror.CodeValidation, "choose a valid positive lock duration")
		}
	}
	var result domain.Transition
	err := s.repository.WithAccountLock(ctx, id, func(account domain.Account) error {
		if account.Role != "candidate" && account.Role != "recruiter" {
			return apperror.New(apperror.CodeForbidden, "only Candidate and Recruiter accounts can be locked or unlocked")
		}
		current := account.Locked(time.Now())
		result = domain.Transition{User: account, Notification: domain.Notification{Status: "not-needed"}}
		if current == locking && (locking || !account.IsLocked) {
			return nil
		}
		updated, err := s.auth.SetLock(ctx, cookie, id, locking, current != locking, request)
		if err != nil {
			return err
		}
		result = updated
		result.Changed = true
		return nil
	})
	if err != nil {
		return result, err
	}
	logLockDecision(ctx, id, locking, result, request)
	return result, nil
}
