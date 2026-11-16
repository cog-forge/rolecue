//go:build integration

package tests

import (
	"testing"
	"time"

	"github.com/cog-forge/rolecue/api/internal/features/application/repository"

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
	s.later(`DELETE FROM avatar_assets s WHERE s.model_path LIKE 'private/%' AND s.created_by IS NULL
		AND NOT EXISTS (SELECT 1 FROM avatars a WHERE a.asset_id = s.id)`)
	s.exec(`WITH s AS (INSERT INTO avatar_assets (model_path) VALUES ('private/posting.vrm'), ('private/personal.vrm'), ('private/foreign.vrm') RETURNING id, model_path)
		INSERT INTO avatars (id,inventory_id,name,asset_id,personality)
		SELECT v.id, v.inv, v.name, s.id, 'p' FROM (VALUES
			($1::uuid,$4::uuid,'Posting avatar','private/posting.vrm'),
			($2::uuid,$4::uuid,'Personal avatar','private/personal.vrm'),
			($3::uuid,$5::uuid,'Foreign avatar','private/foreign.vrm')) AS v(id,inv,name,path) JOIN s ON s.model_path = v.path`,
		lockedAvatar, personalAvatar, foreignAvatar, ownInventory, strangerInventory)
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

// TestCompanyInterviewerReadsTheModelThroughTheAsset proves the posting's avatar resolves its model from
// avatars.asset_id alone: the asset was created by another account and has no listing, exactly like a copy
// the Recruiter received, and the stored key never reaches the public result.
func TestCompanyInterviewerReadsTheModelThroughTheAsset(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	owner, candidate, creator := s.user("recruiter"), s.user("candidate"), s.user("recruiter")
	inventoryID, voiceID, assetID, avatarID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	key := "private/shared-" + uuid.NewString() + ".vrm"
	s.exec(`INSERT INTO avatar_assets (id,created_by,model_path) VALUES ($1,$2,$3)`, assetID, creator, key)
	s.later(`DELETE FROM avatar_assets WHERE id=$1`, assetID)
	s.exec(`INSERT INTO inventories (id,user_id,slot) VALUES ($1,$2,3)`, inventoryID, owner)
	s.later(`DELETE FROM inventories WHERE id=$1`, inventoryID)
	s.exec(`INSERT INTO avatars (id,inventory_id,name,asset_id,personality) VALUES ($1,$2,'Granted interviewer',$3,'p')`, avatarID, inventoryID, assetID)
	s.later(`DELETE FROM avatars WHERE id=$1`, avatarID)
	s.exec(`INSERT INTO voices (id,name) VALUES ($1,'Company voice')`, voiceID)
	s.later(`DELETE FROM voices WHERE id=$1`, voiceID)
	postingID := s.posting(owner, "open", slot(3))
	s.exec(`UPDATE job_postings SET required_avatar_id=$2, required_voice_id=$3 WHERE id=$1`, postingID, avatarID, voiceID)
	appID := s.application(postingID, candidate)
	s.exec(`UPDATE applications SET status='interview_eligible', interview_deadline=$2 WHERE id=$1`, appID, time.Now().Add(time.Hour))

	record, err := repository.NewRepository(pool).CompanyInterviewer(ctx, candidate, appID, uuid.Nil)
	if err != nil {
		t.Fatal(err)
	}
	if record.Name != "Granted interviewer" || record.ModelPath != key {
		t.Fatalf("record = %+v, want the granted avatar with the asset's key", record)
	}
	// Another creator, no listing, and then no creator at all: the reader needs inventory ownership and the asset only.
	if _, err := pool.Exec(ctx, `UPDATE avatar_assets SET created_by = NULL WHERE id = $1`, assetID); err != nil {
		t.Fatal(err)
	}
	if record, err = repository.NewRepository(pool).CompanyInterviewer(ctx, candidate, appID, uuid.Nil); err != nil || record.ModelPath != key {
		t.Fatalf("after the creator vanished: %+v, %v", record, err)
	}
}
