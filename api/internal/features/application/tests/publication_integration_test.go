//go:build integration

package tests

import (
	"testing"
	"time"
)

func TestFirstFundingPublishesOnceAndFailureDoesNotPublish(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newService(t, pool)
	owner := s.user("recruiter")
	s.wallet(owner, 10)
	posting := s.posting(owner, "open", nil)
	var before *time.Time
	if err := pool.QueryRow(ctx, `SELECT published_at FROM job_postings WHERE id=$1`, posting).Scan(&before); err != nil || before != nil {
		t.Fatalf("unfunded publication: %v %v", before, err)
	}
	if _, err := svc.FundCapacity(ctx, owner, "recruiter", posting, 2); err != nil {
		t.Fatal(err)
	}
	var published time.Time
	if err := pool.QueryRow(ctx, `SELECT published_at FROM job_postings WHERE id=$1`, posting).Scan(&published); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.FundCapacity(ctx, owner, "recruiter", posting, 2); err != nil {
		t.Fatal(err)
	}
	var retried time.Time
	if err := pool.QueryRow(ctx, `SELECT published_at FROM job_postings WHERE id=$1`, posting).Scan(&retried); err != nil || !retried.Equal(published) {
		t.Fatalf("retry republished: %v %v", retried, err)
	}
	poor := s.user("recruiter")
	s.wallet(poor, 0)
	unfunded := s.posting(poor, "open", nil)
	if _, err := svc.FundCapacity(ctx, poor, "recruiter", unfunded, 2); err == nil {
		t.Fatal("insufficient funding succeeded")
	}
	var failed *time.Time
	if err := pool.QueryRow(ctx, `SELECT published_at FROM job_postings WHERE id=$1`, unfunded).Scan(&failed); err != nil || failed != nil {
		t.Fatalf("failed funding published: %v %v", failed, err)
	}
}
