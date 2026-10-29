package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"time"

	"github.com/cog-forge/rolecue/api/internal/features/admin/moderation/domain"
	evaluationdomain "github.com/cog-forge/rolecue/api/internal/features/evaluation/domain"
	"github.com/cog-forge/rolecue/api/internal/pkg/logger"
	"github.com/cog-forge/rolecue/api/internal/pkg/storage"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

const adminRole = "admin"

// Repository is the persistence contract consumed by Job Posting moderation.
type Repository interface {
	List(ctx context.Context, filter domain.ListFilter) ([]domain.PostingSummary, int64, error)
	Get(ctx context.Context, id uuid.UUID) (domain.ReviewPosting, error)
	Decide(ctx context.Context, id uuid.UUID, status domain.PostingStatus, rejectReason *string) error
}

// TextStore holds the JD that Job Posting creation uploaded; content_path holds its key. It is a
// PDF, or for postings created before PDF upload, a plain-text object.
type TextStore interface {
	Download(ctx context.Context, key string) (storage.Object, error)
	PresignGet(ctx context.Context, key string, ttl time.Duration) (storage.SignedRequest, error)
}

// documentLinkTTL keeps the PDF link valid long enough to read the posting and decide.
const documentLinkTTL = 15 * time.Minute

// maxJDTextBytes matches the Job Posting feature's stored-text cap (4 bytes per rune, 20,000 runes).
const maxJDTextBytes = 4 * 20_000

type EvaluationReader interface {
	ModerationWeights(context.Context, string, uuid.UUID) (evaluationdomain.PostingWeights, error)
}

type Service struct {
	evaluation EvaluationReader
	repository Repository
	texts      TextStore
	log        *logger.Logger
}

// New builds the service. Decisions are logged because the brain asks for an admin audit trail
// while the frozen schema has no audit table.
func New(repository Repository, texts TextStore, log *logger.Logger) (*Service, error) {
	if repository == nil || texts == nil {
		return nil, fmt.Errorf("moderation repository and text store are required")
	}
	if log == nil {
		log = &logger.Logger{Logger: zap.NewNop()}
	}
	return &Service{repository: repository, texts: texts, log: log}, nil
}

// SetEvaluationReader supplies the shared evaluation service without duplicating its defaults.
func (s *Service) SetEvaluationReader(reader EvaluationReader) { s.evaluation = reader }

func requireAdmin(role string) error {
	if role != adminRole {
		return apperror.New(apperror.CodeForbidden, "only administrators can moderate job postings")
	}
	return nil
}

func (s *Service) List(ctx context.Context, role string, filter domain.ListFilter) ([]domain.PostingSummary, int64, error) {
	if err := requireAdmin(role); err != nil {
		return nil, 0, err
	}
	if err := validateFilter(filter); err != nil {
		return nil, 0, err
	}
	return s.repository.List(ctx, filter)
}

func (s *Service) Get(ctx context.Context, role string, id uuid.UUID) (domain.ReviewPosting, error) {
	if err := requireAdmin(role); err != nil {
		return domain.ReviewPosting{}, err
	}
	return s.review(ctx, id)
}

func (s *Service) Approve(ctx context.Context, adminID uuid.UUID, role string, id uuid.UUID) (domain.ReviewPosting, error) {
	if err := requireAdmin(role); err != nil {
		return domain.ReviewPosting{}, err
	}
	return s.decide(ctx, adminID, id, domain.PostingStatusOpen, nil)
}

func (s *Service) Reject(ctx context.Context, adminID uuid.UUID, role string, id uuid.UUID, reason string) (domain.ReviewPosting, error) {
	if err := requireAdmin(role); err != nil {
		return domain.ReviewPosting{}, err
	}
	reason, err := normalizeReason(reason)
	if err != nil {
		return domain.ReviewPosting{}, err
	}
	return s.decide(ctx, adminID, id, domain.PostingStatusRejected, &reason)
}

// decide applies the transition. When the posting is no longer pending, repeating the decision it
// already has succeeds with the current state, so a client retry after a lost response is safe.
func (s *Service) decide(ctx context.Context, adminID, id uuid.UUID, status domain.PostingStatus, reason *string) (domain.ReviewPosting, error) {
	err := s.repository.Decide(ctx, id, status, reason)
	if err == nil {
		// Log before re-reading: the decision is already committed, and a failed read followed by an
		// idempotent retry would otherwise leave it with no audit line at all.
		s.log.WithContext(ctx).Info("job posting moderated",
			logger.String("admin_id", adminID.String()),
			logger.String("job_posting_id", id.String()),
			logger.String("status", string(status)),
		)
		return s.review(ctx, id)
	}
	if !errors.Is(err, domain.ErrNotPending) {
		return domain.ReviewPosting{}, err
	}
	posting, err := s.repository.Get(ctx, id)
	if err != nil {
		return domain.ReviewPosting{}, err
	}
	switch posting.Status {
	case status:
		return s.withText(ctx, posting)
	case domain.PostingStatusPending:
		if !posting.ReviewSubmitted {
			return domain.ReviewPosting{}, domain.ErrNotSubmitted
		}
		// Submitted and still pending, so the only guard that failed is the non-empty bank.
		return domain.ReviewPosting{}, domain.ErrQuestionBankNotReady
	default:
		return domain.ReviewPosting{}, domain.ErrNotPending
	}
}

func (s *Service) review(ctx context.Context, id uuid.UUID) (domain.ReviewPosting, error) {
	posting, err := s.repository.Get(ctx, id)
	if err != nil {
		return domain.ReviewPosting{}, err
	}
	return s.withText(ctx, posting)
}

// withText fills the JD the Admin reviews. A posting with a PDF gets a short-lived link to it plus
// its stored text; one with stored text but no file (an empty content_path) gets the text alone, with
// no link. Rows created before PDF upload hold a plain-text object, which is read back; older rows
// hold a full URL, which the Admin opens instead. A missing object leaves RawText null so the Admin
// still sees the rest of the posting and can reject it; any other storage failure fails the request.
func (s *Service) withText(ctx context.Context, posting domain.ReviewPosting) (domain.ReviewPosting, error) {
	if s.evaluation != nil {
		weights, err := s.evaluation.ModerationWeights(ctx, adminRole, posting.ID)
		if err != nil {
			return domain.ReviewPosting{}, err
		}
		weights.PostingStatus = string(posting.Status)
		posting.Evaluation = &weights
	}
	if posting.StoredText != nil {
		posting.RawText = posting.StoredText
		if posting.ContentPath == "" {
			return posting, nil
		}
		link, err := s.texts.PresignGet(ctx, posting.ContentPath, documentLinkTTL)
		if err != nil {
			return domain.ReviewPosting{}, apperror.Wrap(apperror.CodeInternal, "job posting file storage failed", err)
		}
		posting.DocumentURL, posting.DocumentExpiresAt = &link.URL, &link.ExpiresAt
		return posting, nil
	}
	if isWebURL(posting.ContentPath) {
		return posting, nil
	}
	text, err := s.readText(ctx, posting.ContentPath)
	if errors.Is(err, storage.ErrNotFound) || errors.Is(err, storage.ErrInvalidInput) {
		s.log.WithContext(ctx).Warn("job posting JD text not found",
			logger.String("job_posting_id", posting.ID.String()),
		)
		return posting, nil
	}
	if err != nil {
		return domain.ReviewPosting{}, apperror.Wrap(apperror.CodeInternal, "job posting file storage failed", err)
	}
	posting.RawText = &text
	return posting, nil
}

func (s *Service) readText(ctx context.Context, key string) (string, error) {
	object, err := s.texts.Download(ctx, key)
	if err != nil {
		return "", err
	}
	defer func() { _ = object.Body.Close() }()
	text, err := io.ReadAll(io.LimitReader(object.Body, maxJDTextBytes+1))
	if err != nil {
		return "", err
	}
	if len(text) > maxJDTextBytes {
		return "", fmt.Errorf("stored JD text exceeds %d bytes", maxJDTextBytes)
	}
	return string(text), nil
}

func isWebURL(contentPath string) bool {
	u, err := url.Parse(contentPath)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}
