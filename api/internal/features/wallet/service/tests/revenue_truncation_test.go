package tests

import (
	"encoding/json"
	"testing"

	"github.com/cog-forge/rolecue/api/internal/features/wallet/domain"
	"github.com/cog-forge/rolecue/api/internal/features/wallet/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A window with more top-ups than the repository reads must say its series is partial instead of
// presenting a series that silently disagrees with the summary totals.
func TestRevenueReportExposesTruncatedSeries(t *testing.T) {
	req := domain.RevenueReportRequest{From: vnDay(2026, 10, 1), To: vnDay(2026, 10, 8), Compare: domain.ComparePreviousPeriod}
	for _, tc := range []struct {
		name                string
		current, comparison bool
		want                bool
	}{
		{"complete", false, false, false},
		{"current window capped", true, false, true},
		{"comparison window capped", false, true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			report := service.BuildRevenueReport(req,
				domain.RevenuePeriodData{Truncated: tc.current}, domain.RevenuePeriodData{Truncated: tc.comparison}, 0, 0, 0)
			assert.Equal(t, tc.want, report.SeriesTruncated)

			raw, err := json.Marshal(report)
			require.NoError(t, err)
			var decoded map[string]any
			require.NoError(t, json.Unmarshal(raw, &decoded))
			assert.Equal(t, tc.want, decoded["series_truncated"])
		})
	}
}
