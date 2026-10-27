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
	"github.com/jackc/pgx/v5/pgconn"
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
			interviewID, created, err = startPracticeOnce(ctx, tx, q, params)
		}
		if err != nil {
			return err
		}
		session, err = readSession(ctx, q, params.CandidateID, interviewID)
		return err
	})
	if err != nil && params.StartKey != nil && isUniqueViolation(err) {
		// A concurrent start with the same key won; its transaction (and charge) is the only one kept.
		return r.sessionOfStartKey(ctx, params)
	}
	if err != nil {
		return domain.Session{}, false, internal(err)
	}
	return session, created, nil
}

// sessionOfStartKey returns the session a practice start key created, after losing the insert race.
func (r *Repository) sessionOfStartKey(ctx context.Context, params domain.CreateParams) (domain.Session, bool, error) {
	var session domain.Session
	err := pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		q := r.queries.WithTx(tx)
		id, err := q.FindInterviewByStartKey(ctx, params.StartKey)
		if err != nil {
			return err
		}
		session, err = readSession(ctx, q, params.CandidateID, id)
		return err
	})
	if err != nil {
		return domain.Session{}, false, internal(err)
	}
	return session, false, nil
}

// isUniqueViolation reports a duplicate key, whatever the error was wrapped in.
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
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
		if err := requireOpenWindow(ctx, q, header.ApplicationID); err != nil {
			return err
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

// evaluationStaleAfter is how long a pending evaluation may run before another Complete takes it over.
// It is longer than the service's evaluation timeout, so a live run is never taken over.
const evaluationStaleAfter = 3 * time.Minute

// Complete sets completed_at once and, for a recruitment session, moves its Application from
// interview_eligible to interviewed in the same transaction. The bool is true only for the one call that
// claimed the evaluation (the first completion, a failed evaluation, or a pending one whose run died), so
// the caller starts it once.
func (r *Repository) Complete(ctx context.Context, candidateID, id uuid.UUID) (domain.Session, bool, error) {
	var session domain.Session
	startEvaluation := false
	err := pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		q := r.queries.WithTx(tx)
		header, err := lockSession(ctx, q, candidateID, id)
		if err != nil {
			return err
		}
		if !header.CompletedAt.Valid {
			if err := requireOpenWindow(ctx, q, header.ApplicationID); err != nil {
				return err
			}
			if err := q.SetInterviewCompleted(ctx, id); err != nil {
				return internal(err)
			}
			if header.ApplicationID != nil {
				if err := q.MarkApplicationInterviewed(ctx, *header.ApplicationID); err != nil {
					return internal(err)
				}
			}
		}
		claimed, err := q.ClaimInterviewEvaluation(ctx, ClaimInterviewEvaluationParams{
			ID: id, StaleSeconds: evaluationStaleAfter.Seconds(),
		})
		if err != nil {
			return internal(err)
		}
		startEvaluation = claimed == 1
		session, err = readSession(ctx, q, candidateID, id)
		return err
	})
	if err != nil {
		return domain.Session{}, false, internal(err)
	}
	return session, startEvaluation, nil
}

// MarkEvaluationFailed sets the evaluation status to failed unless it is already completed. The service
// calls it when an evaluation run panicked.
func (r *Repository) MarkEvaluationFailed(ctx context.Context, id uuid.UUID) error {
	if err := r.queries.MarkEvaluationFailed(ctx, id); err != nil {
		return internal(err)
	}
	return nil
}

// startPracticeOnce is startPractice behind two guards: an idempotency key that already created a session
// and an unfinished session for the same role profile both return that session, with created false, and
// charge nothing.
func startPracticeOnce(ctx context.Context, tx pgx.Tx, q *Queries, p domain.CreateParams) (uuid.UUID, bool, error) {
	if p.StartKey != nil {
		existing, err := q.FindInterviewByStartKey(ctx, p.StartKey)
		if err == nil {
			return existing, false, nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return uuid.Nil, false, internal(err)
		}
	}
	// Resuming the Candidate's unfinished session for this role profile is free, whatever the client
	// sent: a double click or a retry never debits twice. The advisory lock serializes concurrent starts.
	if err := q.LockPracticeStart(ctx, p.CandidateID.String()+":"+p.RoleProfileID.String()); err != nil {
		return uuid.Nil, false, internal(err)
	}
	existing, err := q.FindUnfinishedPractice(ctx, p.RoleProfileID)
	if err == nil {
		return existing, false, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, false, internal(err)
	}
	id, err := startPractice(ctx, tx, q, p)
	return id, err == nil, err
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
	if err := checkPracticeReferences(ctx, q, p.CandidateID, p.AvatarID, p.VoiceID); err != nil {
		return uuid.Nil, err
	}
	interviewID, err := createInterviewRow(ctx, q, p.Type, &profile.ID, nil, p.AvatarID, p.VoiceID, p.StartKey)
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
	questions, err := startQuestions(ctx, q, posting.JdID, p.QuestionLimit)
	if err != nil {
		return uuid.Nil, false, err
	}
	// The posting's interviewer and voice are fixed by the Recruiter; the Candidate cannot override them
	// (the service rejects a request that names its own).
	interviewID, err := createInterviewRow(ctx, q, p.Type, nil, p.ApplicationID, posting.RequiredAvatarID, posting.RequiredVoiceID, nil)
	if err != nil {
		return uuid.Nil, false, err
	}
	if err := insertQuestions(ctx, q, interviewID, questions); err != nil {
		return uuid.Nil, false, err
	}
	return interviewID, true, nil
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
func checkPracticeReferences(ctx context.Context, q *Queries, candidateID uuid.UUID, avatarID, voiceID *uuid.UUID) error {
	if avatarID != nil {
		found, err := q.AvatarOwnedBy(ctx, AvatarOwnedByParams{ID: *avatarID, UserID: candidateID})
		if err != nil {
			return internal(err)
		}
		if !found {
			return apperror.New(apperror.CodeValidation, "avatar not found")
		}
	}
	if voiceID != nil {
		found, err := q.VoiceActive(ctx, *voiceID)
		if err != nil {
			return internal(err)
		}
		if !found {
			return apperror.New(apperror.CodeValidation, "voice not found")
		}
	}
	return nil
}

func createInterviewRow(ctx context.Context, q *Queries, interviewType domain.Type, roleProfileID, applicationID, avatarID, voiceID *uuid.UUID, startKey *string) (uuid.UUID, error) {
	id, err := q.InsertInterview(ctx, InsertInterviewParams{
		RoleProfileID: roleProfileID, InterviewType: string(interviewType), ApplicationID: applicationID,
		AvatarID: avatarID, VoiceID: voiceID, StartKey: startKey,
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

// requireOpenWindow refuses a recruitment session whose Application was decided or rejected, or whose
// interview deadline has passed. A practice session has no Application and is never refused.
func requireOpenWindow(ctx context.Context, q *Queries, applicationID *uuid.UUID) error {
	if applicationID == nil {
		return nil
	}
	open, err := q.ApplicationAcceptsInterview(ctx, *applicationID)
	if err != nil {
		return internal(err)
	}
	if !open {
		return domain.ErrApplicationNotEligible
	}
	return nil
}
