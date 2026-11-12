package repository

import (
	"context"
	"errors"

	"github.com/cog-forge/rolecue/api/internal/features/questionbank/domain"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository takes the pool, not just DBTX, because generation commits several statements together.
type Repository struct {
	pool    *pgxpool.Pool
	queries *Queries
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool, queries: New(pool)}
}

func (r *Repository) Posting(ctx context.Context, recruiterID, postingID uuid.UUID) (domain.Posting, error) {
	row, err := r.queries.GetRecruiterPosting(ctx, GetRecruiterPostingParams{ID: postingID, RecruiterID: recruiterID})
	if err != nil {
		return domain.Posting{}, persistenceError(err)
	}
	return domain.Posting{JDID: row.JdID, Status: row.Status, ReviewSubmitted: row.ReviewSubmitted}, nil
}

func (r *Repository) CandidateOwnsJD(ctx context.Context, candidateID, jdID uuid.UUID) error {
	owns, err := r.queries.CandidateOwnsJD(ctx, CandidateOwnsJDParams{JdID: jdID, CandidateID: candidateID})
	if err != nil {
		return persistenceError(err)
	}
	if !owns {
		return apperror.New(apperror.CodeJDNotFound, "job description not found")
	}
	return nil
}

func (r *Repository) List(ctx context.Context, jdID uuid.UUID) ([]domain.CoreQuestion, error) {
	rows, err := r.queries.ListCoreQuestions(ctx, jdID)
	if err != nil {
		return nil, persistenceError(err)
	}
	out := make([]domain.CoreQuestion, 0, len(rows))
	for _, row := range rows {
		out = append(out, domain.CoreQuestion{ID: row.ID, Content: row.Content})
	}
	return out, nil
}

// ListBank is the Recruiter's view of a bank: each question with its grading setup. The interview runtime
// reads List instead, which never selects grading data.
func (r *Repository) ListBank(ctx context.Context, jdID, recruiterID uuid.UUID) ([]domain.BankQuestion, error) {
	return listBank(ctx, r.queries, jdID, recruiterID)
}

func listBank(ctx context.Context, q *Queries, jdID, recruiterID uuid.UUID) ([]domain.BankQuestion, error) {
	rows, err := q.ListBankQuestions(ctx, ListBankQuestionsParams{JdID: jdID, RecruiterID: recruiterID})
	if err != nil {
		return nil, persistenceError(err)
	}
	out := make([]domain.BankQuestion, 0, len(rows))
	for _, row := range rows {
		out = append(out, domain.NewBankQuestion(row.ID, row.Content, row.RubricID,
			domain.RubricState{Active: row.RubricActive, Permitted: row.RubricPermitted}, row.ExpectedAnswer))
	}
	return out, nil
}

func bankQuestion(ctx context.Context, q *Queries, recruiterID, id uuid.UUID) (domain.BankQuestion, error) {
	row, err := q.GetBankQuestion(ctx, GetBankQuestionParams{ID: id, RecruiterID: recruiterID})
	if err != nil {
		return domain.BankQuestion{}, persistenceError(err)
	}
	return domain.NewBankQuestion(row.ID, row.Content, row.RubricID,
		domain.RubricState{Active: row.RubricActive, Permitted: row.RubricPermitted}, row.ExpectedAnswer), nil
}

// RubricCatalog returns the rubrics a generation for this Recruiter may choose from, capped at
// domain.MaxPromptRubrics, with their criteria. Whether a rubric is offered is decided in SQL: an active
// system template or the Recruiter's own active rubric, never someone else's private rubric.
func (r *Repository) RubricCatalog(ctx context.Context, recruiterID uuid.UUID) ([]domain.RubricOption, error) {
	rows, err := r.queries.ListGenerationRubrics(ctx, ListGenerationRubricsParams{RecruiterID: recruiterID, PageLimit: domain.MaxPromptRubrics})
	if err != nil {
		return nil, persistenceError(err)
	}
	options := make([]domain.RubricOption, 0, len(rows))
	ids := make([]uuid.UUID, 0, len(rows))
	index := make(map[uuid.UUID]int, len(rows))
	for _, row := range rows {
		index[row.ID] = len(options)
		ids = append(ids, row.ID)
		options = append(options, domain.RubricOption{ID: row.ID, Name: row.Name, Description: row.Description, QuestionType: row.QuestionType})
	}
	if len(ids) == 0 {
		return options, nil
	}
	criteria, err := r.queries.ListGenerationRubricCriteria(ctx, ids)
	if err != nil {
		return nil, persistenceError(err)
	}
	for _, row := range criteria {
		option := &options[index[row.RubricID]]
		option.Criteria = append(option.Criteria, domain.RubricOptionCriterion{Name: row.Name, Metric: row.MetricName, MaxPoints: row.MaxPoints})
	}
	return options, nil
}

func (r *Repository) GenerationInput(ctx context.Context, jdID uuid.UUID) (domain.GenerationInput, error) {
	rows, err := r.queries.ListRequirementsForJD(ctx, jdID)
	if err != nil {
		return domain.GenerationInput{}, persistenceError(err)
	}
	jd, err := r.queries.GetJDGenerationContext(ctx, jdID)
	if err != nil {
		return domain.GenerationInput{}, persistenceError(err)
	}
	in := domain.GenerationInput{
		Seniority:    domain.SeniorityLevel{Name: jd.LevelName, Description: jd.LevelDescription},
		Requirements: make([]domain.ConfirmedRequirement, 0, len(rows)),
	}
	for _, row := range rows {
		in.Requirements = append(in.Requirements, domain.ConfirmedRequirement{
			Name: row.Name, Target: row.Target, Type: row.Type, EvidenceText: row.EvidenceText})
	}
	if jd.RefinementNote != nil {
		in.RefinementNote = *jd.RefinementNote
	}
	return in, nil
}

// Add, Update and Delete lock the posting row before they touch the bank, the same lock SaveForPosting
// takes. Admin approval updates that row too, so an edit and an approval run one after the other: an
// edit that wins makes the approval recheck review_submitted and the bank, and an approval that wins
// makes the edit see a posting that is no longer editable (KAN-211). When the guarded statement
// affects no row, a cheap follow-up query tells "not found" from "locked".
func (r *Repository) Add(ctx context.Context, recruiterID, postingID uuid.UUID, content string) (domain.BankQuestion, error) {
	var added domain.BankQuestion
	err := r.withEditablePosting(ctx, recruiterID, postingID, false, func(q *Queries) error {
		row, err := q.AddCoreQuestion(ctx, AddCoreQuestionParams{Content: content, PostingID: postingID, RecruiterID: recruiterID})
		if errors.Is(err, pgx.ErrNoRows) {
			return r.postingRefusal(ctx, q, recruiterID, postingID, false)
		}
		if err != nil {
			return persistenceError(err)
		}
		if err := q.WithdrawSubmission(ctx, postingID); err != nil {
			return persistenceError(err)
		}
		added, err = bankQuestion(ctx, q, recruiterID, row.ID)
		return err
	})
	return added, err
}

func (r *Repository) Update(ctx context.Context, recruiterID, postingID, questionID uuid.UUID, content string) (domain.BankQuestion, error) {
	var updated domain.BankQuestion
	err := r.withEditablePosting(ctx, recruiterID, postingID, true, func(q *Queries) error {
		// A text change withdraws a submission; saving the same text again changes nothing.
		before, err := q.LockQuestionContent(ctx, LockQuestionContentParams{ID: questionID, PostingID: postingID})
		if errors.Is(err, pgx.ErrNoRows) {
			return r.postingRefusal(ctx, q, recruiterID, postingID, true)
		}
		if err != nil {
			return persistenceError(err)
		}
		row, err := q.UpdateCoreQuestion(ctx, UpdateCoreQuestionParams{
			Content: content, ID: questionID, PostingID: postingID, RecruiterID: recruiterID})
		if errors.Is(err, pgx.ErrNoRows) {
			return r.postingRefusal(ctx, q, recruiterID, postingID, true)
		}
		if err != nil {
			return persistenceError(err)
		}
		if before != content {
			if err := q.WithdrawSubmission(ctx, postingID); err != nil {
				return persistenceError(err)
			}
		}
		updated, err = bankQuestion(ctx, q, recruiterID, row.ID)
		return err
	})
	return updated, err
}

// Delete withdraws the posting from the Admin queue, like any other change to the bank.
func (r *Repository) Delete(ctx context.Context, recruiterID, postingID, questionID uuid.UUID) error {
	return r.withEditablePosting(ctx, recruiterID, postingID, true, func(q *Queries) error {
		n, err := q.DeleteCoreQuestion(ctx, DeleteCoreQuestionParams{ID: questionID, PostingID: postingID, RecruiterID: recruiterID})
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && (pgErr.Code == "23503" || pgErr.Code == "23001") {
			return apperror.Wrap(apperror.CodeCoreQuestionInUse, "this question was used in an interview and cannot be deleted", err)
		}
		if err != nil {
			return persistenceError(err)
		}
		if n == 0 {
			return r.postingRefusal(ctx, q, recruiterID, postingID, true)
		}
		return persistenceError(q.WithdrawSubmission(ctx, postingID))
	})
}

// withEditablePosting runs fn in a transaction that first locks the Recruiter's posting while it is
// still editable. A posting that is missing, foreign or no longer editable is refused before fn runs.
func (r *Repository) withEditablePosting(ctx context.Context, recruiterID, postingID uuid.UUID, questionMissing bool, fn func(*Queries) error) error {
	return pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		q := r.queries.WithTx(tx)
		if _, err := q.LockEditablePosting(ctx, LockEditablePostingParams{ID: postingID, RecruiterID: recruiterID}); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return r.postingRefusal(ctx, q, recruiterID, postingID, questionMissing)
			}
			return persistenceError(err)
		}
		return fn(q)
	})
}

// SaveForPosting stores a freshly generated bank. Locking the posting row serializes concurrent
// generate calls, so the second one sees the first one's questions and fails with "exists".
func (r *Repository) SaveForPosting(ctx context.Context, recruiterID, postingID uuid.UUID, questions []domain.GeneratedQuestion) ([]domain.BankQuestion, error) {
	var saved []domain.BankQuestion
	err := pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		q := r.queries.WithTx(tx)
		jdID, err := q.LockEditablePosting(ctx, LockEditablePostingParams{ID: postingID, RecruiterID: recruiterID})
		if errors.Is(err, pgx.ErrNoRows) {
			return r.postingRefusal(ctx, q, recruiterID, postingID, false)
		}
		if err != nil {
			return persistenceError(err)
		}
		saved, err = insertGradedIfEmpty(ctx, q, jdID, recruiterID, questions)
		return err
	})
	if err != nil {
		return nil, err
	}
	return saved, nil
}

// insertGradedIfEmpty stores the questions with their rubric and expected answer. A rubric that is no longer
// usable by the Recruiter (archived or removed since generation started) is stored as no rubric, so the
// question shows as needing grading configuration instead of pointing at a rubric nobody may assign.
func insertGradedIfEmpty(ctx context.Context, q *Queries, jdID, recruiterID uuid.UUID, questions []domain.GeneratedQuestion) ([]domain.BankQuestion, error) {
	count, err := q.CountCoreQuestions(ctx, jdID)
	if err != nil {
		return nil, persistenceError(err)
	}
	if count > 0 {
		return nil, apperror.New(apperror.CodeQuestionBankExists, "this job description already has a question bank")
	}
	contents := make([]string, 0, len(questions))
	rubricIDs := make([]string, 0, len(questions))
	answers := make([]string, 0, len(questions))
	chosen := make([]uuid.UUID, 0, len(questions))
	for _, question := range questions {
		contents = append(contents, question.Content)
		rubricID, answer := "", ""
		if question.RubricID != nil {
			rubricID = question.RubricID.String()
			chosen = append(chosen, *question.RubricID)
		}
		if question.ExpectedAnswer != nil {
			answer = *question.ExpectedAnswer
		}
		rubricIDs = append(rubricIDs, rubricID)
		answers = append(answers, answer)
	}
	if len(chosen) > 0 {
		if _, err := q.LockUsableRubrics(ctx, LockUsableRubricsParams{RubricIds: chosen, RecruiterID: recruiterID}); err != nil {
			return nil, persistenceError(err)
		}
	}
	if _, err := q.InsertGradedCoreQuestions(ctx, InsertGradedCoreQuestionsParams{
		JdID: jdID, Contents: contents, RubricIds: rubricIDs, RecruiterID: recruiterID, ExpectedAnswers: answers}); err != nil {
		return nil, persistenceError(err)
	}
	return listBank(ctx, q, jdID, recruiterID)
}

// SaveForPractice does the same for a Candidate's own practice JD. The questions keep their rubric (active
// system templates only: uuid.Nil owns no rubric) and expected answer, which are never returned to the
// Candidate.
func (r *Repository) SaveForPractice(ctx context.Context, candidateID, jdID uuid.UUID, questions []domain.GeneratedQuestion) ([]domain.CoreQuestion, error) {
	var saved []domain.CoreQuestion
	err := pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		q := r.queries.WithTx(tx)
		locked, err := q.LockCandidateRoleProfile(ctx, LockCandidateRoleProfileParams{JdID: jdID, CandidateID: candidateID})
		if errors.Is(err, pgx.ErrNoRows) {
			return apperror.New(apperror.CodeJDNotFound, "job description not found")
		}
		if err != nil {
			return persistenceError(err)
		}
		graded, err := insertGradedIfEmpty(ctx, q, locked, uuid.Nil, questions)
		if err != nil {
			return err
		}
		saved = make([]domain.CoreQuestion, 0, len(graded))
		for _, question := range graded {
			saved = append(saved, domain.CoreQuestion{ID: question.ID, Content: question.Content})
		}
		return err
	})
	if err != nil {
		return nil, err
	}
	return saved, nil
}

// postingRefusal explains why a guarded statement affected no row. questionMissing is true for
// per-question statements: an editable posting then means the question itself does not exist.
// Otherwise it was Add, which found no generated bank to extend.
func (r *Repository) postingRefusal(ctx context.Context, q *Queries, recruiterID, postingID uuid.UUID, questionMissing bool) error {
	row, err := q.GetRecruiterPosting(ctx, GetRecruiterPostingParams{ID: postingID, RecruiterID: recruiterID})
	if err != nil {
		return persistenceError(err)
	}
	if !(domain.Posting{Status: row.Status}).Editable() {
		return errBankLocked
	}
	if questionMissing {
		return apperror.New(apperror.CodeCoreQuestionNotFound, "question not found")
	}
	return apperror.New(apperror.CodeQuestionBankNotReady, "generate the question bank before adding questions")
}

var errBankLocked = apperror.New(apperror.CodeQuestionBankLocked, "the question bank can only be changed while the job posting is pending or rejected")

func persistenceError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return apperror.Wrap(apperror.CodeJobPostingNotFound, "job posting not found", err)
	}
	return apperror.Wrap(apperror.CodeInternal, "question bank persistence failed", err)
}
