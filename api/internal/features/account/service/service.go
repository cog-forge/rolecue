// Package service implements account erasure (KAN-280) and the personal data export (KAN-282).
package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/cog-forge/rolecue/api/internal/features/account/domain"
	"github.com/cog-forge/rolecue/api/internal/features/account/repository"
	authdomain "github.com/cog-forge/rolecue/api/internal/features/auth/domain"
	"github.com/cog-forge/rolecue/api/internal/pkg/logger"
	"github.com/cog-forge/rolecue/api/internal/pkg/storage"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

// Repository is the data access the service needs.
type Repository interface {
	Account(context.Context, uuid.UUID) (domain.Identity, error)
	ActiveWork(context.Context, uuid.UUID) (domain.ErasureBlockers, error)
	StorageKeys(context.Context, uuid.UUID) ([]string, error)
	Erase(context.Context, uuid.UUID) ([]string, error)
	ModelKeyInUse(context.Context, string) (bool, error)
	ExportProfile(context.Context, uuid.UUID) (domain.ExportProfile, error)
	ExportRoleProfiles(context.Context, uuid.UUID) ([]domain.ExportRoleProfile, error)
	ExportApplications(context.Context, uuid.UUID) ([]domain.ExportApplication, error)
	ExportInterviewSetups(context.Context, uuid.UUID) ([]domain.ExportInterviewSetup, error)
	ExportInterviews(context.Context, uuid.UUID) ([]domain.ExportInterview, error)
	ExportPostings(context.Context, uuid.UUID) ([]domain.ExportPosting, error)
}

// Store is the part of the object store the service uses.
type Store interface {
	Delete(ctx context.Context, key string) error
	PresignGetWithOverrides(ctx context.Context, key string, ttl time.Duration, overrides storage.GetOverrides) (storage.SignedRequest, error)
}

type Service struct {
	repository Repository
	store      Store
	now        func() time.Time
}

func NewService(repo *repository.Repository, store Store) *Service {
	return &Service{repository: repo, store: store, now: time.Now}
}

// Erase deletes the signed-in account. Order matters for retries: the checks run first, then the stored objects
// that only this account can reference (CVs, recordings, JD files, upload grants) are deleted (a missing object
// counts as deleted), and the database rows go in one transaction. A storage failure keeps the account and the
// call can be repeated. If the final in-transaction check finds new work, files removed by then stay removed.
//
// The current profile photo is selected under the account lock and deleted after the commit.
// Avatar models can be shared with other accounts (a copy bought from a listing),
// so the transaction decides which assets are orphans under the asset locks, and the objects of exactly those
// deleted rows are removed after the commit, one by one, and only when no asset stores the key any more. A failure
// there is logged and leaves an unreferenced object behind: the account is already gone and an orphan object
// is harmless, while a deleted object under a live avatar is not.
func (s *Service) Erase(ctx context.Context, user authdomain.User, confirm string) error {
	if user.ID == uuid.Nil {
		return apperror.New(apperror.CodeInvalidToken, "authentication required")
	}
	if user.Role == "admin" {
		return apperror.New(apperror.CodeAdminCannotSelfDelete, "administrator accounts cannot be deleted here")
	}
	confirm = strings.TrimSpace(confirm)
	if confirm != domain.ErasureConfirmation && (user.Email == "" || !strings.EqualFold(confirm, user.Email)) {
		return apperror.New(apperror.CodeValidation, `confirm must be "DELETE" or your email address`)
	}
	work, err := s.repository.ActiveWork(ctx, user.ID)
	if err != nil {
		return err
	}
	if !work.None() {
		return activeWorkError(work)
	}
	keys, err := s.repository.StorageKeys(ctx, user.ID)
	if err != nil {
		return err
	}
	for _, key := range keys {
		if isWebURL(key) {
			continue // a JD that was linked, not uploaded, has nothing in the store
		}
		if err := s.store.Delete(ctx, key); err != nil && !errors.Is(err, storage.ErrNotFound) {
			logger.FromContext(ctx).Error("account erasure: storage delete failed",
				logger.Op("account.erase"), logger.String("user_id", user.ID.String()), logger.Error(err))
			return apperror.Wrap(apperror.CodeInternal, "some of your files could not be removed and your account was kept; please try again", err)
		}
	}
	objects, err := s.repository.Erase(ctx, user.ID)
	if err != nil {
		if errors.Is(err, repository.ErrActiveWork) {
			w, werr := s.repository.ActiveWork(ctx, user.ID)
			if werr != nil {
				return werr
			}
			return activeWorkError(w)
		}
		return err
	}
	removed := s.removeUnreferencedObjects(ctx, user.ID, objects)
	logger.FromContext(ctx).Info("account erased", logger.Op("account.erase"),
		logger.String("user_id", user.ID.String()), logger.Int("object_count", len(keys)+removed))
	return nil
}

// removeUnreferencedObjects deletes the former profile photo and orphaned models after the commit.
// A model may still be stored by another asset, so check for references before deleting each object.
func (s *Service) removeUnreferencedObjects(ctx context.Context, userID uuid.UUID, objects []string) int {
	removed := 0
	seen := map[string]bool{}
	for _, key := range objects {
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		used, err := s.repository.ModelKeyInUse(ctx, key)
		if err == nil && used {
			continue
		}
		if err == nil {
			err = s.store.Delete(ctx, key)
			if errors.Is(err, storage.ErrNotFound) {
				err = nil
			}
		}
		if err != nil {
			// The key is an opaque object id (no personal data) and the only way to find the leftover object.
			logger.FromContext(ctx).Error("account erasure: storage object left behind",
				logger.Op("account.erase"), logger.String("user_id", userID.String()), logger.String("key", key), logger.Error(err))
			continue
		}
		removed++
	}
	return removed
}

func activeWorkError(w domain.ErasureBlockers) error {
	return apperror.New(apperror.CodeAccountHasActiveWork, fmt.Sprintf(
		"close your work before deleting the account: %d open job postings, %d applications on your postings",
		w.OpenPostings, w.ApplicationsInProgress))
}

func isWebURL(key string) bool {
	lower := strings.ToLower(key)
	return strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://")
}

// Export builds the signed-in account's data package. A Candidate gets candidate data; a Recruiter
// gets the profile and their own postings. Other roles are refused. One audit line is written per
// export with the user id and the time only, never any content or link.
func (s *Service) Export(ctx context.Context, user authdomain.User) (domain.DataExport, error) {
	if user.ID == uuid.Nil {
		return domain.DataExport{}, apperror.New(apperror.CodeInvalidToken, "authentication required")
	}
	if user.Role != "candidate" && user.Role != "recruiter" {
		return domain.DataExport{}, apperror.New(apperror.CodeForbidden, "this account has no exportable data")
	}
	now := s.now().UTC()
	out := domain.DataExport{GeneratedAt: now, LinkTTLSeconds: int(domain.ExportLinkTTL / time.Second)}
	var err error
	if out.Profile, err = s.repository.ExportProfile(ctx, user.ID); err != nil {
		return domain.DataExport{}, err
	}
	if user.Role == "recruiter" {
		if out.Postings, err = s.repository.ExportPostings(ctx, user.ID); err != nil {
			return domain.DataExport{}, err
		}
	} else if err = s.candidateData(ctx, user.ID, &out); err != nil {
		return domain.DataExport{}, err
	}
	logger.FromContext(ctx).Info("account data exported", logger.Op("account.export"),
		logger.String("user_id", user.ID.String()), logger.String("at", now.Format(time.RFC3339)))
	return out, nil
}

func (s *Service) candidateData(ctx context.Context, id uuid.UUID, out *domain.DataExport) (err error) {
	if out.RoleProfiles, err = s.repository.ExportRoleProfiles(ctx, id); err != nil {
		return err
	}
	if out.Applications, err = s.repository.ExportApplications(ctx, id); err != nil {
		return err
	}
	if out.InterviewSetups, err = s.repository.ExportInterviewSetups(ctx, id); err != nil {
		return err
	}
	if out.Interviews, err = s.repository.ExportInterviews(ctx, id); err != nil {
		return err
	}
	for n := range out.Applications {
		a := &out.Applications[n]
		if a.CVKey == nil || *a.CVKey == "" || isWebURL(*a.CVKey) {
			continue
		}
		if a.CV, err = s.link(ctx, *a.CVKey, storage.GetOverrides{ContentDisposition: "attachment"}); err != nil {
			return err
		}
	}
	for n := range out.Interviews {
		i := &out.Interviews[n]
		if i.RecordKey == nil || *i.RecordKey == "" {
			continue
		}
		if i.Recording, err = s.link(ctx, *i.RecordKey, storage.GetOverrides{ContentDisposition: "attachment"}); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) link(ctx context.Context, key string, overrides storage.GetOverrides) (*domain.DownloadLink, error) {
	req, err := s.store.PresignGetWithOverrides(ctx, key, domain.ExportLinkTTL, overrides)
	if err != nil {
		return nil, apperror.Wrap(apperror.CodeInternal, "download links are unavailable", err)
	}
	return &domain.DownloadLink{URL: req.URL, ExpiresAt: req.ExpiresAt}, nil
}
