//go:build integration

package tests

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/cog-forge/rolecue/api/internal/features/interview/domain"
	"github.com/cog-forge/rolecue/api/internal/features/interview/repository"
	"github.com/cog-forge/rolecue/api/internal/features/interview/service"
	"github.com/cog-forge/rolecue/api/internal/pkg/storage"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Session lifecycle (KAN-215 leave, KAN-180 abandonment, KAN-272 recording upkeep) against the real
// database. The abandonment and recording jobs read the whole database the way the System Handler
// does; the assertions only look at the rows each test seeded.

const abandonTimeout = 15 * time.Minute

func lifecycleService(t *testing.T, pool *pgxpool.Pool, questionLimit int32) (*service.Service, *countingEvaluator) {
	t.Helper()
	svc, err := service.New(repository.NewRepository(pool), service.Settings{QuestionLimit: questionLimit, AbandonTimeout: abandonTimeout})
	if err != nil {
		t.Fatal(err)
	}
	evaluator := newCountingEvaluator()
	svc.SetEvaluator(evaluator)
	return svc, evaluator
}

// dbNow is the database clock, the clock every stored timestamp was written with.
func (s *seed) dbNow() time.Time {
	s.t.Helper()
	var now time.Time
	if err := s.pool.QueryRow(s.ctx, `SELECT now()`).Scan(&now); err != nil {
		s.t.Fatal(err)
	}
	return now
}

// ended reads how a session ended.
func (s *seed) ended(interviewID uuid.UUID) (completedAt *time.Time, reason *string) {
	s.t.Helper()
	if err := s.pool.QueryRow(s.ctx, `SELECT completed_at, end_reason FROM public.interviews WHERE id = $1`, interviewID).
		Scan(&completedAt, &reason); err != nil {
		s.t.Fatal(err)
	}
	return completedAt, reason
}

// idleSince moves a session's whole history back: the start, every delivery and every answer happened
// `ago` before the database clock.
func (s *seed) idleSince(interviewID uuid.UUID, ago time.Duration) {
	s.t.Helper()
	s.exec(`UPDATE public.interviews SET started_at = now() - $2::interval WHERE id = $1`, interviewID, ago)
	s.exec(`UPDATE public.interview_questions SET asked_at = now() - $2::interval WHERE interview_id = $1 AND asked_at IS NOT NULL`, interviewID, ago)
	s.exec(`UPDATE public.conversation_turns SET asked_at = now() - $2::interval, answered_at = now() - $2::interval WHERE interview_id = $1`, interviewID, ago)
}

// abandonAll runs the abandonment pass until nothing is left to end, since other sessions of the
// database may fill the first batches.
func abandonAll(t *testing.T, ctx context.Context, svc *service.Service, now time.Time) {
	t.Helper()
	for range 50 {
		ended, err := svc.AbandonIdle(ctx, now)
		if err != nil {
			t.Fatalf("AbandonIdle: %v", err)
		}
		if ended == 0 {
			return
		}
	}
	t.Fatal("the abandonment pass never settled")
}

// waitEvaluation waits for the evaluation of want. The abandonment pass may end other sessions of the
// database too, so their evaluations are skipped.
func waitEvaluation(t *testing.T, evaluator *countingEvaluator, want uuid.UUID) {
	t.Helper()
	timeout := time.After(5 * time.Second)
	for {
		select {
		case got := <-evaluator.done:
			if got == want {
				return
			}
		case <-timeout:
			t.Fatalf("the evaluation of %s did not start", want)
		}
	}
}

// noMoreEvaluations fails if the evaluation of id starts again.
func noMoreEvaluations(t *testing.T, evaluator *countingEvaluator, id uuid.UUID) {
	t.Helper()
	timeout := time.After(100 * time.Millisecond)
	for {
		select {
		case got := <-evaluator.done:
			if got == id {
				t.Fatalf("a second evaluation started for %s", got)
			}
		case <-timeout:
			return
		}
	}
}

// Leaving after two of four answers ends the session as left: the two answers stay, the other two core
// questions have no turn, the evaluation runs once, and nothing is charged again. A repeat leave changes
// nothing and the session cannot be resumed or answered.
func TestLeaveMidInterviewEndsWithPartialAnswersAndEvaluates(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc, evaluator := lifecycleService(t, pool, 4)
	candidate := s.user("candidate")
	s.wallet(candidate, 20)
	profile, _ := s.confirmedProfile(candidate, 4)
	session, _, err := svc.Create(ctx, candidate, "candidate", practiceInput(profile))
	if err != nil {
		t.Fatal(err)
	}
	for position := int32(1); position <= 2; position++ {
		if _, err := svc.AppendTurn(ctx, candidate, "candidate", session.ID, position, "answer"); err != nil {
			t.Fatalf("answer %d: %v", position, err)
		}
	}

	left, err := svc.Complete(ctx, candidate, "candidate", session.ID)
	if err != nil {
		t.Fatalf("leave: %v", err)
	}
	if left.CompletedAt == nil || left.EndReason == nil || *left.EndReason != domain.EndLeft ||
		left.AnsweredCount != 2 || left.QuestionCount != 4 || left.CurrentQuestion != nil {
		t.Fatalf("after leave = %+v", left)
	}
	if turns := s.count(`SELECT count(*) FROM public.conversation_turns WHERE interview_id = $1`, session.ID); turns != 2 {
		t.Fatalf("%d turns stored, want the 2 answers only", turns)
	}
	waitEvaluation(t, evaluator, session.ID)

	again, err := svc.Complete(ctx, candidate, "candidate", session.ID)
	if err != nil {
		t.Fatalf("double leave: %v", err)
	}
	if !again.CompletedAt.Equal(*left.CompletedAt) || *again.EndReason != domain.EndLeft {
		t.Fatalf("double leave changed the ending: %+v", again)
	}
	noMoreEvaluations(t, evaluator, session.ID)

	_, err = svc.Resume(ctx, candidate, "candidate", session.ID)
	assertCode(t, err, apperror.CodeInterviewCompleted)
	_, err = svc.AppendTurn(ctx, candidate, "candidate", session.ID, 3, "late answer")
	assertCode(t, err, apperror.CodeInterviewCompleted)
	_, err = svc.Deliver(ctx, candidate, "candidate", session.ID)
	assertCode(t, err, apperror.CodeInterviewCompleted)

	if b := s.balance(candidate); b != 15 {
		t.Fatalf("balance = %d, want 15: one charge at the start, no refund and no second charge", b)
	}
	if rows := len(s.coinRows(candidate)); rows != 1 {
		t.Fatalf("%d coin rows, want the start debit only", rows)
	}
	// Starting again from the same role profile is a new paid session, not a resume of the left one.
	next, created, err := svc.Create(ctx, candidate, "candidate", practiceInput(profile))
	if err != nil || !created || next.ID == session.ID {
		t.Fatalf("new start after leaving: created=%v same=%v err=%v", created, next.ID == session.ID, err)
	}
}

func TestAnsweringEveryQuestionEndsCompleted(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc, evaluator := lifecycleService(t, pool, 2)
	candidate := s.user("candidate")
	s.wallet(candidate, 20)
	profile, _ := s.confirmedProfile(candidate, 2)
	session, _, err := svc.Create(ctx, candidate, "candidate", practiceInput(profile))
	if err != nil {
		t.Fatal(err)
	}
	for position := int32(1); position <= 2; position++ {
		if _, err := svc.AppendTurn(ctx, candidate, "candidate", session.ID, position, "answer"); err != nil {
			t.Fatal(err)
		}
	}
	done, err := svc.Complete(ctx, candidate, "candidate", session.ID)
	if err != nil || done.EndReason == nil || *done.EndReason != domain.EndCompleted {
		t.Fatalf("completion = %+v, %v", done, err)
	}
	waitEvaluation(t, evaluator, session.ID)
}

// Two tabs leaving at once, or a leave racing the socket's completion after the last answer, end the
// session once: one completed_at, one reason, one evaluation.
func TestConcurrentEndsEndTheSessionOnce(t *testing.T) {
	for _, tc := range []struct {
		name     string
		answered int32
		want     domain.EndReason
	}{
		{"double leave mid-interview", 1, domain.EndLeft},
		{"leave racing the normal completion", 3, domain.EndCompleted},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, pool := openPool(t)
			s := newSeed(t, ctx, pool)
			svc, evaluator := lifecycleService(t, pool, 3)
			candidate := s.user("candidate")
			s.wallet(candidate, 20)
			profile, _ := s.confirmedProfile(candidate, 3)
			session, _, err := svc.Create(ctx, candidate, "candidate", practiceInput(profile))
			if err != nil {
				t.Fatal(err)
			}
			for position := int32(1); position <= tc.answered; position++ {
				if _, err := svc.AppendTurn(ctx, candidate, "candidate", session.ID, position, "answer"); err != nil {
					t.Fatal(err)
				}
			}
			const callers = 8
			results := make([]domain.Session, callers)
			errs := make([]error, callers)
			var wg sync.WaitGroup
			for i := range callers {
				wg.Add(1)
				go func() {
					defer wg.Done()
					results[i], errs[i] = svc.Complete(ctx, candidate, "candidate", session.ID)
				}()
			}
			wg.Wait()
			for i := range callers {
				if errs[i] != nil {
					t.Fatalf("caller %d: %v", i, errs[i])
				}
				if !results[i].CompletedAt.Equal(*results[0].CompletedAt) || *results[i].EndReason != tc.want {
					t.Fatalf("caller %d saw %+v", i, results[i])
				}
			}
			waitEvaluation(t, evaluator, session.ID)
			noMoreEvaluations(t, evaluator, session.ID)
			if b := s.balance(candidate); b != 15 {
				t.Fatalf("balance = %d, want 15", b)
			}
		})
	}
}

// A recruitment leave moves the Application on like a normal completion and never uses another slot:
// the posting still counts one interview, and a new start returns the ended session.
func TestRecruitmentLeaveMovesTheApplicationAndUsesNoOtherSlot(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc, evaluator := lifecycleService(t, pool, 3)
	candidate := s.user("candidate")
	_, posting, _ := s.openPosting(slotOf(1), 3)
	application := s.eligibleApplication(posting, candidate)
	session, _, err := svc.Create(ctx, candidate, "candidate", recruitmentInput(application))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.AppendTurn(ctx, candidate, "candidate", session.ID, 1, "answer"); err != nil {
		t.Fatal(err)
	}
	left, err := svc.Complete(ctx, candidate, "candidate", session.ID)
	if err != nil || left.EndReason == nil || *left.EndReason != domain.EndLeft {
		t.Fatalf("leave = %+v, %v", left, err)
	}
	waitEvaluation(t, evaluator, session.ID)
	if status := s.applicationStatus(application); status != "interviewed" {
		t.Fatalf("application status = %s, want interviewed", status)
	}
	again, created, err := svc.Create(ctx, candidate, "candidate", recruitmentInput(application))
	if err != nil || created || again.ID != session.ID {
		t.Fatalf("a start after leaving: created=%v same=%v err=%v", created, again.ID == session.ID, err)
	}
	if n := s.interviewsOfPosting(posting); n != 1 {
		t.Fatalf("%d interviews on the posting, want 1 slot used", n)
	}
}

// The abandonment boundary: a session idle 16 minutes is ended as abandoned and evaluated on what was
// answered; one idle 14 minutes is left alone, and resuming it (a reconnect inside the timeout) keeps
// the same session. A repeated pass changes nothing.
func TestAbandonEndsSessionsIdlePastTheTimeoutOnly(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc, evaluator := lifecycleService(t, pool, 3)
	candidate := s.user("candidate")
	s.wallet(candidate, 20)
	staleProfile, _ := s.confirmedProfile(candidate, 3)
	freshProfile, _ := s.confirmedProfile(candidate, 3)
	stale, _, err := svc.Create(ctx, candidate, "candidate", practiceInput(staleProfile))
	if err != nil {
		t.Fatal(err)
	}
	fresh, _, err := svc.Create(ctx, candidate, "candidate", practiceInput(freshProfile))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Deliver(ctx, candidate, "candidate", stale.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.AppendTurn(ctx, candidate, "candidate", stale.ID, 1, "answer"); err != nil {
		t.Fatal(err)
	}
	s.idleSince(stale.ID, 16*time.Minute)
	s.idleSince(fresh.ID, 14*time.Minute)

	abandonAll(t, ctx, svc, s.dbNow())
	completedAt, reason := s.ended(stale.ID)
	if completedAt == nil || reason == nil || *reason != string(domain.EndAbandoned) {
		t.Fatalf("idle 16 min: completed_at=%v reason=%v, want abandoned", completedAt, reason)
	}
	waitEvaluation(t, evaluator, stale.ID)
	if completedAt, _ := s.ended(fresh.ID); completedAt != nil {
		t.Fatal("a session idle 14 minutes must not be abandoned")
	}

	resumed, err := svc.Resume(ctx, candidate, "candidate", fresh.ID)
	if err != nil || resumed.ID != fresh.ID {
		t.Fatalf("reconnect inside the timeout: %v", err)
	}
	if _, err := svc.Deliver(ctx, candidate, "candidate", fresh.ID); err != nil {
		t.Fatalf("deliver after reconnect: %v", err)
	}

	first, _ := s.ended(stale.ID)
	abandonAll(t, ctx, svc, s.dbNow())
	second, _ := s.ended(stale.ID)
	if !first.Equal(*second) {
		t.Fatal("a repeated pass changed the ending")
	}
	noMoreEvaluations(t, evaluator, stale.ID)
	if b := s.balance(candidate); b != 10 {
		t.Fatalf("balance = %d, want 10: two starts, no refund", b)
	}
	_, err = svc.Resume(ctx, candidate, "candidate", stale.ID)
	assertCode(t, err, apperror.CodeInterviewCompleted)
}

// An answer stored after the pass listed the session keeps it alive: End checks the activity again
// under the session lock.
func TestAbandonRechecksActivityUnderTheLock(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc, _ := lifecycleService(t, pool, 3)
	repo := repository.NewRepository(pool)
	candidate := s.user("candidate")
	s.wallet(candidate, 20)
	profile, _ := s.confirmedProfile(candidate, 3)
	session, _, err := svc.Create(ctx, candidate, "candidate", practiceInput(profile))
	if err != nil {
		t.Fatal(err)
	}
	s.idleSince(session.ID, 20*time.Minute)
	idleBefore := s.dbNow().Add(-abandonTimeout)
	// The Candidate answers between the listing and the end.
	if _, err := svc.AppendTurn(ctx, candidate, "candidate", session.ID, 1, "just in time"); err != nil {
		t.Fatal(err)
	}
	result, err := repo.End(ctx, domain.EndParams{CandidateID: candidate, ID: session.ID, Cause: domain.EndBySystem, IdleBefore: &idleBefore})
	if err != nil {
		t.Fatal(err)
	}
	if result.Ended || result.Session.CompletedAt != nil {
		t.Fatalf("an active session was abandoned: %+v", result)
	}
}

// Recruitment sessions are abandoned only while the Application still accepts the interview; after the
// deadline the expiry pass owns the Application, so the session is left as it is.
func TestAbandonRecruitmentOnlyInsideTheWindow(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc, _ := lifecycleService(t, pool, 2)
	openCandidate, closedCandidate := s.user("candidate"), s.user("candidate")
	_, posting, _ := s.openPosting(slotOf(2), 2)
	openApp := s.eligibleApplication(posting, openCandidate)
	closedApp := s.eligibleApplication(posting, closedCandidate)
	openSession, _, err := svc.Create(ctx, openCandidate, "candidate", recruitmentInput(openApp))
	if err != nil {
		t.Fatal(err)
	}
	closedSession, _, err := svc.Create(ctx, closedCandidate, "candidate", recruitmentInput(closedApp))
	if err != nil {
		t.Fatal(err)
	}
	s.idleSince(openSession.ID, 16*time.Minute)
	s.idleSince(closedSession.ID, 16*time.Minute)
	s.setDeadlineInPast(closedApp)

	abandonAll(t, ctx, svc, s.dbNow())
	if _, reason := s.ended(openSession.ID); reason == nil || *reason != string(domain.EndAbandoned) {
		t.Fatalf("open window: reason %v, want abandoned", reason)
	}
	if status := s.applicationStatus(openApp); status != "interviewed" {
		t.Fatalf("open window application = %s, want interviewed", status)
	}
	if completedAt, _ := s.ended(closedSession.ID); completedAt != nil {
		t.Fatal("a session whose window closed must be left to the expiry pass")
	}
	if status := s.applicationStatus(closedApp); status != "interview_eligible" {
		t.Fatalf("closed window application = %s, want it untouched", status)
	}
}

// upkeepStore is the object storage of the purge: it records deletions and fails the keys in failing
// once each.
type upkeepStore struct {
	mu      sync.Mutex
	deleted []string
	failing map[string]bool
}

func (u *upkeepStore) Delete(_ context.Context, key string) error {
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.failing[key] {
		delete(u.failing, key)
		return errors.New("storage unavailable")
	}
	u.deleted = append(u.deleted, key)
	return nil
}

// endedRecruitment starts and ends a recruitment interview of a new candidate, then sets its recording
// and how long ago it completed.
func (s *seed) endedRecruitment(svc *service.Service, posting uuid.UUID, status string, path *string, completedAgo time.Duration) (uuid.UUID, uuid.UUID) {
	s.t.Helper()
	candidate := s.user("candidate")
	application := s.eligibleApplication(posting, candidate)
	session, _, err := svc.Create(s.ctx, candidate, "candidate", recruitmentInput(application))
	if err != nil {
		s.t.Fatal(err)
	}
	if _, err := svc.AppendTurn(s.ctx, candidate, "candidate", session.ID, 1, "answer"); err != nil {
		s.t.Fatal(err)
	}
	if _, err := svc.Complete(s.ctx, candidate, "candidate", session.ID); err != nil {
		s.t.Fatal(err)
	}
	s.exec(`UPDATE public.interviews SET record_status = $2, record_path = $3, completed_at = now() - $4::interval WHERE id = $1`,
		session.ID, status, path, completedAgo)
	return session.ID, application
}

func keyOf(name string) *string {
	key := "recordings/" + name + "-" + uuid.NewString() + ".webm"
	return &key
}

// A recording still none 5 minutes after completion, or still uploading past the 15-minute grant, is
// failed; younger ones and practice interviews are left alone; a second run changes nothing.
func TestStaleRecordingsAreFailedAfterTheirGrace(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc, _ := lifecycleService(t, pool, 2)
	_, posting, _ := s.openPosting(slotOf(5), 2)
	noneOld, _ := s.endedRecruitment(svc, posting, "none", nil, 6*time.Minute)
	noneYoung, _ := s.endedRecruitment(svc, posting, "none", nil, 4*time.Minute)
	uploadingOld, _ := s.endedRecruitment(svc, posting, "uploading", keyOf("old"), 16*time.Minute)
	uploadingYoung, _ := s.endedRecruitment(svc, posting, "uploading", keyOf("young"), 14*time.Minute)
	ready, _ := s.endedRecruitment(svc, posting, "ready", keyOf("ready"), time.Hour)

	candidate := s.user("candidate")
	s.wallet(candidate, 20)
	profile, _ := s.confirmedProfile(candidate, 2)
	practice, _, err := svc.Create(ctx, candidate, "candidate", practiceInput(profile))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Complete(ctx, candidate, "candidate", practice.ID); err != nil {
		t.Fatal(err)
	}
	s.exec(`UPDATE public.interviews SET completed_at = now() - interval '1 hour' WHERE id = $1`, practice.ID)

	upkeep, err := service.NewRecordingUpkeep(repository.NewRepository(pool), &upkeepStore{}, 5*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := upkeep.FailStale(ctx, s.dbNow()); err != nil {
		t.Fatal(err)
	}
	for id, want := range map[uuid.UUID]string{
		noneOld: "failed", noneYoung: "none", uploadingOld: "failed", uploadingYoung: "uploading", ready: "ready", practice.ID: "none",
	} {
		if status, _ := s.recordingRow(id); status != want {
			t.Errorf("recording of %s = %s, want %s", id, status, want)
		}
	}
	if _, err := upkeep.FailStale(ctx, s.dbNow()); err != nil {
		t.Fatal(err)
	}
	if status, path := s.recordingRow(uploadingOld); status != "failed" || path == nil {
		t.Fatalf("a repeated run changed a failed recording: %s %v", status, path)
	}
}

// Retention: a recording 91 days after the final decision is deleted from storage, its key cleared and
// its status deleted; one 89 days after is kept, as is the recording of an undecided Application however
// old. A storage delete failure keeps the row and the next run deletes it.
func TestRecordingRetentionPurgesNinetyDaysAfterTheFinalDecision(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc, _ := lifecycleService(t, pool, 2)
	_, posting, _ := s.openPosting(slotOf(5), 2)
	oldKey, youngKey, undecidedKey, flakyKey := keyOf("old"), keyOf("young"), keyOf("undecided"), keyOf("flaky")
	old, oldApp := s.endedRecruitment(svc, posting, "ready", oldKey, time.Hour)
	young, youngApp := s.endedRecruitment(svc, posting, "ready", youngKey, time.Hour)
	undecided, undecidedApp := s.endedRecruitment(svc, posting, "ready", undecidedKey, time.Hour)
	flaky, flakyApp := s.endedRecruitment(svc, posting, "failed", flakyKey, time.Hour)
	decide := func(application uuid.UUID, status string, ago string) {
		s.exec(`UPDATE public.applications SET status = $2::public.application_status, updated_at = now() - $3::interval WHERE id = $1`,
			application, status, ago)
		if status == "approved" || status == "rejected" {
			s.exec(`UPDATE public.applications SET decided_at = now() - $2::interval WHERE id = $1`, application, ago)
		}
	}
	decide(oldApp, "approved", "91 days")
	decide(youngApp, "rejected", "89 days")
	decide(undecidedApp, "interviewed", "200 days")
	decide(flakyApp, "rejected", "91 days")

	store := &upkeepStore{failing: map[string]bool{*flakyKey: true}}
	upkeep, err := service.NewRecordingUpkeep(repository.NewRepository(pool), store, 5*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := upkeep.PurgeExpired(ctx, s.dbNow()); err == nil {
		t.Fatal("the failed delete must be reported")
	}
	if status, path := s.recordingRow(old); status != "deleted" || path != nil {
		t.Fatalf("91 days: %s %v, want deleted without a key", status, path)
	}
	if status, path := s.recordingRow(young); status != "ready" || path == nil {
		t.Fatalf("89 days: %s %v, want kept", status, path)
	}
	if status, path := s.recordingRow(undecided); status != "ready" || path == nil {
		t.Fatalf("undecided: %s %v, want kept", status, path)
	}
	if status, path := s.recordingRow(flaky); status != "failed" || path == nil || *path != *flakyKey {
		t.Fatalf("failed delete: %s %v, want the row untouched", status, path)
	}

	if _, err := upkeep.PurgeExpired(ctx, s.dbNow()); err != nil {
		t.Fatalf("retry: %v", err)
	}
	if status, path := s.recordingRow(flaky); status != "deleted" || path != nil {
		t.Fatalf("retry: %s %v, want deleted", status, path)
	}
	deleted := map[string]int{}
	for _, key := range store.deleted {
		deleted[key]++
	}
	if deleted[*oldKey] != 1 || deleted[*flakyKey] != 1 || deleted[*youngKey] != 0 || deleted[*undecidedKey] != 0 {
		t.Fatalf("deleted objects = %v", deleted)
	}
}

// The upkeep uses storage.ErrNotFound for an object already gone; the store in these tests never
// returns it, so the interface stays in sync with storage.Store here.
var _ service.RecordingDeleter = storage.Store(nil)
