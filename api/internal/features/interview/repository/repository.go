package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/cog-forge/rolecue/api/internal/features/interview/domain"
	"github.com/cog-forge/rolecue/api/internal/pkg/ledger"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository stores interview sessions. Every write runs in one transaction. A recruitment start locks
// the posting row and then reads the Application, the order screening, the final decision and the expiry
// pass use, so they never wait on each other the other way round; it takes no wallet lock because the
// Candidate is not charged. A practice start locks only the Candidate's wallet row (through the ledger).
// Turns and completion lock the session row.
type Repository struct {
	pool    *pgxpool.Pool
	queries *Queries
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool, queries: New(pool)}
}

// Create starts a session and reports whether it was created. A recruitment start returns the existing
// session for its Application without writing anything, so a repeat is a no-op. A practice start charges
// the Candidate once, in the same transaction as the rows it creates.
func (r *Repository) Create(ctx context.Context, params domain.CreateParams) (domain.Session, bool, error) {
	var session domain.Session
	created := false
	err := pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		q := r.queries.WithTx(tx)
		var interviewID uuid.UUID
		var err error
		if params.Type == domain.TypeRecruitment {
			interviewID, created, err = startRecruitment(ctx, q, params)
		} else {
			interviewID, err = startPractice(ctx, tx, q, params)
			created = err == nil
		}
		if err != nil {
			return err
		}
		session, err = readSession(ctx, q, params.CandidateID, interviewID)
		return err
	})
	if err != nil {
		return domain.Session{}, false, internal(err)
	}
	return session, created, nil
}

// Get reads one of the Candidate's sessions. Another Candidate's session is not found.
func (r *Repository) Get(ctx context.Context, candidateID, id uuid.UUID) (domain.Session, error) {
	var session domain.Session
	err := pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		var err error
		session, err = readSession(ctx, r.queries.WithTx(tx), candidateID, id)
		return err
	})
	if err != nil {
		return domain.Session{}, internal(err)
	}
	return session, nil
}

// AppendTurn stores the answer to the next unanswered question. A retry of an answer already stored
// returns the session unchanged. The rules are in domain.ClassifyTurn and run under the session lock.
// The turn stores the question text as it was asked.
func (r *Repository) AppendTurn(ctx context.Context, candidateID, id uuid.UUID, position int32, answer string) (domain.Session, error) {
	var session domain.Session
	err := pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		q := r.queries.WithTx(tx)
		header, err := lockSession(ctx, q, candidateID, id)
		if err != nil {
			return err
		}
		if header.CompletedAt.Valid {
			return domain.ErrInterviewCompleted
		}
		turns, err := listTurns(ctx, q, id)
		if err != nil {
			return err
		}
		questionCount, err := q.CountInterviewQuestions(ctx, id)
		if err != nil {
			return internal(err)
		}
		unchanged, err := domain.ClassifyTurn(turns, int(questionCount), position, answer)
		if err != nil {
			return err
		}
		if !unchanged {
			question, err := q.GetInterviewQuestion(ctx, GetInterviewQuestionParams{InterviewID: id, Position: position})
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.ErrTurnConflict
			}
			if err != nil {
				return internal(err)
			}
			err = q.InsertInterviewTurn(ctx, InsertInterviewTurnParams{
				InterviewID: id, Position: position, Question: question.Content, Answer: &answer,
			})
			if err != nil {
				return internal(err)
			}
		}
		session, err = readSession(ctx, q, candidateID, id)
		return err
	})
	if err != nil {
		return domain.Session{}, internal(err)
	}
	return session, nil
}

// Complete sets completed_at once and, for a recruitment session, moves its Application from
// interview_eligible to interviewed in the same transaction. The bool is true only for the call that
// completed the session, so the caller starts the evaluation once.
func (r *Repository) Complete(ctx context.Context, candidateID, id uuid.UUID) (domain.Session, bool, error) {
	var session domain.Session
	firstCompletion := false
	err := pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		q := r.queries.WithTx(tx)
		header, err := lockSession(ctx, q, candidateID, id)
		if err != nil {
			return err
		}
		if !header.CompletedAt.Valid {
			firstCompletion = true
			if err := q.SetInterviewCompleted(ctx, id); err != nil {
				return internal(err)
			}
			if header.ApplicationID != nil {
				if err := q.MarkApplicationInterviewed(ctx, *header.ApplicationID); err != nil {
					return internal(err)
				}
			}
		}
		session, err = readSession(ctx, q, candidateID, id)
		return err
	})
	if err != nil {
		return domain.Session{}, false, internal(err)
	}
	return session, firstCompletion, nil
}

// startPractice creates a practice session from a confirmed role profile of the Candidate and charges the
// practice price when it is set. A failed charge rolls back the rows written before it.
func startPractice(ctx context.Context, tx pgx.Tx, q *Queries, p domain.CreateParams) (uuid.UUID, error) {
	profile, err := q.GetOwnedRoleProfile(ctx, GetOwnedRoleProfileParams{ID: *p.RoleProfileID, CandidateID: p.CandidateID})
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, domain.ErrRoleProfileNotFound
	}
	if err != nil {
		return uuid.Nil, internal(err)
	}
	if profile.Status != domain.RoleProfileConfirmed {
		return uuid.Nil, domain.ErrRoleProfileRequired
	}
	questions, err := startQuestions(ctx, q, profile.JdID, p.QuestionLimit)
	if err != nil {
		return uuid.Nil, err
	}
	if err := checkReferences(ctx, q, p.AvatarID, p.VoiceID); err != nil {
		return uuid.Nil, err
	}
	interviewID, err := createInterviewRow(ctx, q, p.Type, profile.ID, nil, p.AvatarID, p.VoiceID)
	if err != nil {
		return uuid.Nil, err
	}
	if err := insertQuestions(ctx, q, interviewID, questions); err != nil {
		return uuid.Nil, err
	}
	if p.PracticeCoins > 0 {
		description := fmt.Sprintf("Practice interview %s", interviewID)
		if err := ledger.Debit(ctx, tx, p.CandidateID, p.PracticeCoins, description); err != nil {
			return uuid.Nil, err
		}
	}
	return interviewID, nil
}

// startRecruitment creates a recruitment session for an interview_eligible Application inside its
// deadline. It returns the existing session, with created false, when one is already there. The posting
// lock is taken first, so two starts for one posting queue up and the second finds the first's session.
func startRecruitment(ctx context.Context, q *Queries, p domain.CreateParams) (uuid.UUID, bool, error) {
	postingID, err := q.GetApplicationPosting(ctx, GetApplicationPostingParams{ID: *p.ApplicationID, CandidateID: p.CandidateID})
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, false, domain.ErrApplicationNotFound
	}
	if err != nil {
		return uuid.Nil, false, internal(err)
	}
	posting, err := q.LockPostingForInterview(ctx, postingID)
	if err != nil {
		return uuid.Nil, false, internal(err)
	}
	application, err := q.GetApplicationForInterview(ctx, GetApplicationForInterviewParams{
		ID: *p.ApplicationID, CandidateID: p.CandidateID, JobPostingID: postingID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, false, domain.ErrApplicationNotFound
	}
	if err != nil {
		return uuid.Nil, false, internal(err)
	}
	existing, err := q.FindInterviewByApplication(ctx, p.ApplicationID)
	if err == nil {
		return existing, false, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, false, internal(err)
	}
	consumed, err := q.CountConsumedInterviews(ctx, postingID)
	if err != nil {
		return uuid.Nil, false, internal(err)
	}
	if err := domain.CheckRecruitmentStart(application.Status, application.WithinDeadline, posting.InterviewSlot, consumed); err != nil {
		return uuid.Nil, false, err
	}
	roleProfileID, err := recruitmentRoleProfile(ctx, q, p)
	if err != nil {
		return uuid.Nil, false, err
	}
	questions, err := startQuestions(ctx, q, posting.JdID, p.QuestionLimit)
	if err != nil {
		return uuid.Nil, false, err
	}
	avatarID, voiceID := p.AvatarID, p.VoiceID
	if avatarID == nil {
		avatarID = posting.RequiredAvatarID
	}
	if voiceID == nil {
		voiceID = posting.RequiredVoiceID
	}
	if err := checkReferences(ctx, q, avatarID, voiceID); err != nil {
		return uuid.Nil, false, err
	}
	interviewID, err := createInterviewRow(ctx, q, p.Type, roleProfileID, p.ApplicationID, avatarID, voiceID)
	if err != nil {
		return uuid.Nil, false, err
	}
	if err := insertQuestions(ctx, q, interviewID, questions); err != nil {
		return uuid.Nil, false, err
	}
	return interviewID, true, nil
}

// recruitmentRoleProfile uses the role profile the Candidate named when it is theirs and confirmed.
// Otherwise it uses their most recent confirmed one. A Candidate with none cannot start.
func recruitmentRoleProfile(ctx context.Context, q *Queries, p domain.CreateParams) (uuid.UUID, error) {
	if p.RoleProfileID != nil {
		profile, err := q.GetOwnedRoleProfile(ctx, GetOwnedRoleProfileParams{ID: *p.RoleProfileID, CandidateID: p.CandidateID})
		if err == nil && profile.Status == domain.RoleProfileConfirmed {
			return profile.ID, nil
		}
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return uuid.Nil, internal(err)
		}
	}
	id, err := q.LatestConfirmedRoleProfile(ctx, p.CandidateID)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, domain.ErrRoleProfileRequired
	}
	if err != nil {
		return uuid.Nil, internal(err)
	}
	return id, nil
}

// startQuestions reads the question bank a session asks from. An empty bank is not ready.
func startQuestions(ctx context.Context, q *Queries, jdID uuid.UUID, limit int32) ([]ListStartQuestionsRow, error) {
	questions, err := q.ListStartQuestions(ctx, ListStartQuestionsParams{JdID: jdID, QuestionLimit: limit})
	if err != nil {
		return nil, internal(err)
	}
	if len(questions) == 0 {
		return nil, domain.ErrQuestionBankNotReady
	}
	return questions, nil
}

// checkReferences checks that a chosen avatar and voice exist. Defaults from the posting go through the
// same check, which keeps the rule in one place.
func checkReferences(ctx context.Context, q *Queries, avatarID, voiceID *uuid.UUID) error {
	if avatarID != nil {
		found, err := q.AvatarExists(ctx, *avatarID)
		if err != nil {
			return internal(err)
		}
		if !found {
			return apperror.New(apperror.CodeValidation, "avatar not found")
		}
	}
	if voiceID != nil {
		found, err := q.VoiceExists(ctx, *voiceID)
		if err != nil {
			return internal(err)
		}
		if !found {
			return apperror.New(apperror.CodeValidation, "voice not found")
		}
	}
	return nil
}

func createInterviewRow(ctx context.Context, q *Queries, interviewType domain.Type, roleProfileID uuid.UUID, applicationID, avatarID, voiceID *uuid.UUID) (uuid.UUID, error) {
	id, err := q.InsertInterview(ctx, InsertInterviewParams{
		RoleProfileID: roleProfileID, InterviewType: string(interviewType), ApplicationID: applicationID,
		AvatarID: avatarID, VoiceID: voiceID,
	})
	if err != nil {
		return uuid.Nil, internal(err)
	}
	return id, nil
}

// insertQuestions stores the bank questions as positions 1..n in the order they were read.
func insertQuestions(ctx context.Context, q *Queries, interviewID uuid.UUID, questions []ListStartQuestionsRow) error {
	for i, question := range questions {
		err := q.InsertInterviewQuestion(ctx, InsertInterviewQuestionParams{
			InterviewID: interviewID, Position: int32(i + 1), CoreQuestionID: question.ID,
		})
		if err != nil {
			return internal(err)
		}
	}
	return nil
}

// lockOwnedInterview locks the session row and reads its header. Another Candidate's session is not
// found. The caller holds the lock until its transaction ends.
func lockSession(ctx context.Context, q *Queries, candidateID, id uuid.UUID) (GetOwnedInterviewRow, error) {
	if _, err := q.LockOwnedInterview(ctx, LockOwnedInterviewParams{ID: id, CandidateID: candidateID}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return GetOwnedInterviewRow{}, domain.ErrInterviewNotFound
		}
		return GetOwnedInterviewRow{}, internal(err)
	}
	row, err := q.GetOwnedInterview(ctx, GetOwnedInterviewParams{ID: id, CandidateID: candidateID})
	if err != nil {
		return GetOwnedInterviewRow{}, internal(err)
	}
	return row, nil
}

func listTurns(ctx context.Context, q *Queries, id uuid.UUID) ([]domain.Turn, error) {
	rows, err := q.ListInterviewTurns(ctx, id)
	if err != nil {
		return nil, internal(err)
	}
	turns := make([]domain.Turn, 0, len(rows))
	for _, row := range rows {
		turns = append(turns, domain.Turn{Position: row.Position, Question: row.Question, Answer: row.Answer})
	}
	return turns, nil
}

// readSession builds the full session from the rows, including the turns and the current question. The
// service decides what the Candidate sees.
func readSession(ctx context.Context, q *Queries, candidateID, id uuid.UUID) (domain.Session, error) {
	row, err := q.GetOwnedInterview(ctx, GetOwnedInterviewParams{ID: id, CandidateID: candidateID})
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Session{}, domain.ErrInterviewNotFound
	}
	if err != nil {
		return domain.Session{}, internal(err)
	}
	questionCount, err := q.CountInterviewQuestions(ctx, id)
	if err != nil {
		return domain.Session{}, internal(err)
	}
	turns, err := listTurns(ctx, q, id)
	if err != nil {
		return domain.Session{}, err
	}
	session := domain.Session{
		ID: row.ID, Type: domain.Type(row.Type), RoleProfileID: row.RoleProfileID,
		ApplicationID: row.ApplicationID, AvatarID: row.AvatarID, VoiceID: row.VoiceID,
		StartedAt: row.StartedAt.Time, CompletedAt: timePtr(row.CompletedAt),
		EvaluationStatus: domain.EvaluationStatus(row.EvaluationStatus),
		QuestionCount:    int32(questionCount), AnsweredCount: int32(len(turns)), Turns: &turns,
	}
	if !row.CompletedAt.Valid && int64(len(turns)) < questionCount {
		question, err := q.GetInterviewQuestion(ctx, GetInterviewQuestionParams{InterviewID: id, Position: int32(len(turns)) + 1})
		if err != nil {
			return domain.Session{}, internal(err)
		}
		session.CurrentQuestion = &domain.CurrentQuestion{Position: question.Position, Content: question.Content}
	}
	return session, nil
}

// internal keeps AppErrors raised inside a transaction and maps anything else, such as a failed begin or
// commit, to an internal error.
func internal(err error) error {
	var appErr *apperror.AppError
	if errors.As(err, &appErr) {
		return err
	}
	return apperror.Wrap(apperror.CodeInternal, "interview persistence failed", err)
}

func timePtr(value pgtype.Timestamptz) *time.Time {
	if !value.Valid {
		return nil
	}
	t := value.Time
	return &t
}
