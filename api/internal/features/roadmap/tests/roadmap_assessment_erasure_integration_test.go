//go:build integration

package tests

import (
	"testing"

	accountrepo "github.com/cog-forge/rolecue/api/internal/features/account/repository"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

func TestRecordAssessmentIsIdempotentAndChecksEveryReference(t *testing.T) {
	e := newEnv(t)
	owner, stranger := e.user(t, "candidate"), e.user(t, "candidate")
	profile := e.profile(t, owner)
	items := e.publish(t, e.roadmap(t, owner, profile.ID).ID, "SQL", "Go")
	question := e.question(t, profile.JDID, items[0].ID)
	interview := e.interview(t, profile.ID, question, uuid.Nil)

	// A retry changes nothing.
	for attempt, want := range []bool{true, false, false} {
		got, err := e.roadmaps.RecordAssessment(e.ctx, owner, interview, items[0].ID, 1)
		if err != nil || got != want {
			t.Fatalf("attempt %d: written=%v err=%v, want %v", attempt+1, got, err, want)
		}
	}
	if n := e.count(t, `SELECT count(*) FROM roadmap_assessments WHERE interview_id=$1`, interview); n != 1 {
		t.Fatalf("assessments = %d, want 1", n)
	}

	secondQuestion := e.question(t, profile.JDID, items[0].ID)
	e.exec(t, `INSERT INTO interview_questions(interview_id,position,core_question_id) VALUES($1,2,$2)`, interview, secondQuestion)
	for name, call := range map[string]func() (bool, error){
		"a stranger":                  func() (bool, error) { return e.roadmaps.RecordAssessment(e.ctx, stranger, interview, items[0].ID, 2) },
		"another topic than the bank": func() (bool, error) { return e.roadmaps.RecordAssessment(e.ctx, owner, interview, items[1].ID, 2) },
		"an unassigned position":      func() (bool, error) { return e.roadmaps.RecordAssessment(e.ctx, owner, interview, items[0].ID, 9) },
		"an unknown interview":        func() (bool, error) { return e.roadmaps.RecordAssessment(e.ctx, owner, uuid.New(), items[0].ID, 1) },
	} {
		if written, err := call(); err != nil || written {
			t.Errorf("%s: written=%v err=%v, want nothing written", name, written, err)
		}
	}
}

func TestDeletingAProfileWithRoadmapRowsStillWorks(t *testing.T) {
	e := newEnv(t)
	owner := e.user(t, "candidate")
	profile := e.profile(t, owner)
	roadmap := e.roadmap(t, owner, profile.ID)
	items := e.publish(t, roadmap.ID, "SQL", "Go")
	e.question(t, profile.JDID, items[0].ID)

	if err := e.profiles.DeleteProfile(e.ctx, owner, profile.ID); err != nil {
		t.Fatal("delete profile", err)
	}
	if n := e.count(t, `SELECT (SELECT count(*) FROM roadmaps WHERE id=$1)+(SELECT count(*) FROM roadmap_items WHERE roadmap_id=$1)`, roadmap.ID); n != 0 {
		t.Fatalf("%d roadmap rows survived the profile", n)
	}
}

func TestDeletingAProfileWithInterviewsIsStillRefused(t *testing.T) {
	e := newEnv(t)
	owner := e.user(t, "candidate")
	profile := e.profile(t, owner)
	items := e.publish(t, e.roadmap(t, owner, profile.ID).ID, "SQL")
	e.interview(t, profile.ID, e.question(t, profile.JDID, items[0].ID), items[0].ID)
	if err := e.profiles.DeleteProfile(e.ctx, owner, profile.ID); !apperror.IsCode(err, apperror.CodeJDInUse) {
		t.Fatalf("delete = %v, want JD_IN_USE", err)
	}
}

func TestAccountErasureRemovesRoadmapRowsWithoutOrphans(t *testing.T) {
	e := newEnv(t)
	owner, other := e.user(t, "candidate"), e.user(t, "candidate")
	profile := e.profile(t, owner)
	items := e.publish(t, e.roadmap(t, owner, profile.ID).ID, "SQL", "Go")
	question := e.question(t, profile.JDID, items[0].ID)
	interview := e.interview(t, profile.ID, question, items[0].ID)
	if written, err := e.roadmaps.RecordAssessment(e.ctx, owner, interview, items[0].ID, 1); err != nil || !written {
		t.Fatal("record assessment", written, err)
	}
	// Another account's Roadmap must be untouched.
	otherProfile := e.profile(t, other)
	e.publish(t, e.roadmap(t, other, otherProfile.ID).ID, "Kafka")

	if err := accountrepo.NewRepository(e.pool).Erase(e.ctx, owner); err != nil {
		t.Fatal("erase account", err)
	}
	for name, probe := range map[string]struct {
		sql string
		arg uuid.UUID
	}{
		"roadmaps":    {`SELECT count(*) FROM roadmaps WHERE role_profile_id=$1`, profile.ID},
		"items":       {`SELECT count(*) FROM roadmap_items WHERE roadmap_id IN (SELECT id FROM roadmaps WHERE role_profile_id=$1)`, profile.ID},
		"assessments": {`SELECT count(*) FROM roadmap_assessments WHERE interview_id=$1`, interview},
		"interviews":  {`SELECT count(*) FROM interviews WHERE id=$1`, interview},
	} {
		if n := e.count(t, probe.sql, probe.arg); n != 0 {
			t.Errorf("%s left: %d", name, n)
		}
	}
	if n := e.count(t, `SELECT count(*) FROM roadmap_items WHERE roadmap_id IN (SELECT id FROM roadmaps WHERE role_profile_id=$1)`, otherProfile.ID); n != 1 {
		t.Fatalf("the other account's topics = %d, want 1", n)
	}
	if n := e.count(t, `SELECT count(*) FROM roadmap_items ri WHERE NOT EXISTS (SELECT 1 FROM roadmaps r WHERE r.id=ri.roadmap_id)`); n != 0 {
		t.Fatalf("%d orphan topics", n)
	}
}
