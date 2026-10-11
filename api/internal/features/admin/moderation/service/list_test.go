package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/cog-forge/rolecue/api/internal/features/admin/moderation/domain"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

func TestListForwardsFilter(t *testing.T) {
	status, recruiter := domain.PostingStatusRejected, uuid.New()
	from := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	to := from.AddDate(0, 1, 0)
	filter := domain.ListFilter{Status: &status, RecruiterID: &recruiter, From: &from, To: &to, Limit: 50, Offset: 100}
	repo := &fakeRepository{}
	items, total, err := newService(t, repo).List(context.Background(), "admin", filter)
	if err != nil || len(items) != 1 || total != 1 {
		t.Fatalf("items=%d total=%d err=%v", len(items), total, err)
	}
	if repo.filter.Status != &status || repo.filter.RecruiterID != &recruiter || repo.filter.From != &from ||
		repo.filter.To != &to || repo.filter.Limit != 50 || repo.filter.Offset != 100 {
		t.Fatalf("filter not forwarded exactly: %+v", repo.filter)
	}
}

func TestListRejectsInvalidFilter(t *testing.T) {
	bad := domain.PostingStatus("archived")
	day := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	earlier := day.Add(-time.Hour)
	for _, tt := range []struct {
		name   string
		filter domain.ListFilter
	}{
		{"unknown status", domain.ListFilter{Status: &bad, Limit: 20}},
		{"zero limit", domain.ListFilter{Limit: 0}},
		{"limit over 100", domain.ListFilter{Limit: 101}},
		{"negative offset", domain.ListFilter{Limit: 20, Offset: -1}},
		{"empty window", domain.ListFilter{Limit: 20, From: &day, To: &day}},
		{"reversed window", domain.ListFilter{Limit: 20, From: &day, To: &earlier}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			repo := &fakeRepository{}
			_, _, err := newService(t, repo).List(context.Background(), "admin", tt.filter)
			assertCode(t, err, apperror.CodeValidation)
			if repo.listCalls != 0 {
				t.Fatal("invalid filter reached the repository")
			}
		})
	}
}

func TestGetReturnsPosting(t *testing.T) {
	repo := postingIn(domain.PostingStatusPending)
	got, err := newService(t, repo).Get(context.Background(), "admin", repo.posting.ID)
	if err != nil || got.ID != repo.posting.ID || got.Status != domain.PostingStatusPending || repo.getCalls != 1 {
		t.Fatalf("got=%+v err=%v calls=%d", got, err, repo.getCalls)
	}
}

func TestListAndGetPassRepositoryErrorsThrough(t *testing.T) {
	sentinel := errors.New("db down")
	repo := &fakeRepository{listErr: sentinel, getErr: sentinel}
	s := newService(t, repo)
	if _, _, err := s.List(context.Background(), "admin", domain.ListFilter{Limit: 20}); !errors.Is(err, sentinel) {
		t.Fatalf("List error = %v", err)
	}
	if _, err := s.Get(context.Background(), "admin", uuid.New()); !errors.Is(err, sentinel) {
		t.Fatalf("Get error = %v", err)
	}
}
