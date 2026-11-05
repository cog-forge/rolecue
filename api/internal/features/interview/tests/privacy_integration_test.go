//go:build integration

package tests

import (
	"testing"
	"time"

	"github.com/cog-forge/rolecue/api/internal/features/interview/domain"
	"github.com/cog-forge/rolecue/api/internal/features/interview/repository"
	"github.com/cog-forge/rolecue/api/internal/features/interview/service"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

// KAN-296: each successful read writes exactly one audit row; a refused read writes none; only an Admin
// lists the audit.
func TestRecruiterReadsAreAuditedOncePerSuccessfulRead(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newService(t, pool, 3)
	store := newFakeRecordingStore()
	recording := newRecordingService(t, pool, store)
	fx := newRecordingFixture(t, ctx, s, svc)
	other := s.user("recruiter")
	rows := func(action string) int {
		return s.count(`SELECT count(*) FROM public.recruiter_access_audit WHERE recruiter_id = $1 AND application_id = $2 AND interview_id = $3 AND action = $4`,
			fx.recruiter, fx.application, fx.session.ID, action)
	}

	// Before the recording is ready the read has no link: no row.
	if _, err := recording.Recording(ctx, fx.recruiter, "recruiter", fx.posting, fx.application); err != nil {
		t.Fatal(err)
	}
	if rows("recording_view") != 0 {
		t.Fatal("a status read without a link wrote an audit row")
	}

	if _, err := recording.GrantUpload(ctx, fx.candidate, "candidate", fx.session.ID, domain.RecordingUploadInput{ContentType: "video/webm", Size: 1024}); err != nil {
		t.Fatal(err)
	}
	store.putObject(store.lastKey(t), 1024)
	if _, err := recording.CompleteUpload(ctx, fx.candidate, "candidate", fx.session.ID); err != nil {
		t.Fatal(err)
	}
	for i := 1; i <= 2; i++ {
		if _, err := recording.Recording(ctx, fx.recruiter, "recruiter", fx.posting, fx.application); err != nil {
			t.Fatal(err)
		}
		if got := rows("recording_view"); got != i {
			t.Fatalf("after %d successful reads: %d audit rows", i, got)
		}
	}

	// Refused reads write nothing.
	if _, err := recording.Recording(ctx, other, "recruiter", fx.posting, fx.application); codeOf(err) != apperror.CodeApplicationNotFound {
		t.Fatalf("foreign recruiter: %v", err)
	}
	if _, err := recording.Transcript(ctx, fx.recruiter, "recruiter", fx.posting, fx.application); codeOf(err) != apperror.CodeInterviewNotCompleted {
		t.Fatalf("open interview: %v", err)
	}
	if s.count(`SELECT count(*) FROM public.recruiter_access_audit WHERE application_id = $1`, fx.application) != 2 {
		t.Fatal("a refused read wrote an audit row")
	}

	for position := int32(1); position <= 3; position++ {
		if _, err := svc.AppendTurn(ctx, fx.candidate, "candidate", fx.session.ID, position, "answer"); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := svc.Complete(ctx, fx.candidate, "candidate", fx.session.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := recording.Transcript(ctx, fx.recruiter, "recruiter", fx.posting, fx.application); err != nil {
		t.Fatal(err)
	}
	if rows("transcript_view") != 1 {
		t.Fatalf("transcript audit rows = %d, want 1", rows("transcript_view"))
	}

	// Admin list: filterable, newest first; everyone else is FORBIDDEN.
	page, err := recording.AccessAudit(ctx, "admin", domain.AccessAuditFilter{ApplicationID: &fx.application, Page: 1, PageSize: 10})
	if err != nil || page.Total != 3 || len(page.Items) != 3 || page.Items[0].Action != "transcript_view" {
		t.Fatalf("admin list = %+v err=%v", page, err)
	}
	page, err = recording.AccessAudit(ctx, "admin", domain.AccessAuditFilter{RecruiterID: &other, Page: 1, PageSize: 10})
	if err != nil || page.Total != 0 || len(page.Items) != 0 {
		t.Fatalf("filter by another recruiter = %+v err=%v", page, err)
	}
	if _, err := recording.AccessAudit(ctx, "recruiter", domain.AccessAuditFilter{}); codeOf(err) != apperror.CodeForbidden {
		t.Fatalf("recruiter listing the audit: %v", err)
	}
}

// KAN-297: the consent version and time are stored with the grant; with the gate on, no consent means no
// grant and no row change.
func TestRecordingConsentIsStoredAndEnforced(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newService(t, pool, 3)
	recording := newRecordingService(t, pool, newFakeRecordingStore())
	fx := newRecordingFixture(t, ctx, s, svc)
	consent := func() (version *string, at *time.Time) {
		if err := pool.QueryRow(ctx, `SELECT recording_consent_version, recording_consented_at FROM public.interviews WHERE id = $1`,
			fx.session.ID).Scan(&version, &at); err != nil {
			t.Fatal(err)
		}
		return version, at
	}
	input := domain.RecordingUploadInput{ContentType: "video/webm", Size: 1024}

	// Gate off: a grant without consent works and stores none.
	if _, err := recording.GrantUpload(ctx, fx.candidate, "candidate", fx.session.ID, input); err != nil {
		t.Fatal(err)
	}
	if version, at := consent(); version != nil || at != nil {
		t.Fatalf("no consent was sent, stored %v %v", version, at)
	}

	// Gate on, no consent stored: refused, and the grant count does not move.
	recording.SetConsentRequired(true)
	grants := s.count(`SELECT record_grant_count FROM public.interviews WHERE id = $1`, fx.session.ID)
	if _, err := recording.GrantUpload(ctx, fx.candidate, "candidate", fx.session.ID, input); codeOf(err) != apperror.CodeRecordingConsentRequired {
		t.Fatalf("gate on without consent: %v", err)
	}
	if s.count(`SELECT record_grant_count FROM public.interviews WHERE id = $1`, fx.session.ID) != grants {
		t.Fatal("a refused grant was counted")
	}

	// Gate on, consent sent: granted and stored with a time.
	input.ConsentVersion = "2026-11-v1"
	if _, err := recording.GrantUpload(ctx, fx.candidate, "candidate", fx.session.ID, input); err != nil {
		t.Fatal(err)
	}
	if version, at := consent(); version == nil || *version != "2026-11-v1" || at == nil || time.Since(*at) > time.Minute {
		t.Fatalf("stored consent = %v %v", version, at)
	}

	// Consent already stored: a retry without it keeps it and is granted.
	input.ConsentVersion = ""
	if _, err := recording.GrantUpload(ctx, fx.candidate, "candidate", fx.session.ID, input); err != nil {
		t.Fatalf("retry with stored consent: %v", err)
	}
	if version, _ := consent(); version == nil || *version != "2026-11-v1" {
		t.Fatalf("a grant without consent erased the stored one: %v", version)
	}
}

// KAN-281: CV retention uses the real queries. A CV of an Application rejected at the CV stage is deleted
// after the period and its key cleared; younger ones, pending ones and rejected ones that reached an
// interview keep theirs. A storage failure keeps the key, the next run deletes it.
func TestCVRetentionBoundariesAndRetry(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc, _ := lifecycleService(t, pool, 2)
	_, posting, _ := s.openPosting(slotOf(5), 2)
	reject := func(key, ago, status string) uuid.UUID {
		id := s.application(posting, s.user("candidate"), "pending", nil)
		s.exec(`UPDATE public.applications SET cv_path = $2, status = $3::public.application_status, updated_at = now() - $4::interval WHERE id = $1`,
			id, key, status, ago)
		return id
	}
	oldApp := reject("cv/old", "181 days", "rejected")
	youngApp := reject("cv/young", "179 days", "rejected")
	pendingApp := reject("cv/pending", "400 days", "pending")
	flakyApp := reject("cv/flaky", "181 days", "rejected")
	_, interviewedApp := s.endedRecruitment(svc, posting, "none", nil, time.Hour)
	s.exec(`UPDATE public.applications SET cv_path = 'cv/interviewed', status = 'rejected', updated_at = now() - interval '400 days' WHERE id = $1`, interviewedApp)

	store := &upkeepStore{failing: map[string]bool{"cv/flaky": true}}
	upkeep, err := service.NewRecordingUpkeep(repository.NewRepository(pool), store, 5*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	upkeep.WithRetention(repository.NewRepository(pool), service.RetentionPolicy{CVDays: 180, TranscriptDays: 365})

	if _, err := upkeep.PurgeExpiredCVs(ctx, s.dbNow()); err == nil {
		t.Fatal("the failed delete must be reported")
	}
	var path *string
	for id, wantCleared := range map[uuid.UUID]bool{oldApp: true, youngApp: false, pendingApp: false, flakyApp: false, interviewedApp: false} {
		if err := pool.QueryRow(ctx, `SELECT cv_path FROM public.applications WHERE id = $1`, id).Scan(&path); err != nil {
			t.Fatal(err)
		}
		if (path == nil) != wantCleared {
			t.Fatalf("application %s: cv_path cleared = %v, want %v", id, path == nil, wantCleared)
		}
	}
	// The decision time is not moved by the clearing.
	if s.count(`SELECT count(*) FROM public.applications WHERE id = $1 AND updated_at < now() - interval '180 days'`, oldApp) != 1 {
		t.Fatal("clearing the CV moved updated_at")
	}

	if purged, err := upkeep.PurgeExpiredCVs(ctx, s.dbNow()); err != nil || purged != 1 {
		t.Fatalf("retry purged=%d err=%v", purged, err)
	}
	if purged, err := upkeep.PurgeExpiredCVs(ctx, s.dbNow()); err != nil || purged != 0 {
		t.Fatalf("a repeated run purged=%d err=%v", purged, err)
	}
}

// KAN-281: transcript retention clears answers and feedback of interviews completed before the period,
// keeps the questions and the scores, and a repeated run changes nothing.
func TestTranscriptRetentionKeepsQuestionsAndScores(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc, _ := lifecycleService(t, pool, 2)
	_, posting, _ := s.openPosting(slotOf(5), 2)
	old, _ := s.endedRecruitment(svc, posting, "none", nil, 366*24*time.Hour)
	young, _ := s.endedRecruitment(svc, posting, "none", nil, 364*24*time.Hour)
	for _, id := range []uuid.UUID{old, young} {
		s.exec(`UPDATE public.interviews SET score = 77 WHERE id = $1`, id)
		s.exec(`UPDATE public.conversation_turns SET feedback = 'good' WHERE interview_id = $1`, id)
	}
	upkeep, err := service.NewRecordingUpkeep(repository.NewRepository(pool), &upkeepStore{}, 5*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	upkeep.WithRetention(repository.NewRepository(pool), service.RetentionPolicy{CVDays: 180, TranscriptDays: 365})

	answered := func(id uuid.UUID) int {
		return s.count(`SELECT count(*) FROM public.conversation_turns WHERE interview_id = $1 AND (answer IS NOT NULL OR feedback IS NOT NULL)`, id)
	}
	if answered(old) == 0 || answered(young) == 0 {
		t.Fatal("the seed has no answered turns")
	}
	cleared, err := upkeep.ClearExpiredTranscripts(ctx, s.dbNow())
	if err != nil || cleared == 0 {
		t.Fatalf("cleared=%d err=%v", cleared, err)
	}
	if answered(old) != 0 || answered(young) == 0 {
		t.Fatalf("old answered=%d young answered=%d", answered(old), answered(young))
	}
	if s.count(`SELECT count(*) FROM public.conversation_turns WHERE interview_id = $1 AND question <> ''`, old) == 0 ||
		s.count(`SELECT score FROM public.interviews WHERE id = $1`, old) != 77 {
		t.Fatal("the questions and the score must stay")
	}
	if cleared, err := upkeep.ClearExpiredTranscripts(ctx, s.dbNow()); err != nil || cleared != 0 {
		t.Fatalf("a repeated run cleared=%d err=%v", cleared, err)
	}
}
