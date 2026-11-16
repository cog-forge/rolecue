package domain_test

import (
	"strings"
	"testing"

	"github.com/cog-forge/rolecue/api/internal/features/marketplace/domain"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
)

func TestStatusGuards(t *testing.T) {
	type row struct {
		status                                                      domain.Status
		edit, submit, delist, approve, reject, suspend, purchasable bool
	}
	for _, r := range []row{
		{domain.StatusDraft, true, true, false, false, false, false, false},
		{domain.StatusPending, false, false, false, true, true, false, false},
		{domain.StatusActive, false, false, true, false, false, true, true},
		{domain.StatusRejected, true, true, false, false, false, false, false},
		{domain.StatusDelisted, false, false, false, false, false, false, false},
		{domain.StatusSuspended, false, false, false, false, false, false, false},
	} {
		got := row{r.status, r.status.CanEdit(), r.status.CanSubmit(), r.status.CanDelist(), r.status.CanApprove(), r.status.CanReject(), r.status.CanSuspend(), r.status.Purchasable()}
		if got != r {
			t.Errorf("%s: got %+v want %+v", r.status, got, r)
		}
	}
	if domain.Status("nope").Valid() || !domain.StatusSuspended.Valid() {
		t.Error("Valid must accept only the six statuses")
	}
}

func valid() domain.Payload {
	desc := "  A calm interviewer  "
	return domain.Payload{Title: "  Alex  ", Description: &desc, DefaultPersonality: " calm ", RightsNote: " I made it ", PriceCoins: 20}
}

func TestPayloadNormalizeTrims(t *testing.T) {
	p, err := valid().Normalize()
	if err != nil {
		t.Fatal(err)
	}
	if p.Title != "Alex" || *p.Description != "A calm interviewer" || p.DefaultPersonality != "calm" || p.RightsNote != "I made it" {
		t.Errorf("not trimmed: %+v", p)
	}
	blank := "   "
	in := valid()
	in.Description = &blank
	if p, err = in.Normalize(); err != nil || p.Description != nil {
		t.Errorf("blank description must become nil, got %v %v", p.Description, err)
	}
}

func TestPayloadNormalizeBounds(t *testing.T) {
	long := func(n int) string { return strings.Repeat("é", n) }
	for name, mutate := range map[string]func(*domain.Payload){
		"empty title":      func(p *domain.Payload) { p.Title = " " },
		"long title":       func(p *domain.Payload) { p.Title = long(101) },
		"long description": func(p *domain.Payload) { d := long(2001); p.Description = &d },
		"empty persona":    func(p *domain.Payload) { p.DefaultPersonality = "" },
		"long persona":     func(p *domain.Payload) { p.DefaultPersonality = long(2001) },
		"empty rights":     func(p *domain.Payload) { p.RightsNote = "\n" },
		"long rights":      func(p *domain.Payload) { p.RightsNote = long(2001) },
		"zero price":       func(p *domain.Payload) { p.PriceCoins = 0 },
		"negative price":   func(p *domain.Payload) { p.PriceCoins = -1 },
		"huge price":       func(p *domain.Payload) { p.PriceCoins = 1_000_001 },
	} {
		p := valid()
		mutate(&p)
		if _, err := p.Normalize(); !apperror.IsCode(err, apperror.CodeValidation) {
			t.Errorf("%s: want validation error, got %v", name, err)
		}
	}
	for _, edge := range []func(*domain.Payload){
		func(p *domain.Payload) { p.Title = long(100) },
		func(p *domain.Payload) { p.PriceCoins = 1 },
		func(p *domain.Payload) { p.PriceCoins = 1_000_000 },
		func(p *domain.Payload) { p.RightsNote = long(2000) },
	} {
		p := valid()
		edge(&p)
		if _, err := p.Normalize(); err != nil {
			t.Errorf("edge must pass: %v", err)
		}
	}
}

func TestNormalizeReason(t *testing.T) {
	if r, err := domain.NormalizeReason("  too blurry "); err != nil || r != "too blurry" {
		t.Errorf("got %q %v", r, err)
	}
	for _, bad := range []string{"", "  ", strings.Repeat("x", 2001)} {
		if _, err := domain.NormalizeReason(bad); !apperror.IsCode(err, apperror.CodeValidation) {
			t.Errorf("%d chars: want validation error, got %v", len(bad), err)
		}
	}
}
