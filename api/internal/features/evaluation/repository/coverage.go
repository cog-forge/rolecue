package repository

import (
	"context"

	"github.com/cog-forge/rolecue/api/internal/features/evaluation/domain"
	"github.com/google/uuid"
)

// maxSuggestedRubrics bounds the system rubrics suggested per missing metric.
const maxSuggestedRubrics = 3

// PostingCoverage reads the bank of the posting's Job Description and the posting's active metrics through
// db (a pool, or the transaction of a caller that holds the posting lock) and applies the shared coverage
// rule with the session question limit. It is the one reading used by the Recruiter's bank screen,
// submission, Admin approval and posting weight changes; the interview start reads the same facts with its
// own copy of the queries.
func PostingCoverage(ctx context.Context, db DBTX, postingID uuid.UUID, limit int) (domain.CoverageReport, error) {
	q := New(db)
	jdID, err := q.GetCoveragePostingJD(ctx, postingID)
	if err != nil {
		return domain.CoverageReport{}, persistenceError(err)
	}
	return BankCoverage(ctx, db, postingID, jdID, limit)
}

// ActiveMetricIDs returns the metrics a posting is scored on, or the whole metric catalog when postingID is
// nil (a practice session has no posting and so no weights). It is the active-metric rule of evaluation:
// saved weights with a positive percentage, or the catalog when none are saved.
func ActiveMetricIDs(ctx context.Context, db DBTX, postingID *uuid.UUID) ([]uuid.UUID, error) {
	q := New(db)
	if postingID != nil {
		ids, err := q.ListCoveragePostingMetrics(ctx, *postingID)
		if err != nil {
			return nil, persistenceError(err)
		}
		return ids, nil
	}
	metrics, err := q.ListMetrics(ctx)
	if err != nil {
		return nil, persistenceError(err)
	}
	ids := make([]uuid.UUID, 0, len(metrics))
	for _, metric := range metrics {
		ids = append(ids, metric.ID)
	}
	return ids, nil
}

// BankCoverage is PostingCoverage for a caller that already knows the posting's Job Description.
func BankCoverage(ctx context.Context, db DBTX, postingID, jdID uuid.UUID, limit int) (domain.CoverageReport, error) {
	q := New(db)
	rows, err := q.ListCoverageBank(ctx, jdID)
	if err != nil {
		return domain.CoverageReport{}, persistenceError(err)
	}
	bank := make([]domain.CoverageQuestion, 0, len(rows))
	for _, row := range rows {
		bank = append(bank, domain.CoverageQuestion{ID: row.ID, Metrics: row.MetricIds})
	}
	active, err := q.ListCoveragePostingMetrics(ctx, postingID)
	if err != nil {
		return domain.CoverageReport{}, persistenceError(err)
	}
	verdict, err := domain.CheckCoverage(bank, active, limit)
	if err != nil {
		return domain.CoverageReport{}, persistenceError(err)
	}
	names, suggestions, err := coverageLabels(ctx, q, verdict)
	if err != nil {
		return domain.CoverageReport{}, err
	}
	return domain.NewCoverageReport(verdict, limit, names, suggestions), nil
}

// coverageLabels reads the names of the metrics and, for the missing ones, the system rubrics that cover
// them. Both are read only when there is something missing to name.
func coverageLabels(ctx context.Context, q *Queries, verdict domain.Coverage) (map[uuid.UUID]string, map[uuid.UUID][]domain.CoverageRubric, error) {
	if !verdict.Assessed || len(verdict.Missing) == 0 {
		return nil, nil, nil
	}
	metrics, err := q.ListMetrics(ctx)
	if err != nil {
		return nil, nil, persistenceError(err)
	}
	names := make(map[uuid.UUID]string, len(metrics))
	for _, metric := range metrics {
		names[metric.ID] = metric.Name
	}
	missing := make(map[uuid.UUID]bool, len(verdict.Missing))
	for _, id := range verdict.Missing {
		missing[id] = true
	}
	rows, err := q.ListSystemRubricMetrics(ctx)
	if err != nil {
		return nil, nil, persistenceError(err)
	}
	suggestions := make(map[uuid.UUID][]domain.CoverageRubric, len(missing))
	for _, row := range rows {
		if missing[row.MetricID] && len(suggestions[row.MetricID]) < maxSuggestedRubrics {
			suggestions[row.MetricID] = append(suggestions[row.MetricID], domain.CoverageRubric{ID: row.RubricID, Name: row.RubricName})
		}
	}
	return names, suggestions, nil
}
