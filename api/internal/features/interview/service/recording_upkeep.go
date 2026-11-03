package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/cog-forge/rolecue/api/internal/features/interview/domain"
	"github.com/cog-forge/rolecue/api/internal/pkg/logger"
	"github.com/cog-forge/rolecue/api/internal/pkg/storage"
	"github.com/google/uuid"
)

// RecordingUpkeepRepository is the persistence the System Handler's recording jobs need. Every query is
// restricted to recruitment interviews, so practice interviews are never touched.
type RecordingUpkeepRepository interface {
	FailStaleRecordings(ctx context.Context, noneBefore, uploadingBefore time.Time) ([]domain.RecordingChange, error)
	ListExpiredRecordings(ctx context.Context, decidedBefore time.Time, limit int32) ([]domain.StoredRecording, error)
	ClearDeletedRecording(ctx context.Context, id uuid.UUID, key string) (bool, error)
}

// RecordingDeleter deletes a stored object. storage.Store satisfies it.
type RecordingDeleter interface {
	Delete(ctx context.Context, key string) error
}

// purgeBatch bounds the recordings one retention pass deletes; the next pass takes the rest.
const purgeBatch = 100

// RecordingUpkeep runs the System Handler's recording jobs (KAN-272): it fails recordings that never
// arrived and purges recordings past retention. NoneGrace is how long after completion a recording may
// stay none before it is failed (the room asks for an upload grant right after completion); an upload
// still uploading after the grant's lifetime (domain.RecordingUploadTTL) is failed too. Logs carry only
// interview ids, statuses and counts, never an object key or a link.
type RecordingUpkeep struct {
	repository RecordingUpkeepRepository
	store      RecordingDeleter
	noneGrace  time.Duration
}

func NewRecordingUpkeep(repository RecordingUpkeepRepository, store RecordingDeleter, noneGrace time.Duration) (*RecordingUpkeep, error) {
	if repository == nil || store == nil {
		return nil, errors.New("recording upkeep needs a repository and a store")
	}
	if noneGrace <= 0 {
		return nil, errors.New("recording upkeep needs a positive grace period")
	}
	return &RecordingUpkeep{repository: repository, store: store, noneGrace: noneGrace}, nil
}

// FailStale marks failed the recording of every completed recruitment interview that is still none
// NoneGrace after completion, or still uploading RecordingUploadTTL after completion, so the Recruiter
// sees that the recording failed instead of "no recording was uploaded". A failed recording still takes a
// new upload grant, so a late upload from the Candidate's browser is not lost. Running it again changes
// nothing. It returns how many recordings it failed.
func (u *RecordingUpkeep) FailStale(ctx context.Context, now time.Time) (int, error) {
	changes, err := u.repository.FailStaleRecordings(ctx, now.Add(-u.noneGrace), now.Add(-domain.RecordingUploadTTL))
	if err != nil {
		return 0, err
	}
	log := logger.FromContext(ctx)
	for _, change := range changes {
		log.Info("recording status changed",
			logger.Op("interview.recording_status"),
			logger.String("interview_id", change.InterviewID.String()),
			logger.String("from_status", string(change.From)),
			logger.String("to_status", string(change.To)),
			logger.String("reason", "stale"),
		)
	}
	return len(changes), nil
}

// PurgeExpired deletes the stored recording of every recruitment interview whose Application reached its
// final decision more than RecordingRetention before now, then clears its key and marks it deleted. The
// object is deleted first: a failed delete leaves the row as it was, so the next run tries again; a crash
// between the delete and the row update deletes nothing new next time (a missing object counts as
// deleted). Undecided Applications keep their recording. It returns how many recordings it purged; a
// failure on one recording is logged and does not stop the others.
func (u *RecordingUpkeep) PurgeExpired(ctx context.Context, now time.Time) (int, error) {
	expired, err := u.repository.ListExpiredRecordings(ctx, now.Add(-domain.RecordingRetention), purgeBatch)
	if err != nil {
		return 0, err
	}
	log := logger.FromContext(ctx)
	purged, failed := 0, 0
	for _, recording := range expired {
		if err := u.purge(ctx, recording); err != nil {
			if ctx.Err() != nil {
				return purged, ctx.Err()
			}
			failed++
			log.Warn("expired recording not purged; the next run retries",
				logger.Op("interview.recording_purge"),
				logger.String("interview_id", recording.InterviewID.String()),
				logger.Cause(err),
			)
			continue
		}
		purged++
		log.Info("recording status changed",
			logger.Op("interview.recording_status"),
			logger.String("interview_id", recording.InterviewID.String()),
			logger.String("to_status", string(domain.RecordDeleted)),
			logger.String("reason", "retention"),
		)
	}
	if failed > 0 {
		return purged, fmt.Errorf("%d of %d expired recordings could not be purged", failed, len(expired))
	}
	return purged, nil
}

func (u *RecordingUpkeep) purge(ctx context.Context, recording domain.StoredRecording) error {
	if err := u.store.Delete(ctx, recording.Key); err != nil && !errors.Is(err, storage.ErrNotFound) {
		// The storage error is wrapped without the key: the adapters do not put keys in their errors.
		return fmt.Errorf("delete the recording object: %w", err)
	}
	if _, err := u.repository.ClearDeletedRecording(ctx, recording.InterviewID, recording.Key); err != nil {
		return err
	}
	return nil
}
