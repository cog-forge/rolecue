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
// Turns and the end of a session lock the session row.
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

// AppendTurn stores the answer to the next core question. A retry of an answer already stored returns the
// session unchanged. The rules are in domain.AcceptAnswer and run under the session lock. The turn stores
// the question text as it was asked.
func (r *Repository) AppendTurn(ctx context.Context, candidateID, id uuid.UUID, position int32, answer string) (domain.Session, error) {
	return r.appendAnswer(ctx, candidateID, id, func(q *Queries, turns []domain.Turn, questionCount int32) error {
		slot, unchanged, err := domain.AcceptAnswer(turns, questionCount, position, answer)
		if err != nil || unchanged {
			return err
		}
		question, err := q.GetInterviewQuestion(ctx, GetInterviewQuestionParams{InterviewID: id, Position: slot.QuestionNumber})
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrTurnConflict
		}
		if err != nil {
			return internal(err)
		}
		// The question text is the one the interviewer spoke when it was delivered live.
		return insertTurn(ctx, q, InsertInterviewTurnParams{
			InterviewID: id, Position: position, Kind: string(domain.TurnCore), Question: question.Content, Answer: &answer,
			AskedAt: question.AskedAt,
		})
	})
}

// AppendFollowUp stores the answer to a follow-up the live interviewer asked, together with the
// follow-up's text and the time it was asked. A follow-up is stored only with its answer: one that was
// asked but never answered (the socket dropped) leaves no trace. The rules are in domain.AcceptFollowUp:
// the slot must still be open within followUpLimit, so no core question ever gets more follow-ups than
// the limit, and a retry of a stored answer changes nothing.
func (r *Repository) AppendFollowUp(ctx context.Context, candidateID, id uuid.UUID, followUp domain.AskedQuestion, answer string, followUpLimit int32) (domain.Session, error) {
	return r.appendAnswer(ctx, candidateID, id, func(q *Queries, turns []domain.Turn, _ int32) error {
		_, unchanged, err := domain.AcceptFollowUp(turns, followUpLimit, followUp.Position, answer)
		if err != nil || unchanged {
			return err
		}
		return insertTurn(ctx, q, InsertInterviewTurnParams{
			InterviewID: id, Position: followUp.Position, Kind: string(domain.TurnFollowUp), Question: followUp.Text, Answer: &answer,
			AskedAt: pgtype.Timestamptz{Time: followUp.AskedAt, Valid: true},
		})
	})
}

// appendAnswer runs one answer write under the session lock: it refuses a completed session and a
// recruitment session whose window closed, lets store decide and write, and reads the session back.
func (r *Repository) appendAnswer(ctx context.Context, candidateID, id uuid.UUID, store func(q *Queries, turns []domain.Turn, questionCount int32) error) (domain.Session, error) {
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
		questionCount, turns, err := readTurns(ctx, q, id)
		if err != nil {
			return err
		}
		if err := store(q, turns, questionCount); err != nil {
			return err
		}
		session, err = readSession(ctx, q, candidateID, id)
		return err
	})
	if err != nil {
		return domain.Session{}, internal(err)
	}
	return session, nil
}

func insertTurn(ctx context.Context, q *Queries, params InsertInterviewTurnParams) error {
	if err := q.InsertInterviewTurn(ctx, params); err != nil {
		return internal(err)
	}
	return nil
}

// Deliver prepares the next core question under the session lock: it refuses a completed session and a
// recruitment session whose window closed (the same rules as AppendTurn), then freezes the text and the
// time of the next core question and the session's provider voice. A repeat for the same question
// returns what was frozen the first time. When every core question is answered it freezes nothing and
// returns no question. It never asks a follow-up: follow-ups are decided only after a new answer
// (FollowUpContext).
func (r *Repository) Deliver(ctx context.Context, candidateID, id uuid.UUID, defaultProviderVoiceID string) (domain.Delivery, error) {
	var delivery domain.Delivery
	err := r.atTurnBoundary(ctx, candidateID, id, defaultProviderVoiceID, func(q *Queries, turns []domain.Turn, questionCount int32, d *domain.Delivery) error {
		delivery = *d
		slot, ok := domain.NextCore(turns, questionCount)
		if !ok {
			return nil
		}
		asked, err := q.MarkInterviewQuestionAsked(ctx, MarkInterviewQuestionAskedParams{InterviewID: id, Position: slot.QuestionNumber})
		if err != nil {
			return internal(err)
		}
		delivery.Question = &domain.AskedQuestion{
			Position: slot.Position, QuestionNumber: slot.QuestionNumber, Kind: domain.TurnCore,
			Text: asked.SpokenText, AskedAt: asked.AskedAt.Time,
		}
		return nil
	})
	if err != nil {
		return domain.Delivery{}, err
	}
	return delivery, nil
}

// FollowUpContext reads, under the session lock, what a follow-up decider needs when a follow-up slot is
// open after the last stored answer (domain.FollowUpSlot): the core question as it was asked, its answer
// and the earlier follow-ups with theirs, and the Job Description's requirements. It returns nil when no
// slot is open. The Delivery carries the session's position and provider voice, without a question.
func (r *Repository) FollowUpContext(ctx context.Context, candidateID, id uuid.UUID, defaultProviderVoiceID string, followUpLimit int32) (domain.Delivery, *domain.FollowUpContext, error) {
	var delivery domain.Delivery
	var pending *domain.FollowUpContext
	err := r.atTurnBoundary(ctx, candidateID, id, defaultProviderVoiceID, func(q *Queries, turns []domain.Turn, _ int32, d *domain.Delivery) error {
		delivery = *d
		slot, ok := domain.FollowUpSlot(turns, followUpLimit)
		if !ok {
			return nil
		}
		core, err := q.GetInterviewQuestion(ctx, GetInterviewQuestionParams{InterviewID: id, Position: slot.QuestionNumber})
		if err != nil {
			return internal(err)
		}
		pending = &domain.FollowUpContext{
			Position: slot.Position, QuestionNumber: slot.QuestionNumber, Ordinal: slot.Ordinal, CoreQuestion: core.Content,
			Exchanges: []domain.Exchange{}, Requirements: []domain.Requirement{},
		}
		for _, turn := range turns {
			if turn.QuestionNumber == slot.QuestionNumber {
				pending.Exchanges = append(pending.Exchanges, domain.Exchange{Question: turn.Question, Answer: turn.Answer})
			}
		}
		rows, err := q.ListInterviewRequirements(ctx, id)
		if err != nil {
			return internal(err)
		}
		for _, row := range rows {
			pending.Requirements = append(pending.Requirements, domain.Requirement{Name: row.Name, Target: row.Target, Type: row.Type})
		}
		return nil
	})
	if err != nil {
		return domain.Delivery{}, nil, err
	}
	return delivery, pending, nil
}

// atTurnBoundary runs read under the session lock with the session's position and frozen provider voice:
// it refuses a completed session and a recruitment session whose window closed, the same rules as an
// answer.
func (r *Repository) atTurnBoundary(ctx context.Context, candidateID, id uuid.UUID, defaultProviderVoiceID string, read func(q *Queries, turns []domain.Turn, questionCount int32, d *domain.Delivery) error) error {
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
		questionCount, turns, err := readTurns(ctx, q, id)
		if err != nil {
			return err
		}
		voice, err := q.FreezeInterviewVoice(ctx, FreezeInterviewVoiceParams{ID: id, DefaultProviderVoiceID: defaultProviderVoiceID})
		if err != nil {
			return internal(err)
		}
		d := domain.Delivery{Type: domain.Type(header.Type), QuestionCount: questionCount, AnsweredCount: int32(len(turns))}
		if voice != nil {
			d.ProviderVoiceID = *voice
		}
		return read(q, turns, questionCount, &d)
	})
	if err != nil {
		return internal(err)
	}
	return nil
}

// evaluationStaleAfter is how long a pending evaluation may run before another Complete takes it over.
// It is longer than the service's evaluation timeout, so a live run is never taken over.
const evaluationStaleAfter = 3 * time.Minute

// End is the one end path of a session: a normal completion, the Candidate leaving early and the System
// Handler abandoning an idle session all go through it. Under the session lock it sets completed_at and
// the end reason once (domain.ReasonFor, from the answers stored) and, for a recruitment session, moves
// its Application from interview_eligible to interviewed in the same transaction, exactly as a normal
// completion does. Unanswered questions stay without a turn. It never charges, refunds or consumes a
// slot: those happen only at the start.
//
// It is idempotent: ending an ended session changes nothing and returns it, so a double click, two tabs or
// a reconnect racing a leave all see the first ending. A recruitment session whose window closed cannot be
// ended (APPLICATION_NOT_ELIGIBLE), the same rule as an answer.
//
// An abandonment (params.IdleBefore set) re-reads the last activity under the lock and does nothing,
// reporting ended false, when the session was active at or after IdleBefore: an answer stored while the
// System Handler was deciding keeps the session alive.
//
// The bool startEvaluation is true only for the one call that claimed the evaluation (the first ending, a
// failed evaluation, or a pending one whose run died), so the caller starts it once.
func (r *Repository) End(ctx context.Context, params domain.EndParams) (result domain.EndResult, err error) {
	err = pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		q := r.queries.WithTx(tx)
		header, err := lockSession(ctx, q, params.CandidateID, params.ID)
		if err != nil {
			return err
		}
		if !header.CompletedAt.Valid {
			if params.IdleBefore != nil {
				last, err := q.InterviewLastActivity(ctx, params.ID)
				if err != nil {
					return internal(err)
				}
				if !last.Time.Before(*params.IdleBefore) {
					result.Session, err = readSession(ctx, q, params.CandidateID, params.ID)
					return err
				}
			}
			if err := requireOpenWindow(ctx, q, header.ApplicationID); err != nil {
				return err
			}
			questionCount, turns, err := readTurns(ctx, q, params.ID)
			if err != nil {
				return err
			}
			reason := string(domain.ReasonFor(params.Cause, domain.AnsweredCore(turns), questionCount))
			if err := q.SetInterviewEnded(ctx, SetInterviewEndedParams{ID: params.ID, EndReason: &reason}); err != nil {
				return internal(err)
			}
			if header.ApplicationID != nil {
				if err := q.MarkApplicationInterviewed(ctx, *header.ApplicationID); err != nil {
					return internal(err)
				}
			}
			result.Ended = true
		}
		claimed, err := q.ClaimInterviewEvaluation(ctx, ClaimInterviewEvaluationParams{
			ID: params.ID, StaleSeconds: evaluationStaleAfter.Seconds(),
		})
		if err != nil {
			return internal(err)
		}
		result.StartEvaluation = claimed == 1
		result.Session, err = readSession(ctx, q, params.CandidateID, params.ID)
		return err
	})
	if err != nil {
		return domain.EndResult{}, internal(err)
	}
	return result, nil
}

// ListAbandoned returns up to limit unfinished sessions idle since before idleBefore, with their owners,
// for the System Handler. now is the time a recruitment window is judged at. The list is a hint: End
// checks again under the session lock.
func (r *Repository) ListAbandoned(ctx context.Context, now, idleBefore time.Time, limit int32) ([]domain.SessionRef, error) {
	rows, err := r.queries.ListAbandonedInterviews(ctx, ListAbandonedInterviewsParams{
		Now:        pgtype.Timestamptz{Time: now, Valid: true},
		IdleBefore: pgtype.Timestamptz{Time: idleBefore, Valid: true},
		BatchSize:  limit,
	})
	if err != nil {
		return nil, internal(err)
	}
	refs := make([]domain.SessionRef, 0, len(rows))
	for _, row := range rows {
		refs = append(refs, domain.SessionRef{ID: row.ID, CandidateID: row.CandidateID})
	}
	return refs, nil
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

func readTurns(ctx context.Context, q *Queries, id uuid.UUID) (int32, []domain.Turn, error) {
	questionCount, err := q.CountInterviewQuestions(ctx, id)
	if err != nil {
		return 0, nil, internal(err)
	}
	turns, err := listTurns(ctx, q, id)
	if err != nil {
		return 0, nil, err
	}
	return int32(questionCount), turns, nil
}

func listTurns(ctx context.Context, q *Queries, id uuid.UUID) ([]domain.Turn, error) {
	rows, err := q.ListInterviewTurns(ctx, id)
	if err != nil {
		return nil, internal(err)
	}
	turns := make([]domain.Turn, 0, len(rows))
	for _, row := range rows {
		turns = append(turns, domain.Turn{
			Position: row.Position, QuestionNumber: row.QuestionNumber, Kind: domain.TurnKind(row.Kind),
			Question: row.Question, Answer: row.Answer,
		})
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
	questionCount, turns, err := readTurns(ctx, q, id)
	if err != nil {
		return domain.Session{}, err
	}
	session := domain.Session{
		ID: row.ID, Type: domain.Type(row.Type), RoleProfileID: row.RoleProfileID,
		ApplicationID: row.ApplicationID, AvatarID: row.AvatarID, VoiceID: row.VoiceID,
		StartedAt: row.StartedAt.Time, CompletedAt: timePtr(row.CompletedAt),
		EndReason:        (*domain.EndReason)(row.EndReason),
		EvaluationStatus: domain.EvaluationStatus(row.EvaluationStatus),
		QuestionCount:    questionCount, AnsweredCount: domain.AnsweredCore(turns), AnsweredTurns: int32(len(turns)), Turns: &turns,
	}
	if row.CompletedAt.Valid {
		return session, nil
	}
	slot, ok := domain.NextCore(turns, questionCount)
	if !ok {
		return session, nil
	}
	question, err := q.GetInterviewQuestion(ctx, GetInterviewQuestionParams{InterviewID: id, Position: slot.QuestionNumber})
	if err != nil {
		return domain.Session{}, internal(err)
	}
	session.CurrentQuestion = &domain.CurrentQuestion{Position: slot.Position, QuestionNumber: slot.QuestionNumber, Content: question.Content}
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
