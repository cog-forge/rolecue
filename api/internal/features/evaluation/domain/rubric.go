package domain

import (
	"time"

	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

// Rubric management (KAN-326, RB-02). A rubric ("Barem") is a reusable grading guide: criteria, each tied to
// one metric of the catalog. Admins manage the system rubrics; a Recruiter manages private ones and may read
// and copy the active system rubrics.

// Rubric scopes, as stored in rubrics.scope.
const (
	RubricScopeSystem    = "system"
	RubricScopeRecruiter = "recruiter"
)

// Roles that may use the rubric API. Any other role (a Candidate) is refused.
const (
	RubricRoleAdmin     = "admin"
	RubricRoleRecruiter = "recruiter"
)

// Limits the server applies to rubric content. They are the proposal of KAN-326 for the front end to share;
// the database only enforces what makes the data unusable (blank text, max_points <= 0, position < 1).
const (
	MaxRubricNameRunes         = 120
	MaxRubricDescriptionRunes  = 1000
	MaxRubricQuestionTypeRunes = 64
	MaxRubricCriteria          = 10
	MaxCriterionNameRunes      = 160
	MaxCriterionPoints         = 10
	MaxScoringGuideRunes       = 2000
)

// Page sizes of the rubric list.
const (
	DefaultRubricPageSize = 20
	MaxRubricPageSize     = 100
)

// Status filters of the rubric list.
const (
	RubricStatusActive   = "active"
	RubricStatusArchived = "archived"
	RubricStatusAll      = "all"
)

// Scope filters of the rubric list: the system templates or the caller's own rubrics.
const (
	RubricListSystem = "system"
	RubricListOwn    = "own"
)

var (
	ErrRubricNotFound = apperror.New(apperror.CodeRubricNotFound, "rubric not found")
	ErrRubricLocked   = apperror.New(apperror.CodeRubricLocked, "this rubric is used by questions and can no longer be edited; copy it to change it")
	ErrRubricInUse    = apperror.New(apperror.CodeRubricInUse, "this rubric is used by questions and cannot be deleted; archive it instead")
	ErrRubricStale    = apperror.New(apperror.CodeRubricStale, "the rubric was changed by someone else; reload it and try again")
)

// RubricLocked is the single place that decides when a rubric's content is frozen. For now it is the
// reference check of KAN-326: any question pointing at the rubric (core_questions.rubric_id) locks it.
// RB-07 refines this to approved postings and started sessions; archiving never needs it.
func RubricLocked(referenceCount int32) bool { return referenceCount > 0 }

// RubricCriterion is one gradable point of a rubric. A candidate earns an integer from 0 to MaxPoints.
type RubricCriterion struct {
	ID           uuid.UUID `json:"id" format:"uuid"`
	MetricID     uuid.UUID `json:"metric_id" format:"uuid"`
	MetricName   string    `json:"metric_name"`
	Name         string    `json:"name"`
	MaxPoints    int32     `json:"max_points"`
	ScoringGuide string    `json:"scoring_guide"`
	Position     int32     `json:"position"`
}

// Rubric is a rubric as the API shows it. The list leaves Criteria out and fills CriteriaCount and
// MaxPoints; the detail view fills all of them. UpdatedAt is the concurrency token: send it back unchanged
// to change the rubric, and the change is refused with RUBRIC_STALE when someone else saved in between.
type Rubric struct {
	ID            uuid.UUID `json:"id" format:"uuid"`
	Name          string    `json:"name"`
	Description   string    `json:"description"`
	QuestionType  string    `json:"question_type"`
	Scope         string    `json:"scope" enums:"system,recruiter"`
	IsActive      bool      `json:"is_active"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
	CriteriaCount int32     `json:"criteria_count"`
	MaxPoints     int32     `json:"max_points"`
	// IsOwn is true for the caller's private rubrics. CanManage tells whether the caller may change,
	// archive, delete this rubric at all (an Admin for system rubrics, the owner for private ones).
	IsOwn     bool `json:"is_own"`
	CanManage bool `json:"can_manage"`
	// Locked and ReferenceCount are shown to whoever can manage the rubric: how many questions use it and
	// whether that freezes its content. ReferenceCount is omitted for rubrics the caller cannot manage.
	Locked         bool              `json:"locked"`
	ReferenceCount *int32            `json:"reference_count,omitempty"`
	Criteria       []RubricCriterion `json:"criteria,omitempty"`

	// OwnerID and References come from the repository and never leave the API.
	OwnerID    *uuid.UUID `json:"-"`
	References int32      `json:"-"`
}

// RubricAccess is who is asking. The repository applies it in SQL, so a rubric the caller may not see is
// never returned, whatever id was asked for.
type RubricAccess struct {
	UserID uuid.UUID
	Role   string
}

// RubricListQuery filters the list. Scope and QuestionType are empty for no filter.
type RubricListQuery struct {
	Scope        string
	QuestionType string
	Status       string
	Limit        int32
	Offset       int32
}

// CriterionInput is one criterion of a create or update request. Pointers tell a missing field from a zero
// value. ID is set only when an update keeps an existing criterion; a criterion left out of an update is
// removed, one without an ID is added.
type CriterionInput struct {
	ID           *uuid.UUID
	MetricID     *uuid.UUID
	Name         *string
	MaxPoints    *int32
	ScoringGuide *string
}

// RubricCreate is a request to create a rubric from scratch. The scope and owner follow the caller's role.
type RubricCreate struct {
	Name         *string
	Description  *string
	QuestionType *string
	Criteria     []CriterionInput
}

// RubricPatch changes some fields of a rubric. UpdatedAt is required. Criteria, when present, is the whole
// new criteria set in order. IsActive false archives the rubric and true restores it.
type RubricPatch struct {
	UpdatedAt    *time.Time
	Name         *string
	Description  *string
	QuestionType *string
	Criteria     *[]CriterionInput
	IsActive     *bool
}

// RubricWrite is a validated rubric ready to be stored.
type RubricWrite struct {
	Name         string
	Description  string
	QuestionType string
	Scope        string
	OwnerID      *uuid.UUID
	Criteria     []CriterionWrite
}

// CriterionWrite is a validated criterion. ID is non-nil when an update keeps an existing criterion.
type CriterionWrite struct {
	ID           *uuid.UUID
	MetricID     uuid.UUID
	Name         string
	MaxPoints    int32
	ScoringGuide string
}

// RubricCopy describes the new rubric a copy creates. The criteria are copied in SQL from the source.
type RubricCopy struct {
	Name    string
	Scope   string
	OwnerID *uuid.UUID
}

// RubricUpdate is a validated change. ExpectedUpdatedAt is the token the client read; a mismatch under the
// row lock is RUBRIC_STALE. Content changes (everything except IsActive) are refused while the rubric is
// locked.
type RubricUpdate struct {
	ExpectedUpdatedAt time.Time
	Name              *string
	Description       *string
	QuestionType      *string
	Criteria          *[]CriterionWrite
	IsActive          *bool
}

// ContentChange reports whether the update touches anything other than the archive flag.
func (u RubricUpdate) ContentChange() bool {
	return u.Name != nil || u.Description != nil || u.QuestionType != nil || u.Criteria != nil
}
