//go:build integration

package tests

import (
	"slices"
	"testing"

	"github.com/google/uuid"
)

// The card previews only required requirements, five at most, in the order the detail lists them,
// and the detail names the Candidate's own Application.
func TestSkillPreviewAndOwnApplication(t *testing.T) {
	ctx, b := newBoard(t)
	id := b.seed.posting(b.recruiter, postingSpec{title: "Skills", status: "open", createdAt: b.base, slot: 3,
		requirements: []string{"Redis", "Go", "Kafka", "PostgreSQL", "Docker", "AWS", "Terraform"}})
	var jdID uuid.UUID
	if err := b.seed.pool.QueryRow(ctx, `SELECT jd_id FROM public.job_postings WHERE id = $1`, id).Scan(&jdID); err != nil {
		t.Fatal(err)
	}
	b.seed.exec(`INSERT INTO public.requirements (jd_id, name, target, type) VALUES ($1, 'Agile', 'other', 'preferred')`, jdID)
	want := []string{"AWS", "Docker", "Go", "Kafka", "PostgreSQL"}

	items, _, err := b.repo.ListOpen(ctx, b.filter(10, 0))
	if err != nil {
		t.Fatal(err)
	}
	sameIDs(t, items, id)
	if !slices.Equal(items[0].TopSkills, want) || items[0].RequiredSkillCount != 7 {
		t.Fatalf("list preview = %v (%d), want %v (7)", items[0].TopSkills, items[0].RequiredSkillCount, want)
	}

	record, err := b.repo.GetOpen(ctx, b.candidate, id)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(record.TopSkills, want) || record.RequiredSkillCount != 7 {
		t.Fatalf("detail preview = %v (%d), want %v (7)", record.TopSkills, record.RequiredSkillCount, want)
	}
	if record.AlreadyApplied || record.ApplicationID != nil {
		t.Fatalf("before applying: already_applied = %v, application_id = %v", record.AlreadyApplied, record.ApplicationID)
	}

	applicationID := uuid.New()
	b.seed.exec(`INSERT INTO public.applications (id, job_posting_id, candidate_id) VALUES ($1, $2, $3)`, applicationID, id, b.candidate)
	b.seed.later(`DELETE FROM public.applications WHERE id = $1`, applicationID)
	applied, err := b.repo.GetOpen(ctx, b.candidate, id)
	if err != nil {
		t.Fatal(err)
	}
	if !applied.AlreadyApplied || applied.ApplicationID == nil || *applied.ApplicationID != applicationID {
		t.Fatalf("after applying: already_applied = %v, application_id = %v, want %v", applied.AlreadyApplied, applied.ApplicationID, applicationID)
	}
	other, err := b.repo.GetOpen(ctx, b.seed.user("candidate", nil), id)
	if err != nil {
		t.Fatal(err)
	}
	if other.ApplicationID != nil {
		t.Fatal("another Candidate sees an application_id")
	}
}
