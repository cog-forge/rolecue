//go:build integration

package tests

import (
	"testing"
	"time"

	"github.com/cog-forge/rolecue/api/internal/features/jobboard/domain"
	"github.com/cog-forge/rolecue/api/internal/features/jobboard/repository"
)

func TestPublicationWindowCountsActualVisiblePublications(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	owner := s.user("recruiter", nil)
	from := time.Date(2026, 10, 29, 0, 0, 0, 0, time.UTC)
	to := from.Add(24 * time.Hour)
	oldCreated := from.Add(-7 * 24 * time.Hour)
	first := s.posting(owner, postingSpec{title: "newly live", status: "open", slot: 2, createdAt: oldCreated})
	second := s.posting(owner, postingSpec{title: "also live", status: "open", slot: 2, createdAt: oldCreated})
	old := s.posting(owner, postingSpec{title: "old publication", status: "open", slot: 2, createdAt: from})
	hidden := s.posting(owner, postingSpec{title: "unfunded", status: "open", unfunded: true, createdAt: from})
	historical := s.posting(owner, postingSpec{title: "unknown publication", status: "open", slot: 2, createdAt: from})
	s.exec(`UPDATE job_postings SET published_at=$1 WHERE id IN($2,$3,$4)`, from, first, second, hidden)
	s.exec(`UPDATE job_postings SET published_at=$1 WHERE id=$2`, from.Add(-time.Second), old)
	repo := repository.NewRepository(pool)
	search := s.tag
	filter := domain.BoardFilter{Search: &search, PublishedFrom: &from, PublishedTo: &to, Limit: 1}
	for _, offset := range []int32{0, 1, 2} {
		filter.Offset = offset
		items, total, err := repo.ListOpen(ctx, filter)
		want := 1
		if offset == 2 {
			want = 0
		}
		if err != nil || total != 2 || len(items) != want {
			t.Fatalf("offset=%d items=%+v total=%d err=%v", offset, items, total, err)
		}
		if len(items) > 0 && (items[0].PublishedAt == nil || !items[0].PublishedAt.Equal(from)) {
			t.Fatalf("publication lost: %+v", items[0])
		}
	}
	detail, err := repo.GetOpen(ctx, s.user("candidate", nil), historical)
	if err != nil || detail.PublishedAt != nil {
		t.Fatalf("historical publication was invented: %+v %v", detail.PublishedAt, err)
	}
}
