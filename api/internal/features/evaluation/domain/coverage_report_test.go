package domain

import (
	"slices"
	"testing"

	"github.com/google/uuid"
)

func TestCoverageReportNamesMissingMetricsSortedWithSuggestions(t *testing.T) {
	m := metricIDs(3)
	names := map[uuid.UUID]string{m[0]: "Zeta", m[1]: "Alpha", m[2]: "Beta"}
	rubric := CoverageRubric{ID: uuid.New(), Name: "System rubric"}
	verdict, err := CheckCoverage([]CoverageQuestion{covQuestion(1, m[2])}, m, 8)
	if err != nil {
		t.Fatal(err)
	}
	report := NewCoverageReport(verdict, 8, names, map[uuid.UUID][]CoverageRubric{m[0]: {rubric}})
	got := []string{report.MissingMetrics[0].Name, report.MissingMetrics[1].Name}
	if report.Passes || report.Covered != 1 || report.Total != 3 || report.QuestionLimit != 8 || !slices.Equal(got, []string{"Alpha", "Zeta"}) {
		t.Fatalf("report = %+v", report)
	}
	if len(report.MissingMetrics[1].SuggestedRubrics) != 1 || report.MissingMetrics[0].SuggestedRubrics == nil {
		t.Fatalf("suggestions = %+v, want the rubric on Zeta and an empty (not nil) list on Alpha", report.MissingMetrics)
	}
}

func TestCoverageReportOfAnUnassessedBankNamesNothing(t *testing.T) {
	m := metricIDs(2)
	verdict, err := CheckCoverage([]CoverageQuestion{covQuestion(1)}, m, 8)
	if err != nil {
		t.Fatal(err)
	}
	report := NewCoverageReport(verdict, 8, map[uuid.UUID]string{m[0]: "A"}, nil)
	if !report.Passes || report.Assessed || report.MissingMetrics == nil || len(report.MissingMetrics) != 0 {
		t.Fatalf("report = %+v, want an unassessed pass with an empty missing list", report)
	}
}
