//go:build integration

package tests

import (
	"testing"

	"github.com/cog-forge/rolecue/api/internal/features/jobposting/domain"
)

func TestStageFiltersApplyBeforePagination(t *testing.T) {
	ctx, f := newFixture(t)
	f.create(t, ctx, "Preparing")
	submitted := f.create(t, ctx, "Submitted")
	unfunded := f.create(t, ctx, "Awaiting funding")
	live := f.create(t, ctx, "Live")
	f.seed.exec(`UPDATE job_postings SET review_submitted = true WHERE id=$1`, submitted)
	f.seed.exec(`UPDATE job_postings SET status='open', review_submitted=true WHERE id IN ($1,$2)`, unfunded, live)
	f.seed.exec(`UPDATE job_postings SET interview_slot=2 WHERE id=$1`, live)
	for _, tt := range []struct {
		name   string
		filter domain.ListFilter
		title  string
	}{
		{"preparing", domain.ListFilter{Status: ptr(domain.StatusPending), ReviewSubmitted: ptr(false), Limit: 1}, "Preparing"},
		{"review", domain.ListFilter{Status: ptr(domain.StatusPending), ReviewSubmitted: ptr(true), Limit: 1}, "Submitted"},
		{"funding", domain.ListFilter{Status: ptr(domain.StatusOpen), Funded: ptr(false), Limit: 1}, "Awaiting funding"},
		{"live", domain.ListFilter{Status: ptr(domain.StatusOpen), Funded: ptr(true), Limit: 1}, "Live"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			items, total, err := f.repo.List(ctx, f.recruiter, tt.filter)
			if err != nil || total != 1 || len(items) != 1 || items[0].Title != tt.title {
				t.Fatalf("items=%+v total=%d err=%v", items, total, err)
			}
			tt.filter.Offset = 1
			items, total, err = f.repo.List(ctx, f.recruiter, tt.filter)
			if err != nil || total != 1 || len(items) != 0 {
				t.Fatalf("pagination changed total: %d %+v %v", total, items, err)
			}
		})
	}
}
