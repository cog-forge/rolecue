package repository

import (
	"context"
	"errors"
	"time"

	"github.com/cog-forge/rolecue/api/internal/features/interview/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// GetCandidateRecording reads the recording state of one of the Candidate's interviews. Another
// Candidate's interview is not found.
func (r *Repository) GetCandidateRecording(ctx context.Context, candidateID, id uuid.UUID) (domain.RecordingState, error) {
	row, err := r.queries.GetCandidateRecordingState(ctx, GetCandidateRecordingStateParams{ID: id, CandidateID: candidateID})
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.RecordingState{}, domain.ErrInterviewNotFound
	}
	if err != nil {
		return domain.RecordingState{}, internal(err)
	}
	return recordingState(row.ID, row.Type, row.RecordStatus, row.RecordPath, row.Completed), nil
}

// SetRecordingUpload stores the key of a new upload grant and marks the recording uploading. It reports
// false when the recording is no longer in a state a grant may replace.
func (r *Repository) SetRecordingUpload(ctx context.Context, candidateID, id uuid.UUID, key string) (bool, error) {
	updated, err := r.queries.SetRecordingUpload(ctx, SetRecordingUploadParams{
		RecordPath: &key, ID: id, CandidateID: candidateID,
	})
	if err != nil {
		return false, internal(err)
	}
	return updated == 1, nil
}

// SetRecordingResult moves the upload at key to a final status. It reports false when that upload is no
// longer the current one, so a stale check cannot overwrite a newer grant.
func (r *Repository) SetRecordingResult(ctx context.Context, candidateID, id uuid.UUID, key string, status domain.RecordStatus) (bool, error) {
	updated, err := r.queries.SetRecordingResult(ctx, SetRecordingResultParams{
		RecordStatus: string(status), ID: id, RecordPath: &key, CandidateID: candidateID,
	})
	if err != nil {
		return false, internal(err)
	}
	return updated == 1, nil
}

// RecruiterOwnsApplication reports whether the Application belongs to a posting of the Recruiter under
// the given Job Posting. Missing and foreign Applications are both false.
func (r *Repository) RecruiterOwnsApplication(ctx context.Context, recruiterID, jobPostingID, applicationID uuid.UUID) (bool, error) {
	owned, err := r.queries.RecruiterOwnsApplication(ctx, RecruiterOwnsApplicationParams{
		ID: applicationID, JobPostingID: jobPostingID, RecruiterID: recruiterID,
	})
	if err != nil {
		return false, internal(err)
	}
	return owned, nil
}

// GetApplicationRecording reads the interview of an Application. An Application without an interview is
// not found.
func (r *Repository) GetApplicationRecording(ctx context.Context, applicationID uuid.UUID) (domain.RecordingState, error) {
	row, err := r.queries.GetApplicationRecordingState(ctx, &applicationID)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.RecordingState{}, domain.ErrInterviewNotFound
	}
	if err != nil {
		return domain.RecordingState{}, internal(err)
	}
	return recordingState(row.ID, row.Type, row.RecordStatus, row.RecordPath, row.Completed), nil
}

// ListTranscript returns the turns of an interview in position order. The answer is nil when the
// question was never answered.
func (r *Repository) ListTranscript(ctx context.Context, interviewID uuid.UUID) ([]domain.TranscriptTurn, error) {
	rows, err := r.queries.ListRecruitmentTurns(ctx, interviewID)
	if err != nil {
		return nil, internal(err)
	}
	turns := make([]domain.TranscriptTurn, 0, len(rows))
	for _, row := range rows {
		turns = append(turns, domain.TranscriptTurn{
			Position: row.Position, QuestionNumber: row.QuestionNumber, Kind: domain.TurnKind(row.Kind),
			Question: row.Question, Answer: row.Answer,
		})
	}
	return turns, nil
}

// FailStaleRecordings marks failed the recordings of completed recruitment interviews still none since
// before noneBefore or uploading since before uploadingBefore (both against the completion time) and
// returns what changed. Rows another transaction holds are skipped and picked up by the next run.
func (r *Repository) FailStaleRecordings(ctx context.Context, noneBefore, uploadingBefore time.Time) ([]domain.RecordingChange, error) {
	rows, err := r.queries.FailStaleRecordings(ctx, FailStaleRecordingsParams{
		NoneBefore:      pgtype.Timestamptz{Time: noneBefore, Valid: true},
		UploadingBefore: pgtype.Timestamptz{Time: uploadingBefore, Valid: true},
	})
	if err != nil {
		return nil, internal(err)
	}
	changes := make([]domain.RecordingChange, 0, len(rows))
	for _, row := range rows {
		changes = append(changes, domain.RecordingChange{
			InterviewID: row.ID, From: domain.RecordStatus(row.PreviousStatus), To: domain.RecordFailed,
		})
	}
	return changes, nil
}

// ListExpiredRecordings returns up to limit stored recordings whose Application reached its final decision
// before decidedBefore.
func (r *Repository) ListExpiredRecordings(ctx context.Context, decidedBefore time.Time, limit int32) ([]domain.StoredRecording, error) {
	rows, err := r.queries.ListExpiredRecordings(ctx, ListExpiredRecordingsParams{
		DecidedBefore: pgtype.Timestamptz{Time: decidedBefore, Valid: true},
		BatchSize:     limit,
	})
	if err != nil {
		return nil, internal(err)
	}
	recordings := make([]domain.StoredRecording, 0, len(rows))
	for _, row := range rows {
		recordings = append(recordings, domain.StoredRecording{InterviewID: row.ID, Key: row.RecordPath})
	}
	return recordings, nil
}

// ClearDeletedRecording clears the key of a recording whose object was deleted and marks it deleted. It
// reports false when the key is no longer the interview's (already cleared).
func (r *Repository) ClearDeletedRecording(ctx context.Context, id uuid.UUID, key string) (bool, error) {
	updated, err := r.queries.ClearDeletedRecording(ctx, ClearDeletedRecordingParams{ID: id, RecordPath: &key})
	if err != nil {
		return false, internal(err)
	}
	return updated == 1, nil
}

func recordingState(id uuid.UUID, interviewType, status string, path *string, completed bool) domain.RecordingState {
	return domain.RecordingState{
		InterviewID:  id,
		Type:         domain.Type(interviewType),
		RecordStatus: domain.RecordStatus(status),
		RecordPath:   path,
		Completed:    completed,
	}
}
