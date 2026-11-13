package repository

import (
	"context"
	"errors"

	evaluationdomain "github.com/cog-forge/rolecue/api/internal/features/evaluation/domain"
	evaluationrepository "github.com/cog-forge/rolecue/api/internal/features/evaluation/repository"
	"github.com/cog-forge/rolecue/api/internal/features/questionbank/domain"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository takes the pool, not just DBTX, because generation commits several statements together.
type Repository struct {
	pool          *pgxpool.Pool
	queries       *Queries
	questionLimit int
}

// Option configures a Repository.
type Option func(*Repository)

// WithQuestionLimit sets how many core questions one interview session asks, the limit the metric-coverage
// reads use. It must be the value the interview start uses (the default applies when it is not set or not
// positive).
func WithQuestionLimit(limit int) Option {
	return func(r *Repository) { r.questionLimit = evaluationdomain.ResolveQuestionLimit(limit) }
}

func NewRepository(pool *pgxpool.Pool, options ...Option) *Repository {
	r := &Repository{pool: pool, queries: New(pool), questionLimit: evaluationdomain.DefaultQuestionLimit}
	for _, option := range options {
		option(r)
	}
	return r
}

// Coverage reports whether the posting's bank can cover every metric active under the posting's weights
// within the session question limit, by the shared rule of evaluation/domain (KAN-377). The caller has
// already checked the posting is the Recruiter's own.
func (r *Repository) Coverage(ctx context.Context, postingID uuid.UUID) (evaluationdomain.CoverageReport, error) {
	report, err := evaluationrepository.PostingCoverage(ctx, r.pool, postingID, r.questionLimit)
	if err != nil {
		return evaluationdomain.CoverageReport{}, persistenceError(err)
	}
	return report, nil
}

// ActiveMetrics returns the metrics a generated bank must be able to cover: the posting's active metrics, or
// the whole catalog for a practice bank (postingID nil).
func (r *Repository) ActiveMetrics(ctx context.Context, postingID *uuid.UUID) ([]uuid.UUID, error) {
	ids, err := evaluationrepository.ActiveMetricIDs(ctx, r.pool, postingID)
	if err != nil {
		return nil, persistenceError(err)
	}
	return ids, nil
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
		option.Criteria = append(option.Criteria, domain.RubricOptionCriterion{Name: row.Name, Metric: row.MetricName, MaxPoints: row.MaxPoints, MetricID: row.MetricID})
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
	return r.WriteQuestion(ctx, recruiterID, postingID, &questionID, domain.QuestionPatch{Content: &content})
}

func equalValue[T comparable](a, b *T) bool {
	return (a == nil && b == nil) || (a != nil && b != nil && *a == *b)
}

func usableRubric(ctx context.Context, q *Queries, recruiterID uuid.UUID, id *uuid.UUID) error {
	if id == nil {
		return nil
	}
	ids, err := q.LockUsableRubrics(ctx, LockUsableRubricsParams{RubricIds: []uuid.UUID{*id}, RecruiterID: recruiterID})
	if err != nil {
		return persistenceError(err)
	}
	if len(ids) != 1 {
		return apperror.New(apperror.CodeRubricNotFound, "rubric not available")
	}
	return nil
}

// WriteQuestion takes the same posting lock as submit and approval, before validating or changing rows.
func (r *Repository) WriteQuestion(ctx context.Context, recruiterID, postingID uuid.UUID, questionID *uuid.UUID, patch domain.QuestionPatch) (domain.BankQuestion, error) {
	var saved domain.BankQuestion
	err := r.withEditablePosting(ctx, recruiterID, postingID, questionID != nil, func(q *Queries) error {
		if err := usableRubric(ctx, q, recruiterID, patch.RubricID); patch.SetRubric && err != nil {
			return err
		}
		id := questionID
		if id == nil {
			if patch.Content == nil {
				return apperror.New(apperror.CodeValidation, "content is required")
			}
			row, err := q.AddCoreQuestion(ctx, AddCoreQuestionParams{Content: *patch.Content, PostingID: postingID, RecruiterID: recruiterID})
			if errors.Is(err, pgx.ErrNoRows) {
				return r.postingRefusal(ctx, q, recruiterID, postingID, false)
			}
			if err != nil {
				return persistenceError(err)
			}
			id = &row.ID
		}
		var err error
		saved, err = r.writeLockedQuestion(ctx, q, recruiterID, postingID, *id, patch)
		if err != nil {
			return err
		}
		if questionID == nil {
			return persistenceError(q.WithdrawSubmission(ctx, postingID))
		}
		return nil
	})
	return saved, err
}

func (r *Repository) writeLockedQuestion(ctx context.Context, q *Queries, recruiterID, postingID, id uuid.UUID, patch domain.QuestionPatch) (domain.BankQuestion, error) {
	if _, err := q.LockQuestionForEdit(ctx, LockQuestionForEditParams{ID: id, PostingID: postingID}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.BankQuestion{}, apperror.New(apperror.CodeCoreQuestionNotFound, "question not found")
		}
		return domain.BankQuestion{}, persistenceError(err)
	}
	before, err := bankQuestion(ctx, q, recruiterID, id)
	if err != nil {
		return domain.BankQuestion{}, err
	}
	if patch.PreviousRevision != "" && patch.PreviousRevision != before.Revision {
		return domain.BankQuestion{}, apperror.New(apperror.CodeQuestionStale, "question changed; reload before reapplying your draft")
	}
	after := before
	if patch.Content != nil {
		after.Content = *patch.Content
	}
	if patch.SetRubric {
		after.RubricID = patch.RubricID
	}
	if patch.SetExpectedAnswer {
		after.ExpectedAnswer = patch.ExpectedAnswer
	}
	if !equalValue(before.RubricID, after.RubricID) && !patch.SetExpectedAnswer {
		return domain.BankQuestion{}, apperror.New(apperror.CodeExpectedAnswerReviewRequired, "review the expected answer or explicitly clear it when changing the rubric")
	}
	if before.Content == after.Content && equalValue(before.RubricID, after.RubricID) && equalValue(before.ExpectedAnswer, after.ExpectedAnswer) {
		return before, nil
	}
	inUse, err := q.QuestionInUse(ctx, id)
	if err != nil {
		return domain.BankQuestion{}, persistenceError(err)
	}
	if inUse {
		return domain.BankQuestion{}, apperror.New(apperror.CodeCoreQuestionInUse, "question has been used in an interview")
	}
	if err := q.WriteQuestionGrading(ctx, WriteQuestionGradingParams{ID: id, Content: after.Content, RubricID: after.RubricID, ExpectedAnswer: after.ExpectedAnswer}); err != nil {
		return domain.BankQuestion{}, persistenceError(err)
	}
	if err := q.WithdrawSubmission(ctx, postingID); err != nil {
		return domain.BankQuestion{}, persistenceError(err)
	}
	return bankQuestion(ctx, q, recruiterID, id)
}

// ApplyRubric is atomic. Only changed bindings clear the answer; a repeat assignment is a no-op.
func (r *Repository) ApplyRubric(ctx context.Context, recruiterID, postingID uuid.UUID, request domain.BulkRubricRequest) ([]domain.BankQuestion, error) {
	saved := make([]domain.BankQuestion, 0, len(request.QuestionIDs))
	err := r.withEditablePosting(ctx, recruiterID, postingID, true, func(q *Queries) error {
		if err := usableRubric(ctx, q, recruiterID, &request.RubricID); err != nil {
			return err
		}
		for _, id := range request.QuestionIDs {
			// Membership is checked by writeLockedQuestion before any field is returned.
			patch := domain.QuestionPatch{SetRubric: true, RubricID: &request.RubricID, PreviousRevision: request.PreviousRevisions[id]}
			if _, err := q.LockQuestionForEdit(ctx, LockQuestionForEditParams{ID: id, PostingID: postingID}); err != nil {
				if errors.Is(err, pgx.ErrNoRows) {
					return apperror.New(apperror.CodeCoreQuestionNotFound, "question not found")
				}
				return persistenceError(err)
			}
			before, err := bankQuestion(ctx, q, recruiterID, id)
			if err != nil {
				return err
			}
			patch.SetExpectedAnswer = !equalValue(before.RubricID, &request.RubricID)
			updated, err := r.writeLockedQuestion(ctx, q, recruiterID, postingID, id, patch)
			if err != nil {
				return err
			}
			saved = append(saved, updated)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return saved, nil
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
	return insertGraded(ctx, q, jdID, recruiterID, questions)
}

// insertGraded writes the questions of an empty bank. The caller has checked that it is empty.
func insertGraded(ctx context.Context, q *Queries, jdID, recruiterID uuid.UUID, questions []domain.GeneratedQuestion) ([]domain.BankQuestion, error) {
	itemIDs := make([]string, 0, len(questions))
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
		itemID := ""
		if question.TopicID != nil {
			itemID = question.TopicID.String()
		}
		itemIDs = append(itemIDs, itemID)
	}
	if len(chosen) > 0 {
		if _, err := q.LockUsableRubrics(ctx, LockUsableRubricsParams{RubricIds: chosen, RecruiterID: recruiterID}); err != nil {
			return nil, persistenceError(err)
		}
	}
	if _, err := q.InsertGradedCoreQuestions(ctx, InsertGradedCoreQuestionsParams{
		JdID: jdID, Contents: contents, RubricIds: rubricIDs, RecruiterID: recruiterID, ExpectedAnswers: answers, ItemIds: itemIDs}); err != nil {
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

// ReplacePracticeBank swaps the practice bank of a profile's job description for the given questions inside the
// caller's transaction, which also publishes the Roadmap topics the questions are mapped to (KAN-344). The old
// bank is deleted only after the new one is valid and in the same transaction, so a failure anywhere leaves
// the old bank intact. The caller holds the practice start lock of the profile, so no session can start
// between the check below and the commit; a profile that already has an interview keeps its bank
// (ErrBankInUse).
func (r *Repository) ReplacePracticeBank(ctx context.Context, tx pgx.Tx, jdID uuid.UUID, questions []domain.GeneratedQuestion) error {
	q := r.queries.WithTx(tx)
	used, err := q.JDHasPracticeInterviews(ctx, jdID)
	if err != nil {
		return persistenceError(err)
	}
	if used {
		return domain.ErrBankInUse
	}
	if err = q.DeleteJDCoreQuestions(ctx, jdID); err != nil {
		return persistenceError(err)
	}
	// Practice rubrics are the active system templates only: uuid.Nil owns no rubric.
	_, err = insertGraded(ctx, q, jdID, uuid.Nil, questions)
	return err
}

// AppendTopicQuestions adds questions to the practice bank of one Roadmap topic inside the caller's transaction,
// which holds the profile's practice start lock (KAN-347 top-up). It changes nothing that exists: no stored
// question, rubric binding or topic mapping is touched. When the topic already has total questions or more (a
// concurrent top-up got there first) nothing is written and false is returned. A topic that was replaced by a
// regeneration since the draft is a conflict, not a foreign key error.
func (r *Repository) AppendTopicQuestions(ctx context.Context, tx pgx.Tx, jdID, topicID uuid.UUID, total int, questions []domain.GeneratedQuestion) (bool, error) {
	q := r.queries.WithTx(tx)
	have, err := q.CountTopicCoreQuestions(ctx, CountTopicCoreQuestionsParams{JdID: jdID, RoadmapItemID: &topicID})
	if err != nil {
		return false, persistenceError(err)
	}
	if int(have) >= total {
		return false, nil
	}
	if _, err = insertGraded(ctx, q, jdID, uuid.Nil, questions); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && (pgErr.Code == "23503" || pgErr.Code == "23514") {
			return false, apperror.New(apperror.CodeRoadmapGenerating, "The learning roadmap changed while the questions were being prepared.")
		}
		return false, err
	}
	return true, nil
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
