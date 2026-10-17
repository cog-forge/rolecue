package repository

import (
	"context"
	"errors"

	"github.com/cog-forge/rolecue/api/internal/features/jobposting/domain"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository stores a Job Posting across job_descriptions, requirements and job_postings.
// Writes touch several tables, so they run in one transaction.
type Repository struct {
	pool    *pgxpool.Pool
	queries *Queries
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool, queries: New(pool)}
}

// Create returns the stored posting built inside the transaction, so a successful insert is never
// reported as a failure by a read after commit (a client retry would then create a duplicate).
func (r *Repository) Create(ctx context.Context, recruiterID uuid.UUID, input domain.NewPosting) (domain.Record, error) {
	var record domain.Record
	err := pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		q := r.queries.WithTx(tx)
		avatar, voice, err := checkReferences(ctx, q, recruiterID, &input.RequiredAvatarID, &input.RequiredVoiceID)
		if err != nil {
			return err
		}
		levelID, err := levelID(ctx, q, input.SeniorityLevel)
		if err != nil {
			return err
		}
		jdID, err := q.InsertJobDescription(ctx, InsertJobDescriptionParams{ContentPath: &input.ContentPath, RawText: &input.RawText, LevelID: levelID})
		if err != nil {
			return err
		}
		if err := saveRequirements(ctx, q, jdID, input.Requirements); err != nil {
			return err
		}
		row, err := q.InsertJobPosting(ctx, InsertJobPostingParams{
			JdID: jdID, RecruiterID: recruiterID, Title: input.Title, Location: input.Location,
			EmploymentType: input.EmploymentType, SalaryMin: input.SalaryMin, SalaryMax: input.SalaryMax,
			InterviewDuration: input.InterviewDuration, RequiredAvatarID: input.RequiredAvatarID,
			RequiredVoiceID: input.RequiredVoiceID,
		})
		if err != nil {
			return err
		}
		record = domain.Record{ContentPath: input.ContentPath, StoredText: &input.RawText, JobPosting: domain.JobPosting{
			ID: row.ID, Title: input.Title, Status: domain.StatusPending, SeniorityLevel: input.SeniorityLevel,
			Requirements: input.Requirements, Location: input.Location, EmploymentType: input.EmploymentType,
			SalaryMin: input.SalaryMin, SalaryMax: input.SalaryMax, InterviewDuration: input.InterviewDuration,
			RequiredAvatar: &domain.Ref{ID: input.RequiredAvatarID, Name: avatar},
			RequiredVoice:  &domain.Ref{ID: input.RequiredVoiceID, Name: voice}, CreatedAt: row.CreatedAt.Time,
		}}
		return nil
	})
	if err != nil {
		return domain.Record{}, persistenceError(err)
	}
	return record, nil
}

func (r *Repository) List(ctx context.Context, recruiterID uuid.UUID, filter domain.ListFilter) ([]domain.Summary, int64, error) {
	var status *string
	if filter.Status != nil {
		s := string(*filter.Status)
		status = &s
	}
	rows, err := r.queries.ListOwnedPostings(ctx, ListOwnedPostingsParams{
		RecruiterID: recruiterID, Status: status, Search: filter.Search, PageLimit: filter.Limit, PageOffset: filter.Offset,
	})
	if err != nil {
		return nil, 0, persistenceError(err)
	}
	total, err := r.queries.CountOwnedPostings(ctx, CountOwnedPostingsParams{RecruiterID: recruiterID, Status: status, Search: filter.Search})
	if err != nil {
		return nil, 0, persistenceError(err)
	}
	out := make([]domain.Summary, 0, len(rows))
	for _, row := range rows {
		out = append(out, domain.Summary{
			ID: row.ID, Title: row.Title, Status: domain.Status(row.Status), RejectReason: row.RejectReason,
			Location: row.Location, EmploymentType: row.EmploymentType, InterviewSlot: row.InterviewSlot,
			CreatedAt: row.CreatedAt.Time,
		})
	}
	return out, total, nil
}

func (r *Repository) Get(ctx context.Context, recruiterID, id uuid.UUID) (domain.Record, error) {
	row, err := r.queries.GetOwnedPosting(ctx, GetOwnedPostingParams{ID: id, RecruiterID: recruiterID})
	if err != nil {
		return domain.Record{}, persistenceError(err)
	}
	requirements, err := r.queries.ListRequirements(ctx, row.JdID)
	if err != nil {
		return domain.Record{}, persistenceError(err)
	}
	record := domain.Record{ContentPath: storedPath(row.ContentPath), StoredText: row.RawText, JobPosting: domain.JobPosting{
		ID: row.ID, Title: row.Title, Status: domain.Status(row.Status), RejectReason: row.RejectReason,
		SeniorityLevel: domain.Level(row.LevelName), Location: row.Location, EmploymentType: row.EmploymentType,
		SalaryMin: row.SalaryMin, SalaryMax: row.SalaryMax, InterviewDuration: row.InterviewDuration,
		InterviewSlot: row.InterviewSlot, RequiredAvatar: namedRef(row.RequiredAvatarID, row.AvatarName),
		RequiredVoice: namedRef(row.RequiredVoiceID, row.VoiceName), CoreQuestionCount: row.CoreQuestionCount,
		CreatedAt: row.CreatedAt.Time, Requirements: make([]domain.PostingRequirement, 0, len(requirements)),
	}}
	for _, req := range requirements {
		record.Requirements = append(record.Requirements, domain.PostingRequirement{Name: req.Name, Target: req.Target, Type: domain.RequirementType(req.Type)})
	}
	return record, nil
}

// Update applies a validated patch to a pending or rejected posting. document, when set, is a
// replacement JD PDF and its text (rejected postings only). The row is locked first; the question
// count and current requirements are read by later statements, so they see whatever a concurrent
// question-bank save committed before the lock was granted.
func (r *Repository) Update(ctx context.Context, recruiterID, id uuid.UUID, patch domain.Edit, document *domain.NewDocument) error {
	err := pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		q := r.queries.WithTx(tx)
		current, err := q.LockOwnedPosting(ctx, LockOwnedPostingParams{ID: id, RecruiterID: recruiterID})
		if err != nil {
			return err
		}
		status := domain.Status(current.Status)
		if status != domain.StatusPending && status != domain.StatusRejected {
			return domain.ErrNotEditable
		}
		if document != nil && status != domain.StatusRejected {
			return domain.ErrTextLocked
		}
		if err := domain.CheckSalary(merged(patch.SalaryMin, current.SalaryMin), merged(patch.SalaryMax, current.SalaryMax)); err != nil {
			return err
		}
		// Resending the voice the posting already has is not a new selection, so it stays valid
		// even if an Admin has deactivated that voice since.
		if patch.RequiredVoiceID.Value != nil && *patch.RequiredVoiceID.Value == current.RequiredVoiceID {
			patch.RequiredVoiceID = domain.UUIDChange{}
		}
		if _, _, err := checkReferences(ctx, q, recruiterID, patch.RequiredAvatarID.Value, patch.RequiredVoiceID.Value); err != nil {
			return err
		}
		if err := dropUnchangedBankInputs(ctx, q, current.JdID, &patch); err != nil {
			return err
		}
		if patch.SeniorityLevel.Present || patch.Requirements.Present || document != nil {
			if err := guardQuestionBank(ctx, q, current.JdID, status); err != nil {
				return err
			}
		}
		if patch.SeniorityLevel.Present {
			level, err := levelID(ctx, q, domain.Level(*patch.SeniorityLevel.Value))
			if err != nil {
				return err
			}
			if err := q.UpdateJDLevel(ctx, UpdateJDLevelParams{ID: current.JdID, LevelID: level}); err != nil {
				return err
			}
		}
		if patch.Requirements.Present {
			if err := q.DeleteRequirements(ctx, current.JdID); err != nil {
				return err
			}
			if err := saveRequirements(ctx, q, current.JdID, *patch.Requirements.Value); err != nil {
				return err
			}
		}
		if document != nil {
			if err := q.UpdateJDDocument(ctx, UpdateJDDocumentParams{ID: current.JdID, ContentPath: &document.ContentPath, RawText: &document.RawText}); err != nil {
				return err
			}
		}
		return q.UpdatePosting(ctx, updateParams(recruiterID, id, patch))
	})
	return persistenceError(err)
}

// Resubmit sends a rejected posting back for Admin review. Repeating it on a posting that is
// already pending changes nothing; any other status cannot be resubmitted.
func (r *Repository) Resubmit(ctx context.Context, recruiterID, id uuid.UUID) error {
	err := pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		q := r.queries.WithTx(tx)
		current, err := q.LockOwnedPosting(ctx, LockOwnedPostingParams{ID: id, RecruiterID: recruiterID})
		if err != nil {
			return err
		}
		switch domain.Status(current.Status) {
		case domain.StatusPending:
			return nil
		case domain.StatusRejected:
			return q.ResubmitPosting(ctx, ResubmitPostingParams{ID: id, RecruiterID: recruiterID})
		default:
			return domain.ErrNotRejected
		}
	})
	return persistenceError(err)
}

// guardQuestionBank keeps the bank from outliving the JD it was generated from. While pending, the
// change is refused until the Recruiter deletes the questions. A rejected posting's bank cannot be
// edited at all (the bank endpoints need a pending posting), so the outdated bank is cleared here,
// in the same transaction; the Admin's non-empty-bank check then blocks approval until the
// Recruiter regenerates it after resubmitting.
func guardQuestionBank(ctx context.Context, q *Queries, jdID uuid.UUID, status domain.Status) error {
	count, err := q.CountCoreQuestions(ctx, jdID)
	if err != nil || count == 0 {
		return err
	}
	if status == domain.StatusPending {
		return domain.ErrRequirementsLocked
	}
	return q.DeleteCoreQuestions(ctx, jdID)
}

// dropUnchangedBankInputs removes seniority and requirements from the patch when they equal the
// stored values, so only real changes reach guardQuestionBank.
func dropUnchangedBankInputs(ctx context.Context, q *Queries, jdID uuid.UUID, patch *domain.Edit) error {
	if patch.SeniorityLevel.Present {
		level, err := q.GetJDLevelName(ctx, jdID)
		if err != nil {
			return err
		}
		if level == *patch.SeniorityLevel.Value {
			patch.SeniorityLevel = domain.StringChange{}
		}
	}
	if patch.Requirements.Present {
		rows, err := q.ListRequirements(ctx, jdID)
		if err != nil {
			return err
		}
		current := make([]domain.PostingRequirement, 0, len(rows))
		for _, row := range rows {
			current = append(current, domain.PostingRequirement{Name: row.Name, Target: row.Target, Type: domain.RequirementType(row.Type)})
		}
		if domain.SameRequirements(current, *patch.Requirements.Value) {
			patch.Requirements = domain.RequirementsChange{}
		}
	}
	return nil
}

func updateParams(recruiterID, id uuid.UUID, patch domain.Edit) UpdatePostingParams {
	p := UpdatePostingParams{
		ID: id, RecruiterID: recruiterID,
		SetTitle: patch.Title.Present, SetLocation: patch.Location.Present, Location: patch.Location.Value,
		SetEmploymentType: patch.EmploymentType.Present, EmploymentType: patch.EmploymentType.Value,
		SetSalaryMin: patch.SalaryMin.Present, SalaryMin: patch.SalaryMin.Value,
		SetSalaryMax: patch.SalaryMax.Present, SalaryMax: patch.SalaryMax.Value,
		SetInterviewDuration: patch.InterviewDuration.Present, InterviewDuration: patch.InterviewDuration.Value,
		SetAvatar: patch.RequiredAvatarID.Present, SetVoice: patch.RequiredVoiceID.Present,
	}
	if patch.Title.Value != nil {
		p.Title = *patch.Title.Value
	}
	if patch.RequiredAvatarID.Value != nil {
		p.RequiredAvatarID = *patch.RequiredAvatarID.Value
	}
	if patch.RequiredVoiceID.Value != nil {
		p.RequiredVoiceID = *patch.RequiredVoiceID.Value
	}
	return p
}

// merged is the value a field will have after the patch.
func merged(change domain.Int32Change, current *int32) *int32 {
	if change.Present {
		return change.Value
	}
	return current
}

// checkReferences validates the presentation the Candidate will be interviewed with and returns
// the names. A nil ID is not being changed. Both checks run inside the write transaction.
func checkReferences(ctx context.Context, q *Queries, recruiterID uuid.UUID, avatarID, voiceID *uuid.UUID) (avatar, voice *string, err error) {
	if avatarID != nil {
		name, err := q.OwnedAvatarName(ctx, OwnedAvatarNameParams{AvatarID: *avatarID, UserID: recruiterID})
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil, domain.ErrAvatarNotOwned
		}
		if err != nil {
			return nil, nil, err
		}
		avatar = &name
	}
	if voiceID != nil {
		name, err := q.ActiveVoiceName(ctx, *voiceID)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil, domain.ErrVoiceNotActive
		}
		if err != nil {
			return nil, nil, err
		}
		voice = &name
	}
	return avatar, voice, nil
}

func levelID(ctx context.Context, q *Queries, level domain.Level) (uuid.UUID, error) {
	id, err := q.GetLevelIDByName(ctx, string(level))
	if errors.Is(err, pgx.ErrNoRows) {
		// Migration 000007 seeds every level, so a miss is a deployment problem.
		return uuid.Nil, apperror.Wrap(apperror.CodeInternal, "job posting persistence failed", errors.New("seniority level is not seeded: "+string(level)))
	}
	return id, err
}

func saveRequirements(ctx context.Context, q *Queries, jdID uuid.UUID, rows []domain.PostingRequirement) error {
	if len(rows) == 0 {
		return nil
	}
	params := InsertRequirementsParams{JdID: jdID}
	for _, row := range rows {
		params.Names = append(params.Names, row.Name)
		params.Targets = append(params.Targets, row.Target)
		params.Types = append(params.Types, string(row.Type))
	}
	return q.InsertRequirements(ctx, params)
}

// storedPath reads job_descriptions.content_path, which migration 000010 made nullable for
// Role Profiles that keep raw_text instead. A Job Posting always stores its JD as an object, so
// a missing key only shows up as a storage error when the JD is read.
func storedPath(path *string) string {
	if path == nil {
		return ""
	}
	return *path
}

// namedRef turns a nullable reference into nil. The uuid override maps NULL to uuid.Nil.
func namedRef(id uuid.UUID, name *string) *domain.Ref {
	if id == uuid.Nil {
		return nil
	}
	return &domain.Ref{ID: id, Name: name}
}

func persistenceError(err error) error {
	if err == nil {
		return nil
	}
	var appErr *apperror.AppError
	if errors.As(err, &appErr) {
		return err
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return apperror.Wrap(apperror.CodeJobPostingNotFound, "job posting not found", err)
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23503" { // foreign_key_violation
		switch pgErr.ConstraintName {
		case "job_postings_required_avatar_id_fkey", "job_postings_required_voice_id_fkey":
			// The avatar or voice was deleted between the check and the write.
			return apperror.Wrap(apperror.CodeValidation, "the selected avatar or voice no longer exists", err)
		}
	}
	return apperror.Wrap(apperror.CodeInternal, "job posting persistence failed", err)
}
