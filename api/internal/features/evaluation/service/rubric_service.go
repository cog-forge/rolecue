package service

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/cog-forge/rolecue/api/internal/features/evaluation/domain"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

const copyNamePrefix = "Copy of "

var questionTypeSlug = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// RubricRepository is the persistence contract of rubric management. Reads take the caller's access and apply
// the visibility rule in SQL, so a rubric the caller may not see is reported as not found. UpdateRubric,
// DeleteRubric and CopyRubric run in one transaction under the rubric's row lock; the repository decides
// stale tokens (RUBRIC_STALE), locked content (RUBRIC_LOCKED) and referenced deletes (RUBRIC_IN_USE) there.
type RubricRepository interface {
	ListMetrics(ctx context.Context) ([]domain.Metric, error)
	ListRubrics(ctx context.Context, access domain.RubricAccess, query domain.RubricListQuery) ([]domain.Rubric, int64, error)
	GetRubric(ctx context.Context, access domain.RubricAccess, id uuid.UUID) (domain.Rubric, error)
	CreateRubric(ctx context.Context, write domain.RubricWrite) (domain.Rubric, error)
	CopyRubric(ctx context.Context, access domain.RubricAccess, sourceID uuid.UUID, copy domain.RubricCopy) (domain.Rubric, error)
	UpdateRubric(ctx context.Context, id uuid.UUID, update domain.RubricUpdate) (domain.Rubric, error)
	DeleteRubric(ctx context.Context, id uuid.UUID) error
}

// Rubrics manages scoring rubrics. It is separate from Service (posting weights) and Scoring (interviews).
//
// Who may do what: an Admin manages system rubrics and cannot see a Recruiter's private ones. A Recruiter reads
// the active system rubrics and their own, copies either, and changes only their own. A system rubric that a
// Recruiter may read but not change answers 403; a rubric they may not see at all answers 404, the same as a
// missing id.
type Rubrics struct {
	repository RubricRepository
}

// NewRubrics builds the rubric management service.
func NewRubrics(repository RubricRepository) (*Rubrics, error) {
	if repository == nil {
		return nil, fmt.Errorf("rubric repository is required")
	}
	return &Rubrics{repository: repository}, nil
}

func rubricAccess(userID uuid.UUID, role string) (domain.RubricAccess, error) {
	if role != domain.RubricRoleAdmin && role != domain.RubricRoleRecruiter {
		return domain.RubricAccess{}, apperror.New(apperror.CodeForbidden, "only administrators and recruiters can use rubrics")
	}
	return domain.RubricAccess{UserID: userID, Role: role}, nil
}

// canManage reports whether the caller may change, archive or delete the rubric.
func canManage(access domain.RubricAccess, rubric domain.Rubric) bool {
	switch access.Role {
	case domain.RubricRoleAdmin:
		return rubric.Scope == domain.RubricScopeSystem
	case domain.RubricRoleRecruiter:
		return rubric.Scope == domain.RubricScopeRecruiter && rubric.OwnerID != nil && *rubric.OwnerID == access.UserID
	}
	return false
}

// present adds what depends on the caller: ownership, the right to manage and, for managers only, how many
// questions use the rubric. Usage of a system rubric across all Recruiters is not shown to a Recruiter.
func present(access domain.RubricAccess, rubric domain.Rubric) domain.Rubric {
	rubric.IsOwn = rubric.OwnerID != nil && *rubric.OwnerID == access.UserID
	rubric.CanManage = canManage(access, rubric)
	rubric.Locked, rubric.ReferenceCount = false, nil
	if rubric.CanManage {
		references := rubric.References
		rubric.ReferenceCount = &references
		rubric.Locked = domain.RubricLocked(references)
	}
	return rubric
}

// scopeFor is the scope and owner a new rubric gets: the caller's role decides, never the request.
func scopeFor(access domain.RubricAccess) (string, *uuid.UUID) {
	if access.Role == domain.RubricRoleAdmin {
		return domain.RubricScopeSystem, nil
	}
	owner := access.UserID
	return domain.RubricScopeRecruiter, &owner
}

// List returns the rubrics the caller may see, without their criteria, and the total for paging.
func (s *Rubrics) List(ctx context.Context, userID uuid.UUID, role string, query domain.RubricListQuery) ([]domain.Rubric, int64, error) {
	access, err := rubricAccess(userID, role)
	if err != nil {
		return nil, 0, err
	}
	query, err = normalizeListQuery(query)
	if err != nil {
		return nil, 0, err
	}
	rubrics, total, err := s.repository.ListRubrics(ctx, access, query)
	if err != nil {
		return nil, 0, err
	}
	for i := range rubrics {
		rubrics[i] = present(access, rubrics[i])
	}
	return rubrics, total, nil
}

func normalizeListQuery(query domain.RubricListQuery) (domain.RubricListQuery, error) {
	switch query.Scope {
	case "", domain.RubricListSystem, domain.RubricListOwn:
	default:
		return query, validationError("scope must be system or own")
	}
	switch query.Status {
	case "":
		query.Status = domain.RubricStatusActive
	case domain.RubricStatusActive, domain.RubricStatusArchived, domain.RubricStatusAll:
	default:
		return query, validationError("status must be active, archived or all")
	}
	if query.QuestionType != "" && !validQuestionType(query.QuestionType) {
		return query, validationError("question_type is not valid")
	}
	if query.Limit == 0 {
		query.Limit = domain.DefaultRubricPageSize
	}
	if query.Limit < 1 || query.Limit > domain.MaxRubricPageSize || query.Offset < 0 {
		return query, validationError("invalid pagination parameters")
	}
	return query, nil
}

// Get returns one rubric with its criteria.
func (s *Rubrics) Get(ctx context.Context, userID uuid.UUID, role string, id uuid.UUID) (domain.Rubric, error) {
	access, err := rubricAccess(userID, role)
	if err != nil {
		return domain.Rubric{}, err
	}
	rubric, err := s.repository.GetRubric(ctx, access, id)
	if err != nil {
		return domain.Rubric{}, err
	}
	return present(access, rubric), nil
}

// Create makes a rubric from scratch: private for a Recruiter, system for an Admin.
func (s *Rubrics) Create(ctx context.Context, userID uuid.UUID, role string, in domain.RubricCreate) (domain.Rubric, error) {
	access, err := rubricAccess(userID, role)
	if err != nil {
		return domain.Rubric{}, err
	}
	metrics, err := s.metricSet(ctx)
	if err != nil {
		return domain.Rubric{}, err
	}
	write, err := buildRubricWrite(in, metrics)
	if err != nil {
		return domain.Rubric{}, err
	}
	write.Scope, write.OwnerID = scopeFor(access)
	rubric, err := s.repository.CreateRubric(ctx, write)
	if err != nil {
		return domain.Rubric{}, err
	}
	return present(access, rubric), nil
}

// Copy creates an independent rubric (and its own copy of the criteria) from a rubric the caller may read. The
// copy belongs to the caller; changing it never changes the source. name is optional.
func (s *Rubrics) Copy(ctx context.Context, userID uuid.UUID, role string, id uuid.UUID, name *string) (domain.Rubric, error) {
	access, err := rubricAccess(userID, role)
	if err != nil {
		return domain.Rubric{}, err
	}
	source, err := s.repository.GetRubric(ctx, access, id)
	if err != nil {
		return domain.Rubric{}, err
	}
	copyName := defaultCopyName(source.Name)
	if name != nil {
		if copyName, err = cleanText("name", *name, domain.MaxRubricNameRunes, true, false); err != nil {
			return domain.Rubric{}, err
		}
	}
	scope, owner := scopeFor(access)
	rubric, err := s.repository.CopyRubric(ctx, access, id, domain.RubricCopy{Name: copyName, Scope: scope, OwnerID: owner})
	if err != nil {
		return domain.Rubric{}, err
	}
	return present(access, rubric), nil
}

// Update changes some fields of a rubric the caller manages. The request must carry the updated_at it read;
// a rubric saved by someone else since then is RUBRIC_STALE. Content changes are refused while the rubric is
// locked; archiving or restoring is always allowed.
func (s *Rubrics) Update(ctx context.Context, userID uuid.UUID, role string, id uuid.UUID, patch domain.RubricPatch) (domain.Rubric, error) {
	access, err := rubricAccess(userID, role)
	if err != nil {
		return domain.Rubric{}, err
	}
	current, err := s.repository.GetRubric(ctx, access, id)
	if err != nil {
		return domain.Rubric{}, err
	}
	if !canManage(access, current) {
		return domain.Rubric{}, apperror.New(apperror.CodeForbidden, "system rubrics can only be changed by an administrator, and private rubrics only by their owner")
	}
	var metrics map[uuid.UUID]bool
	if patch.Criteria != nil {
		if metrics, err = s.metricSet(ctx); err != nil {
			return domain.Rubric{}, err
		}
	}
	update, err := buildRubricUpdate(patch, metrics)
	if err != nil {
		return domain.Rubric{}, err
	}
	rubric, err := s.repository.UpdateRubric(ctx, id, update)
	if err != nil {
		return domain.Rubric{}, err
	}
	return present(access, rubric), nil
}

// Delete removes a rubric the caller manages. A rubric that questions still use cannot be deleted
// (RUBRIC_IN_USE): archive it instead.
func (s *Rubrics) Delete(ctx context.Context, userID uuid.UUID, role string, id uuid.UUID) error {
	access, err := rubricAccess(userID, role)
	if err != nil {
		return err
	}
	current, err := s.repository.GetRubric(ctx, access, id)
	if err != nil {
		return err
	}
	if !canManage(access, current) {
		return apperror.New(apperror.CodeForbidden, "system rubrics can only be deleted by an administrator, and private rubrics only by their owner")
	}
	return s.repository.DeleteRubric(ctx, id)
}

func (s *Rubrics) metricSet(ctx context.Context) (map[uuid.UUID]bool, error) {
	metrics, err := s.repository.ListMetrics(ctx)
	if err != nil {
		return nil, err
	}
	set := make(map[uuid.UUID]bool, len(metrics))
	for _, metric := range metrics {
		set[metric.ID] = true
	}
	return set, nil
}

func validationError(format string, args ...any) error {
	return apperror.New(apperror.CodeValidation, fmt.Sprintf(format, args...))
}

func validQuestionType(value string) bool {
	return utf8.RuneCountInString(value) <= domain.MaxRubricQuestionTypeRunes && questionTypeSlug.MatchString(value)
}

func defaultCopyName(source string) string {
	room := domain.MaxRubricNameRunes - utf8.RuneCountInString(copyNamePrefix)
	runes := []rune(strings.TrimSpace(source))
	if len(runes) > room {
		runes = runes[:room]
	}
	return copyNamePrefix + strings.TrimSpace(string(runes))
}

// cleanText checks one text field and returns it trimmed. It must be valid UTF-8 without control characters
// (line breaks and tabs are allowed only when multiline) and within max runes; required text may not be blank.
func cleanText(field, value string, max int, required, multiline bool) (string, error) {
	if !utf8.ValidString(value) {
		return "", validationError("%s must be valid UTF-8 text", field)
	}
	for _, r := range value {
		lineBreakOrTab := r == '\n' || r == '\r' || r == '\t'
		if unicode.IsControl(r) && (!multiline || !lineBreakOrTab) {
			return "", validationError("%s must not contain control characters", field)
		}
	}
	value = strings.TrimSpace(value)
	if required && value == "" {
		return "", validationError("%s is required", field)
	}
	if utf8.RuneCountInString(value) > max {
		return "", validationError("%s must be at most %d characters", field, max)
	}
	return value, nil
}

func requiredText(field string, value *string, max int, multiline bool) (string, error) {
	if value == nil {
		return "", validationError("%s is required", field)
	}
	return cleanText(field, *value, max, true, multiline)
}

func buildRubricWrite(in domain.RubricCreate, metrics map[uuid.UUID]bool) (domain.RubricWrite, error) {
	name, err := requiredText("name", in.Name, domain.MaxRubricNameRunes, false)
	if err != nil {
		return domain.RubricWrite{}, err
	}
	description := ""
	if in.Description != nil {
		if description, err = cleanText("description", *in.Description, domain.MaxRubricDescriptionRunes, false, true); err != nil {
			return domain.RubricWrite{}, err
		}
	}
	questionType, err := requiredQuestionType(in.QuestionType)
	if err != nil {
		return domain.RubricWrite{}, err
	}
	criteria, err := buildCriteria(in.Criteria, metrics, false)
	if err != nil {
		return domain.RubricWrite{}, err
	}
	return domain.RubricWrite{Name: name, Description: description, QuestionType: questionType, Criteria: criteria}, nil
}

func requiredQuestionType(value *string) (string, error) {
	if value == nil {
		return "", validationError("question_type is required")
	}
	if !validQuestionType(strings.TrimSpace(*value)) {
		return "", validationError("question_type must be a lower-case slug such as concept_explanation (at most %d characters)", domain.MaxRubricQuestionTypeRunes)
	}
	return strings.TrimSpace(*value), nil
}

func buildRubricUpdate(patch domain.RubricPatch, metrics map[uuid.UUID]bool) (domain.RubricUpdate, error) {
	if patch.UpdatedAt == nil {
		return domain.RubricUpdate{}, validationError("updated_at is required: send the value you read")
	}
	update := domain.RubricUpdate{ExpectedUpdatedAt: *patch.UpdatedAt, IsActive: patch.IsActive}
	if patch.Name != nil {
		name, err := requiredText("name", patch.Name, domain.MaxRubricNameRunes, false)
		if err != nil {
			return domain.RubricUpdate{}, err
		}
		update.Name = &name
	}
	if patch.Description != nil {
		description, err := cleanText("description", *patch.Description, domain.MaxRubricDescriptionRunes, false, true)
		if err != nil {
			return domain.RubricUpdate{}, err
		}
		update.Description = &description
	}
	if patch.QuestionType != nil {
		questionType, err := requiredQuestionType(patch.QuestionType)
		if err != nil {
			return domain.RubricUpdate{}, err
		}
		update.QuestionType = &questionType
	}
	if patch.Criteria != nil {
		criteria, err := buildCriteria(*patch.Criteria, metrics, true)
		if err != nil {
			return domain.RubricUpdate{}, err
		}
		update.Criteria = &criteria
	}
	if !update.ContentChange() && update.IsActive == nil {
		return domain.RubricUpdate{}, validationError("nothing to change")
	}
	return update, nil
}

// buildCriteria validates the whole criteria set. allowIDs is true on an update, where an ID keeps an existing
// criterion; each ID may appear once.
func buildCriteria(inputs []domain.CriterionInput, metrics map[uuid.UUID]bool, allowIDs bool) ([]domain.CriterionWrite, error) {
	if len(inputs) == 0 || len(inputs) > domain.MaxRubricCriteria {
		return nil, validationError("a rubric needs between 1 and %d criteria", domain.MaxRubricCriteria)
	}
	out := make([]domain.CriterionWrite, 0, len(inputs))
	seen := make(map[uuid.UUID]bool, len(inputs))
	for i, in := range inputs {
		label := fmt.Sprintf("criteria[%d]", i)
		write := domain.CriterionWrite{}
		if in.ID != nil {
			switch {
			case !allowIDs:
				return nil, validationError("%s.id cannot be set when creating a rubric", label)
			case seen[*in.ID]:
				return nil, validationError("%s.id appears twice", label)
			}
			seen[*in.ID] = true
			id := *in.ID
			write.ID = &id
		}
		if in.MetricID == nil {
			return nil, validationError("%s.metric_id is required", label)
		}
		if !metrics[*in.MetricID] {
			return nil, apperror.New(apperror.CodeEvaluationMetricUnknown, fmt.Sprintf("%s.metric_id is not a metric of the catalog", label))
		}
		write.MetricID = *in.MetricID
		if in.MaxPoints == nil || *in.MaxPoints < 1 || *in.MaxPoints > domain.MaxCriterionPoints {
			return nil, validationError("%s.max_points must be a whole number from 1 to %d", label, domain.MaxCriterionPoints)
		}
		write.MaxPoints = *in.MaxPoints
		var err error
		if write.Name, err = requiredText(label+".name", in.Name, domain.MaxCriterionNameRunes, false); err != nil {
			return nil, err
		}
		if write.ScoringGuide, err = requiredText(label+".scoring_guide", in.ScoringGuide, domain.MaxScoringGuideRunes, true); err != nil {
			return nil, err
		}
		out = append(out, write)
	}
	return out, nil
}
