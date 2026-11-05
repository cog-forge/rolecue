//go:build integration

package tests

import (
	"testing"

	"github.com/cog-forge/rolecue/api/internal/features/roleprofile/repository"
	"github.com/google/uuid"
)

// KAN-268: every Role Profile write names the owner in SQL, so a call that carries another user's id
// changes nothing even if a caller forgot the ownership read.
func TestRoleProfileWritesAreScopedToTheOwnerInSQL(t *testing.T) {
	env := newRoleProfileEnv(t)
	owner := insertFixtureUser(t, env, "candidate")
	intruder := insertFixtureUser(t, env, "candidate")
	q := repository.New(env.pool)

	var jdID, profileID uuid.UUID
	if err := env.pool.QueryRow(env.ctx, `INSERT INTO job_descriptions(raw_text, refinement_note) VALUES ('scope fixture', 'original note') RETURNING id`).Scan(&jdID); err != nil {
		t.Fatal(err)
	}
	if err := env.pool.QueryRow(env.ctx, `INSERT INTO role_profiles(jd_id, candidate_id, title) VALUES ($1, $2, 'Original title') RETURNING id`, jdID, owner).Scan(&profileID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = env.pool.Exec(env.ctx, `DELETE FROM role_profiles WHERE id = $1`, profileID)
		_, _ = env.pool.Exec(env.ctx, `DELETE FROM job_descriptions WHERE id = $1`, jdID)
	})

	state := func() (title, status, note string, exists bool) {
		t.Helper()
		var n int
		if err := env.pool.QueryRow(env.ctx, `SELECT count(*) FROM role_profiles WHERE id = $1`, profileID).Scan(&n); err != nil {
			t.Fatal(err)
		}
		if n == 0 {
			return "", "", "", false
		}
		if err := env.pool.QueryRow(env.ctx, `SELECT rp.title, rp.status::text, COALESCE(jd.refinement_note, '')
			FROM role_profiles rp JOIN job_descriptions jd ON jd.id = rp.jd_id WHERE rp.id = $1`, profileID).Scan(&title, &status, &note); err != nil {
			t.Fatal(err)
		}
		return title, status, note, true
	}

	note := "intruder note"
	if err := q.SaveProfileJD(env.ctx, repository.SaveProfileJDParams{ID: jdID, CandidateID: intruder, RefinementNote: &note}); err != nil {
		t.Fatal(err)
	}
	if err := q.ClaimProfileAnalysis(env.ctx, repository.ClaimProfileAnalysisParams{ID: profileID, CandidateID: intruder}); err != nil {
		t.Fatal(err)
	}
	if err := q.CompleteProfileReview(env.ctx, repository.CompleteProfileReviewParams{ID: profileID, CandidateID: intruder, Title: "Intruder title"}); err != nil {
		t.Fatal(err)
	}
	if err := q.ConfirmRoleProfile(env.ctx, repository.ConfirmRoleProfileParams{ID: profileID, CandidateID: intruder}); err != nil {
		t.Fatal(err)
	}
	if err := q.DeleteOwnedRoleProfile(env.ctx, repository.DeleteOwnedRoleProfileParams{ID: profileID, CandidateID: intruder}); err != nil {
		t.Fatal(err)
	}
	// The JD is still referenced by the owner's profile, so it survives whoever asks.
	if err := q.DeleteProfileJD(env.ctx, jdID); err != nil {
		t.Fatal(err)
	}
	title, status, savedNote, exists := state()
	if !exists || title != "Original title" || status != "draft" || savedNote != "original note" {
		t.Fatalf("after cross-owner writes: exists=%v title=%q status=%q note=%q, want the original untouched draft", exists, title, status, savedNote)
	}

	// The owner's own calls still work.
	if err := q.ClaimProfileAnalysis(env.ctx, repository.ClaimProfileAnalysisParams{ID: profileID, CandidateID: owner}); err != nil {
		t.Fatal(err)
	}
	if err := q.CompleteProfileReview(env.ctx, repository.CompleteProfileReviewParams{ID: profileID, CandidateID: owner, Title: "Owner title"}); err != nil {
		t.Fatal(err)
	}
	if err := q.ConfirmRoleProfile(env.ctx, repository.ConfirmRoleProfileParams{ID: profileID, CandidateID: owner}); err != nil {
		t.Fatal(err)
	}
	ownerNote := "owner note"
	if err := q.SaveProfileJD(env.ctx, repository.SaveProfileJDParams{ID: jdID, CandidateID: owner, RefinementNote: &ownerNote}); err != nil {
		t.Fatal(err)
	}
	title, status, savedNote, _ = state()
	if title != "Owner title" || status != "confirmed" || savedNote != "owner note" {
		t.Fatalf("after owner writes: title=%q status=%q note=%q", title, status, savedNote)
	}
	if err := q.DeleteOwnedRoleProfile(env.ctx, repository.DeleteOwnedRoleProfileParams{ID: profileID, CandidateID: owner}); err != nil {
		t.Fatal(err)
	}
	if _, _, _, exists := state(); exists {
		t.Fatal("the owner could not delete the profile")
	}
}
