package service

import (
	"context"
	"time"

	"github.com/cog-forge/rolecue/api/internal/features/application/domain"
	inventory "github.com/cog-forge/rolecue/api/internal/features/inventory/domain"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

type InterviewerRepository interface {
	CompanyInterviewer(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (domain.InterviewerRecord, error)
}

// CompanyInterviewer grants GET access to the locked presentation only. It never reads inventory
// metadata or creates a session; the repository checks ownership and eligibility on every renewal.
func (s *Service) CompanyInterviewer(ctx context.Context, candidateID uuid.UUID, role string, applicationID, sessionID uuid.UUID) (domain.CompanyInterviewer, error) {
	if role != candidateRole {
		return domain.CompanyInterviewer{}, domain.ErrApplicationNotFound
	}
	repo, ok := s.repository.(InterviewerRepository)
	if !ok {
		return domain.CompanyInterviewer{}, apperror.New(apperror.CodeInternal, "company interviewer is not configured")
	}
	record, err := repo.CompanyInterviewer(ctx, candidateID, applicationID, sessionID)
	if err != nil {
		return domain.CompanyInterviewer{}, err
	}
	ttl := min(5*time.Minute, time.Until(record.Deadline)).Truncate(time.Second)
	if ttl <= 0 {
		return domain.CompanyInterviewer{}, domain.ErrApplicationNotFound
	}
	grant, err := s.cvLinker.PresignGet(ctx, record.ModelPath, ttl)
	if err != nil {
		return domain.CompanyInterviewer{}, apperror.Wrap(apperror.CodeAvatarAssetUnavailable, "company interviewer is temporarily unavailable", err)
	}
	if grant.ExpiresAt.After(record.Deadline) {
		grant.ExpiresAt = record.Deadline
	}
	if grant.Headers == nil {
		grant.Headers = map[string]string{}
	}
	return domain.CompanyInterviewer{Name: record.Name, VoiceName: record.VoiceName, ModelAccess: inventory.ModelAccess{URL: grant.URL, Method: grant.Method, Headers: grant.Headers, ExpiresAt: grant.ExpiresAt}}, nil
}
