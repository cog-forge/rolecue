//go:build integration

package tests

import (
	"sync"
	"testing"

	"github.com/google/uuid"
)

// check_violation 23514, unique_violation 23505, foreign_key_violation 23503.
func TestProfileCreationCreatesExactlyOnePendingRoadmap(t *testing.T) {
	e := newEnv(t)
	owner := e.user(t, "candidate")
	profile := e.profile(t, owner)

	roadmap := e.roadmap(t, owner, profile.ID)
	if roadmap.GenerationStatus != "pending" || roadmap.RoleProfileID != profile.ID {
		t.Fatalf("roadmap = %+v, want pending for the new profile", roadmap)
	}
	if n := e.count(t, `SELECT count(*) FROM roadmaps WHERE role_profile_id=$1`, profile.ID); n != 1 {
		t.Fatalf("roadmaps of the profile = %d, want 1", n)
	}
	if n := e.count(t, `SELECT count(*) FROM roadmap_items WHERE roadmap_id=$1`, roadmap.ID); n != 0 {
		t.Fatalf("a new roadmap has %d topics, want none", n)
	}
	_, err := e.pool.Exec(e.ctx, `INSERT INTO roadmaps(role_profile_id) VALUES($1)`, profile.ID)
	if sqlState(err) != "23505" {
		t.Fatalf("a second roadmap for one profile: %v, want unique violation", err)
	}
}

func TestProfileAndRoadmapAreOneTransaction(t *testing.T) {
	e := newEnv(t)
	owner := e.user(t, "candidate")
	jdCount := e.count(t, `SELECT count(*) FROM job_descriptions`)
	roadmapCount := e.count(t, `SELECT count(*) FROM roadmaps`)
	profileCount := e.count(t, `SELECT count(*) FROM role_profiles WHERE candidate_id=$1`, owner)

	// The same statement the repository runs, rolled back after the insert: neither row may survive.
	tx, err := e.pool.Begin(e.ctx)
	if err != nil {
		t.Fatal(err)
	}
	var jd uuid.UUID
	if err = tx.QueryRow(e.ctx, `INSERT INTO job_descriptions(raw_text) VALUES('x') RETURNING id`).Scan(&jd); err != nil {
		t.Fatal(err)
	}
	var profile uuid.UUID
	if err = tx.QueryRow(e.ctx, `WITH profile AS (INSERT INTO role_profiles(jd_id,candidate_id,title) VALUES($1,$2,'t') RETURNING id),
 roadmap AS (INSERT INTO roadmaps(role_profile_id) SELECT id FROM profile) SELECT id FROM profile`, jd, owner).Scan(&profile); err != nil {
		t.Fatal(err)
	}
	if err = tx.Rollback(e.ctx); err != nil {
		t.Fatal(err)
	}
	if e.count(t, `SELECT count(*) FROM roadmaps`) != roadmapCount ||
		e.count(t, `SELECT count(*) FROM role_profiles WHERE candidate_id=$1`, owner) != profileCount ||
		e.count(t, `SELECT count(*) FROM job_descriptions`) != jdCount {
		t.Fatal("a rolled back creation left a profile, roadmap or job description behind")
	}
}

func TestRoadmapReadsAreOwnerScoped(t *testing.T) {
	e := newEnv(t)
	owner, stranger := e.user(t, "candidate"), e.user(t, "candidate")
	profile := e.profile(t, owner)
	roadmap := e.roadmap(t, owner, profile.ID)
	items := e.publish(t, roadmap.ID, "SQL", "Go concurrency")

	if _, err := e.roadmaps.GetByProfile(e.ctx, stranger, profile.ID); err == nil {
		t.Fatal("a stranger read the roadmap")
	}
	if got, err := e.roadmaps.ListItems(e.ctx, stranger, profile.ID); err != nil || len(got) != 0 {
		t.Fatalf("a stranger listed %d topics (err=%v)", len(got), err)
	}
	if _, err := e.roadmaps.GetItem(e.ctx, stranger, profile.ID, items[0].ID); err == nil {
		t.Fatal("a stranger read a topic")
	}
	other := e.profile(t, owner)
	if _, err := e.roadmaps.GetItem(e.ctx, owner, other.ID, items[0].ID); err == nil {
		t.Fatal("a topic was read through another profile of the same owner")
	}
	got, err := e.roadmaps.ListItems(e.ctx, owner, profile.ID)
	if err != nil || len(got) != 2 || got[0].Title != "SQL" || got[1].Title != "Go concurrency" || got[1].Description != "Go concurrency scope" {
		t.Fatalf("owner items = %+v (err=%v)", got, err)
	}
}

func TestSchemaRejectsInvalidRows(t *testing.T) {
	e := newEnv(t)
	owner := e.user(t, "candidate")
	profile := e.profile(t, owner)
	roadmap := e.roadmap(t, owner, profile.ID)

	cases := map[string]string{
		"unknown status":  `UPDATE roadmaps SET generation_status='done' WHERE id='` + roadmap.ID.String() + `'`,
		"blank title":     `INSERT INTO roadmap_items(roadmap_id,title,position) VALUES('` + roadmap.ID.String() + `','  ',0)`,
		"negative order":  `INSERT INTO roadmap_items(roadmap_id,title,position) VALUES('` + roadmap.ID.String() + `','SQL',-1)`,
		"missing roadmap": `INSERT INTO roadmap_items(roadmap_id,title,position) VALUES(gen_random_uuid(),'SQL',0)`,
	}
	for name, sql := range cases {
		if _, err := e.pool.Exec(e.ctx, sql); err == nil {
			t.Errorf("%s was accepted", name)
		}
	}
}

func TestAssessmentKeysAndAssignmentForeignKey(t *testing.T) {
	e := newEnv(t)
	owner := e.user(t, "candidate")
	profile := e.profile(t, owner)
	items := e.publish(t, e.roadmap(t, owner, profile.ID).ID, "SQL", "Go")
	question := e.question(t, profile.JDID, items[0].ID)
	interview := e.interview(t, profile.ID, question, uuid.Nil)

	e.exec(t, `INSERT INTO roadmap_assessments(roadmap_item_id,interview_id,question_position) VALUES($1,$2,1)`, items[0].ID, interview)
	// One core question of a session has one primary topic.
	if _, err := e.pool.Exec(e.ctx, `INSERT INTO roadmap_assessments(roadmap_item_id,interview_id,question_position) VALUES($1,$2,1)`, items[1].ID, interview); sqlState(err) != "23505" {
		t.Fatalf("second topic for one assignment: %v, want unique violation", err)
	}
	// The assignment must exist: position 2 was never assigned.
	if _, err := e.pool.Exec(e.ctx, `INSERT INTO roadmap_assessments(roadmap_item_id,interview_id,question_position) VALUES($1,$2,2)`, items[0].ID, interview); sqlState(err) != "23503" {
		t.Fatalf("assessment of an unassigned position: %v, want foreign key violation", err)
	}
}

func TestQuestionAndFocusMustStayInsideTheirProfile(t *testing.T) {
	e := newEnv(t)
	owner := e.user(t, "candidate")
	mine, theirs := e.profile(t, owner), e.profile(t, owner)
	myItems := e.publish(t, e.roadmap(t, owner, mine.ID).ID, "SQL")
	theirItems := e.publish(t, e.roadmap(t, owner, theirs.ID).ID, "Go")

	// A bank question maps to a topic of its own profile only.
	own := e.question(t, mine.JDID, myItems[0].ID)
	if _, err := e.pool.Exec(e.ctx, `INSERT INTO core_questions(jd_id,content,roadmap_item_id) VALUES($1,'x',$2)`, mine.JDID, theirItems[0].ID); sqlState(err) != "23514" {
		t.Fatalf("question mapped to a foreign topic: %v, want check violation", err)
	}
	if _, err := e.pool.Exec(e.ctx, `UPDATE core_questions SET roadmap_item_id=$2 WHERE id=$1`, own, theirItems[0].ID); sqlState(err) != "23514" {
		t.Fatalf("question remapped to a foreign topic: %v, want check violation", err)
	}
	// A session focuses a topic of its own profile only.
	if _, err := e.pool.Exec(e.ctx, `INSERT INTO interviews(role_profile_id,type,focus_item_id) VALUES($1,'practice',$2)`, mine.ID, theirItems[0].ID); sqlState(err) != "23514" {
		t.Fatalf("focus on a foreign topic: %v, want check violation", err)
	}
	e.exec(t, `INSERT INTO interviews(role_profile_id,type,focus_item_id) VALUES($1,'practice',$2)`, mine.ID, myItems[0].ID)
}

// A retried or racing PDF completion must give one profile with one Roadmap, never a second of either.
func TestConcurrentPDFCompletionsCreateOneProfileAndOneRoadmap(t *testing.T) {
	e := newEnv(t)
	owner := e.user(t, "candidate")
	key := uuid.New()
	if err := e.profiles.RecordUpload(e.ctx, owner, key); err != nil {
		t.Fatal(err)
	}
	raw := "Senior backend engineer using Go, concurrency, caching and PostgreSQL."

	ids := make(chan uuid.UUID, 6)
	var wg sync.WaitGroup
	for range 6 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			profile, err := e.profiles.CreatePDFProfile(e.ctx, owner, raw, key.String())
			if err != nil {
				t.Error(err)
				return
			}
			ids <- profile.ID
		}()
	}
	wg.Wait()
	close(ids)
	var profile uuid.UUID
	for id := range ids {
		if profile != uuid.Nil && id != profile {
			t.Fatalf("two profiles from one upload: %s and %s", profile, id)
		}
		profile = id
	}
	if profile == uuid.Nil {
		t.Fatal("no profile was created")
	}
	t.Cleanup(func() { e.removeProfile(profile, owner) })
	if n := e.count(t, `SELECT count(*) FROM role_profiles WHERE candidate_id=$1`, owner); n != 1 {
		t.Fatalf("profiles = %d, want 1", n)
	}
	if n := e.count(t, `SELECT count(*) FROM roadmaps WHERE role_profile_id=$1 AND generation_status='pending'`, profile); n != 1 {
		t.Fatalf("pending roadmaps = %d, want 1", n)
	}
}
