//go:build integration

package tests

import (
	"testing"
	"time"

	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

// KAN-305: the Candidate is only ever given the avatar the posting locked, never another avatar of the
// Recruiter, and not an avatar that belongs to someone else.
func TestCompanyInterviewerIsLimitedToThePostingsAvatar(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	owner, candidate, stranger := s.user("recruiter"), s.user("candidate"), s.user("recruiter")
	ownInventory, strangerInventory, voiceID := uuid.New(), uuid.New(), uuid.New()
	lockedAvatar, personalAvatar, foreignAvatar := uuid.New(), uuid.New(), uuid.New()
	s.exec(`INSERT INTO inventories (id,user_id,slot) VALUES ($1,$2,3),($3,$4,3)`, ownInventory, owner, strangerInventory, stranger)
	s.later(`DELETE FROM inventories WHERE id IN ($1,$2)`, ownInventory, strangerInventory)
	s.exec(`INSERT INTO avatars (id,inventory_id,name,model_path,personality) VALUES
		($1,$4,'Posting avatar','private/posting.vrm','p'),
		($2,$4,'Personal avatar','private/personal.vrm','p'),
		($3,$5,'Foreign avatar','private/foreign.vrm','p')`, lockedAvatar, personalAvatar, foreignAvatar, ownInventory, strangerInventory)
	s.later(`DELETE FROM avatars WHERE id IN ($1,$2,$3)`, lockedAvatar, personalAvatar, foreignAvatar)
	s.exec(`INSERT INTO voices (id,name) VALUES ($1,'Company voice')`, voiceID)
	s.later(`DELETE FROM voices WHERE id=$1`, voiceID)
	postingID := s.posting(owner, "open", slot(3))
	appID := s.application(postingID, candidate)
	s.exec(`UPDATE applications SET status='interview_eligible', interview_deadline=$2 WHERE id=$1`, appID, time.Now().Add(time.Hour))
	svc := newApplyService(t, pool, &fakeCVStore{})

	lock := func(avatar uuid.UUID) {
		s.exec(`UPDATE job_postings SET required_avatar_id=$2, required_voice_id=$3 WHERE id=$1`, postingID, avatar, voiceID)
	}
	lock(lockedAvatar)
	got, err := svc.CompanyInterviewer(ctx, candidate, "candidate", appID, uuid.Nil)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "Posting avatar" {
		t.Fatalf("interviewer = %q, want the posting's avatar", got.Name)
	}

	lock(foreignAvatar)
	_, err = svc.CompanyInterviewer(ctx, candidate, "candidate", appID, uuid.Nil)
	assertCode(t, err, apperror.CodeApplicationNotFound)

	s.exec(`UPDATE job_postings SET required_avatar_id=NULL WHERE id=$1`, postingID)
	_, err = svc.CompanyInterviewer(ctx, candidate, "candidate", appID, uuid.Nil)
	assertCode(t, err, apperror.CodeApplicationNotFound)
}
