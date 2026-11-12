package domain

import (
	"slices"
	"strings"

	"github.com/google/uuid"
)

// CoverageRubric names a system rubric that has a criterion for a metric: what a Recruiter can assign to a
// question to cover it.
type CoverageRubric struct {
	ID   uuid.UUID `json:"id" format:"uuid"`
	Name string    `json:"name"`
}

// CoverageMetric is a metric no question of the bank covers, with the system rubrics that would cover it.
type CoverageMetric struct {
	ID               uuid.UUID        `json:"id" format:"uuid"`
	Name             string           `json:"name"`
	SuggestedRubrics []CoverageRubric `json:"suggested_rubrics"`
}

// CoverageReport is the Recruiter's and Admin's view of the coverage of a bank (KAN-377). Passes is the
// binary gate; Covered and Total only feed the display "n of m metrics". Recruiter and Admin only: it names
// metrics and counts of a bank, which a Candidate never sees.
type CoverageReport struct {
	Passes bool `json:"passes"`
	// Assessed is false when there was nothing to assess (no question has rubric criteria, or no metric is
	// active); a screen shows no coverage warning then, because the grading-incomplete warning already
	// covers it.
	Assessed bool `json:"assessed"`
	Covered  int  `json:"covered"`
	Total    int  `json:"total"`
	// QuestionLimit is how many core questions one session asks; the check is whether that many questions
	// can cover every active metric.
	QuestionLimit int `json:"question_limit"`
	// MissingMetrics are the active metrics no question of the bank covers, by name.
	MissingMetrics []CoverageMetric `json:"missing_metrics"`
	// ExceedsLimit is true when every active metric is covered somewhere in the bank, but no QuestionLimit
	// questions cover them all; MinQuestions is how many it would take.
	ExceedsLimit bool `json:"exceeds_limit"`
	MinQuestions int  `json:"min_questions"`
}

// NewCoverageReport turns a verdict into the DTO. names maps metric ids to their names; suggestions maps a
// metric id to the system rubrics that cover it. Missing metrics come back sorted by name so the screen is
// stable.
func NewCoverageReport(c Coverage, limit int, names map[uuid.UUID]string, suggestions map[uuid.UUID][]CoverageRubric) CoverageReport {
	missing := make([]CoverageMetric, 0, len(c.Missing))
	if c.Assessed {
		for _, id := range c.Missing {
			rubrics := slices.Clone(suggestions[id])
			if rubrics == nil {
				rubrics = []CoverageRubric{}
			}
			missing = append(missing, CoverageMetric{ID: id, Name: names[id], SuggestedRubrics: rubrics})
		}
		slices.SortFunc(missing, func(a, b CoverageMetric) int {
			if byName := strings.Compare(a.Name, b.Name); byName != 0 {
				return byName
			}
			return strings.Compare(a.ID.String(), b.ID.String())
		})
	}
	return CoverageReport{
		Passes: c.Passes, Assessed: c.Assessed, Covered: c.Covered, Total: c.Total, QuestionLimit: limit,
		MissingMetrics: missing, ExceedsLimit: c.ExceedsLimit, MinQuestions: c.MinQuestions,
	}
}
