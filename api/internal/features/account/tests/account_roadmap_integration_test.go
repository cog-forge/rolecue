//go:build integration

// Learning Roadmap data in account erasure and export (KAN-356, RM-15). The fixture helpers come from
// account_integration_test.go.
package tests

import (
	"fmt"
	"strings"
	"testing"

	"github.com/google/uuid"
)

// roadmapOf seeds a ready Roadmap with two topics on a Role Profile of the candidate, a mapped bank question per
// topic, a graded and completed practice session with a link per topic, one more session that was started on a topic
// and never finished (a focused practice), and returns the profile and the interviews.
func (f *fixture) roadmapOf(candidate uuid.UUID) (profile uuid.UUID, items []uuid.UUID, graded, focused uuid.UUID) {
	jd, profile, roadmap := uuid.New(), uuid.New(), uuid.New()
	f.exec(`INSERT INTO public.job_descriptions (id, content_path) VALUES ($1, $2)`, jd, uuid.NewString())
	f.exec(`INSERT INTO public.role_profiles (id, jd_id, candidate_id, title, status) VALUES ($1, $2, $3, 'Roadmap role', 'confirmed')`, profile, jd, candidate)
	// A new profile already has a pending Roadmap row only through the API; these tests seed through SQL.
	f.exec(`INSERT INTO public.roadmaps (id, role_profile_id, generation_status) VALUES ($1, $2, 'ready')`, roadmap, profile)
	for i, title := range []string{"SQL", "Go concurrency"} {
		id := uuid.New()
		items = append(items, id)
		f.exec(`INSERT INTO public.roadmap_items (id, roadmap_id, title, description, position) VALUES ($1, $2, $3, $4, $5)`, id, roadmap, title, title+" scope", i+1)
	}
	graded, focused = uuid.New(), uuid.New()
	f.exec(`INSERT INTO public.interviews (id, type, role_profile_id, score, feedback, completed_at, evaluation_status) VALUES ($1, 'practice', $2, 70, 'roadmap feedback', now(), 'completed')`, graded, profile)
	for i, item := range items {
		question := uuid.New()
		f.exec(`INSERT INTO public.core_questions (id, jd_id, content, expected_answer, roadmap_item_id) VALUES ($1, $2, $3, 'EXPECTED-SECRET', $4)`, question, jd, "Roadmap question", item)
		f.exec(`INSERT INTO public.interview_questions (interview_id, position, core_question_id, score, spoken_text, asked_at) VALUES ($1, $2, $3, $4, 'Roadmap question', now())`, graded, i+1, question, 60+10*i)
		f.exec(`INSERT INTO public.conversation_turns (interview_id, position, question, answer) VALUES ($1, $2, 'Roadmap question', 'roadmap answer')`, graded, i+1)
		f.exec(`INSERT INTO public.roadmap_assessments (roadmap_item_id, interview_id, question_position) VALUES ($1, $2, $3)`, item, graded, i+1)
	}
	// Started on the first topic and never finished: focus_item_id restricts the topic from being deleted on its own.
	f.exec(`INSERT INTO public.interviews (id, type, role_profile_id, focus_item_id) VALUES ($1, 'practice', $2, $3)`, focused, profile, items[0])
	return profile, items, graded, focused
}

type roadmapRows struct{ roadmaps, items, assessments, focused, mapped int }

func (f *fixture) roadmapRows(user uuid.UUID) roadmapRows {
	return roadmapRows{
		roadmaps:    f.count(`SELECT count(*) FROM public.roadmaps r JOIN public.role_profiles rp ON rp.id = r.role_profile_id WHERE rp.candidate_id = $1`, user),
		items:       f.count(`SELECT count(*) FROM public.roadmap_items ri JOIN public.roadmaps r ON r.id = ri.roadmap_id JOIN public.role_profiles rp ON rp.id = r.role_profile_id WHERE rp.candidate_id = $1`, user),
		assessments: f.count(`SELECT count(*) FROM public.roadmap_assessments ra JOIN public.roadmap_items ri ON ri.id = ra.roadmap_item_id JOIN public.roadmaps r ON r.id = ri.roadmap_id JOIN public.role_profiles rp ON rp.id = r.role_profile_id WHERE rp.candidate_id = $1`, user),
		focused:     f.count(`SELECT count(*) FROM public.interviews i JOIN public.role_profiles rp ON rp.id = i.role_profile_id WHERE rp.candidate_id = $1 AND i.focus_item_id IS NOT NULL`, user),
		mapped:      f.count(`SELECT count(*) FROM public.core_questions cq JOIN public.roadmap_items ri ON ri.id = cq.roadmap_item_id JOIN public.roadmaps r ON r.id = ri.roadmap_id JOIN public.role_profiles rp ON rp.id = r.role_profile_id WHERE rp.candidate_id = $1`, user),
	}
}

func TestEraseRemovesRoadmapRowsInOrderAndLeavesOthersUntouched(t *testing.T) {
	f := newFixture(t)
	alice, bob := f.user("candidate"), f.user("candidate")
	f.roadmapOf(alice.ID)
	f.roadmapOf(bob.ID)
	aliceBefore, bobBefore := f.roadmapRows(alice.ID), f.roadmapRows(bob.ID)
	want := roadmapRows{roadmaps: 1, items: 2, assessments: 2, focused: 1, mapped: 2}
	if aliceBefore != want || bobBefore != want {
		t.Fatalf("seed is wrong: alice %+v bob %+v", aliceBefore, bobBefore)
	}

	// Topics are referenced by assessments, a focused session and bank questions, all of which restrict or detach:
	// none of them may stop the erasure.
	if err := f.svc.Erase(f.ctx, alice, "DELETE"); err != nil {
		t.Fatalf("erase with Roadmap rows: %v", err)
	}
	if got := f.roadmapRows(alice.ID); got != (roadmapRows{}) {
		t.Fatalf("alice still has Roadmap rows: %+v", got)
	}
	if got := f.roadmapRows(bob.ID); got != bobBefore {
		t.Fatalf("bob changed: before %+v after %+v", bobBefore, got)
	}
	// No orphans anywhere: no link without its question, no item without its Roadmap, no focus on a missing topic.
	for name, sql := range map[string]string{
		"assessment without a question": `SELECT count(*) FROM public.roadmap_assessments ra LEFT JOIN public.interview_questions iq ON iq.interview_id = ra.interview_id AND iq.position = ra.question_position WHERE iq.interview_id IS NULL`,
		"assessment without a topic":    `SELECT count(*) FROM public.roadmap_assessments ra LEFT JOIN public.roadmap_items ri ON ri.id = ra.roadmap_item_id WHERE ri.id IS NULL`,
		"item without a roadmap":        `SELECT count(*) FROM public.roadmap_items ri LEFT JOIN public.roadmaps r ON r.id = ri.roadmap_id WHERE r.id IS NULL`,
		"roadmap without a profile":     `SELECT count(*) FROM public.roadmaps r LEFT JOIN public.role_profiles rp ON rp.id = r.role_profile_id WHERE rp.id IS NULL`,
		"focus on a missing topic":      `SELECT count(*) FROM public.interviews i LEFT JOIN public.roadmap_items ri ON ri.id = i.focus_item_id WHERE i.focus_item_id IS NOT NULL AND ri.id IS NULL`,
	} {
		if n := f.count(sql); n != 0 {
			t.Errorf("%s: %d orphans", name, n)
		}
	}
	// Retried erasure of a gone account is safe.
	if err := f.repo.Erase(f.ctx, alice.ID); err != nil {
		t.Fatalf("second erase: %v", err)
	}
}

func TestExportIncludesOnlyTheOwnersRoadmapAndNoText(t *testing.T) {
	f := newFixture(t)
	recruiter, alice, bob := f.user("recruiter"), f.user("candidate"), f.user("candidate")
	aliceProfile, items, graded, _ := f.roadmapOf(alice.ID)
	f.roadmapOf(bob.ID)
	// A recruitment interview of Alice with a mapped question and a (never legitimate) link: it must not be exported.
	posting := f.posting(recruiter.ID, "open", uuid.NewString())
	_, recruitment := f.application(alice.ID, posting, "interviewed", "alice-cv", "alice-rec")
	f.exec(`INSERT INTO public.core_questions (id, jd_id, content, roadmap_item_id) SELECT gen_random_uuid(), rp.jd_id, 'recruitment mapped', $2 FROM public.role_profiles rp WHERE rp.id = $1`, aliceProfile, items[0])
	f.exec(`INSERT INTO public.interview_questions (interview_id, position, core_question_id, score)
		SELECT $1, 2, cq.id, 99 FROM public.core_questions cq WHERE cq.content = 'recruitment mapped' AND cq.roadmap_item_id = $2`, recruitment, items[0])
	f.exec(`INSERT INTO public.roadmap_assessments (roadmap_item_id, interview_id, question_position) VALUES ($1, $2, 2)`, items[0], recruitment)

	out, err := f.svc.Export(f.ctx, alice)
	if err != nil {
		t.Fatal(err)
	}
	if len(out.RoleProfiles) != 1 || out.RoleProfiles[0].ID != aliceProfile || out.RoleProfiles[0].Roadmap == nil {
		t.Fatalf("role profiles = %+v", out.RoleProfiles)
	}
	rm := out.RoleProfiles[0].Roadmap
	if rm.GenerationStatus != "ready" || len(rm.Topics) != 2 || rm.Topics[0].Title != "SQL" || rm.Topics[1].Title != "Go concurrency" || rm.Topics[0].Position != 1 {
		t.Fatalf("roadmap = %+v", rm)
	}
	if len(rm.Results) != 2 {
		t.Fatalf("results = %+v, want the two practice links (the recruitment link and Bob's rows are not the owner's practice evidence)", rm.Results)
	}
	for i, r := range rm.Results {
		if r.InterviewID != graded || r.QuestionNumber != int32(i+1) || r.TopicID != items[i] || r.Score == nil || *r.Score != float64(60+10*i) {
			t.Fatalf("result %d = %+v", i, r)
		}
	}
	// Practice interviews exported by the account package already hold the transcript; the Roadmap part adds no
	// text of its own and never the bank's expected answer.
	for _, forbidden := range []string{"EXPECTED-SECRET", "roadmap answer", "recruitment mapped"} {
		if containsAnywhere(out.RoleProfiles, forbidden) {
			t.Errorf("the role profile part of the export contains %q", forbidden)
		}
	}
	// Bob exports only his own.
	bobOut, err := f.svc.Export(f.ctx, bob)
	if err != nil || len(bobOut.RoleProfiles) != 1 || bobOut.RoleProfiles[0].Roadmap == nil || len(bobOut.RoleProfiles[0].Roadmap.Results) != 2 {
		t.Fatalf("bob export = %+v, err %v", bobOut.RoleProfiles, err)
	}
	if bobOut.RoleProfiles[0].ID == aliceProfile {
		t.Fatal("alice's profile leaked into bob's export")
	}
}

func TestExportRoleProfileWithoutPracticeOrRoadmapContent(t *testing.T) {
	f := newFixture(t)
	alice := f.user("candidate")
	jd, profile := uuid.New(), uuid.New()
	f.exec(`INSERT INTO public.job_descriptions (id, content_path) VALUES ($1, $2)`, jd, uuid.NewString())
	f.exec(`INSERT INTO public.role_profiles (id, jd_id, candidate_id, title) VALUES ($1, $2, $3, 'Pending role')`, profile, jd, alice.ID)
	f.exec(`INSERT INTO public.roadmaps (role_profile_id) VALUES ($1)`, profile)
	out, err := f.svc.Export(f.ctx, alice)
	if err != nil || len(out.RoleProfiles) != 1 || out.RoleProfiles[0].Roadmap == nil {
		t.Fatalf("export = %+v, err %v", out.RoleProfiles, err)
	}
	rm := out.RoleProfiles[0].Roadmap
	if rm.GenerationStatus != "pending" || rm.Topics == nil || len(rm.Topics) != 0 || rm.Results == nil || len(rm.Results) != 0 {
		t.Fatalf("roadmap = %+v, want a pending Roadmap with empty (non-null) lists", rm)
	}
}

func containsAnywhere(v any, needle string) bool {
	return strings.Contains(fmt.Sprintf("%+v", v), needle)
}
