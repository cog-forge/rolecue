//go:build integration

package tests

import (
	"fmt"
	"slices"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/cog-forge/rolecue/api/internal/features/interview/domain"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

// roadmapOf gives a seeded profile a ready Roadmap whose topics each own the given number of bank questions
// (added to the profile's job description, already holding `existing` unmapped questions). It returns the topic
// ids and, per topic, the question ids.
func (s *seed) roadmapOf(profile uuid.UUID, status string, perTopic ...int) ([]uuid.UUID, [][]uuid.UUID) {
	s.t.Helper()
	var roadmap, jd uuid.UUID
	s.exec(`INSERT INTO public.roadmaps (role_profile_id, generation_status) VALUES ($1, $2)`, profile, status)
	if err := s.pool.QueryRow(s.ctx, `SELECT r.id, rp.jd_id FROM public.roadmaps r JOIN public.role_profiles rp ON rp.id = r.role_profile_id WHERE rp.id = $1`, profile).Scan(&roadmap, &jd); err != nil {
		s.t.Fatal(err)
	}
	topics := make([]uuid.UUID, len(perTopic))
	questions := make([][]uuid.UUID, len(perTopic))
	for i, n := range perTopic {
		topics[i] = uuid.New()
		s.exec(`INSERT INTO public.roadmap_items (id, roadmap_id, title, position) VALUES ($1, $2, $3, $4)`, topics[i], roadmap, fmt.Sprintf("Topic %d", i), i)
		for j := 0; j < n; j++ {
			id := uuid.New()
			s.exec(`INSERT INTO public.core_questions (id, jd_id, content, roadmap_item_id) VALUES ($1, $2, $3, $4)`, id, jd, fmt.Sprintf("topic %d question %s", i, id), topics[i])
			questions[i] = append(questions[i], id)
		}
	}
	return topics, questions
}

// evaluated records a finished, successfully evaluated practice session of the profile in which each listed
// question was answered with the given score, and links every answer to its topic (what RM-09 does).
func (s *seed) evaluated(profile uuid.UUID, topic uuid.UUID, scores map[uuid.UUID]float64) uuid.UUID {
	s.t.Helper()
	interview := uuid.New()
	s.exec(`INSERT INTO public.interviews (id, role_profile_id, type, completed_at, evaluation_status, end_reason) VALUES ($1, $2, 'practice', now(), 'completed', 'completed')`, interview, profile)
	position := 0
	ids := make([]uuid.UUID, 0, len(scores))
	for id := range scores {
		ids = append(ids, id)
	}
	slices.SortFunc(ids, func(a, b uuid.UUID) int { return slices.Compare(a[:], b[:]) })
	for _, id := range ids {
		position++
		s.exec(`INSERT INTO public.interview_questions (interview_id, position, core_question_id, score) VALUES ($1, $2, $3, $4)`, interview, position, id, scores[id])
		s.exec(`INSERT INTO public.roadmap_assessments (roadmap_item_id, interview_id, question_position) VALUES ($1, $2, $3)`, topic, interview, position)
	}
	return interview
}

func scoresOf(ids []uuid.UUID, score float64) map[uuid.UUID]float64 {
	out := make(map[uuid.UUID]float64, len(ids))
	for _, id := range ids {
		out[id] = score
	}
	return out
}

func inTopic(picked, topic []uuid.UUID) int {
	n := 0
	for _, id := range picked {
		if slices.Contains(topic, id) {
			n++
		}
	}
	return n
}

func focusedInput(profile, topic uuid.UUID) domain.CreateInput {
	return domain.CreateInput{Type: domain.TypePractice, RoleProfileID: &profile, FocusItemID: &topic}
}

func TestGeneralPracticeLeavesAchievedTopicsForLast(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newService(t, pool, 8)
	candidate := s.user("candidate")
	s.wallet(candidate, 100)
	profile, _ := s.confirmedProfile(candidate, 0)
	topics, questions := s.roadmapOf(profile, "ready", 8, 8)
	// Topic 0 is achieved: five different questions answered with 90.
	s.evaluated(profile, topics[0], scoresOf(questions[0][:5], 90))

	for round := 0; round < 6; round++ {
		session, _, err := svc.Create(ctx, candidate, "candidate", practiceInput(profile))
		if err != nil {
			t.Fatal(err)
		}
		picked := s.snapshot(session.ID)
		if n := inTopic(picked, questions[1]); n != 8 {
			t.Fatalf("round %d: %d of 8 questions come from the topic that is not achieved, want all 8", round, n)
		}
		s.exec(`UPDATE public.interviews SET completed_at = now() WHERE id = $1`, session.ID) // finish it so the next start is a new one
	}
}

func TestGeneralPracticePrefersQuestionsNeverEvaluated(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newService(t, pool, 8)
	candidate := s.user("candidate")
	s.wallet(candidate, 100)
	profile, _ := s.confirmedProfile(candidate, 0)
	topics, questions := s.roadmapOf(profile, "ready", 12)
	seen := questions[0][:4]
	s.evaluated(profile, topics[0], scoresOf(seen, 50))

	session, _, err := svc.Create(ctx, candidate, "candidate", practiceInput(profile))
	if err != nil {
		t.Fatal(err)
	}
	if n := inTopic(s.snapshot(session.ID), seen); n != 0 {
		t.Fatalf("%d repeated questions were asked while 8 never evaluated ones were available", n)
	}
}

func TestGeneralPracticeOfALegacyBankWithoutARoadmapStillStarts(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newService(t, pool, 3)
	candidate := s.user("candidate")
	s.wallet(candidate, 20)
	profile, bank := s.confirmedProfile(candidate, 5)
	session, created, err := svc.Create(ctx, candidate, "candidate", practiceInput(profile))
	if err != nil || !created || session.FocusItemID != nil {
		t.Fatalf("created=%v focus=%v err=%v", created, session.FocusItemID, err)
	}
	assertSampleOf(t, s.snapshot(session.ID), bank)
}

func TestFocusedPracticeAsksOnlyTheTopicAndStoresIt(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newService(t, pool, 8)
	candidate := s.user("candidate")
	s.wallet(candidate, 100)
	profile, legacy := s.confirmedProfile(candidate, 0)
	_ = legacy
	topics, questions := s.roadmapOf(profile, "ready", 10, 10)
	// A bank written before the Roadmap has questions without a topic: they are never used for a focus either.
	s.coreQuestions(s.jdOf(profile), 6)

	for round := 0; round < 4; round++ {
		session, created, err := svc.Create(ctx, candidate, "candidate", focusedInput(profile, topics[1]))
		if err != nil || !created {
			t.Fatalf("created=%v err=%v", created, err)
		}
		if session.FocusItemID == nil || *session.FocusItemID != topics[1] || session.Resumed == nil || *session.Resumed {
			t.Fatalf("focus=%v resumed=%v, want the topic and a new session", session.FocusItemID, session.Resumed)
		}
		if n := inTopic(s.snapshot(session.ID), questions[1]); n != 8 {
			t.Fatalf("round %d: %d of 8 questions are of the focused topic, want 8", round, n)
		}
		s.exec(`UPDATE public.interviews SET completed_at = now() WHERE id = $1`, session.ID)
	}
	if rows := s.coinRows(candidate); len(rows) != 4 {
		t.Fatalf("coin rows = %d, want one per session", len(rows))
	}
}

func (s *seed) jdOf(profile uuid.UUID) uuid.UUID {
	s.t.Helper()
	var jd uuid.UUID
	if err := s.pool.QueryRow(s.ctx, `SELECT jd_id FROM public.role_profiles WHERE id = $1`, profile).Scan(&jd); err != nil {
		s.t.Fatal(err)
	}
	return jd
}

func TestFocusedPracticePrefersTheTopicsUnevaluatedQuestions(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newService(t, pool, 8)
	candidate := s.user("candidate")
	s.wallet(candidate, 100)
	profile, _ := s.confirmedProfile(candidate, 0)
	topics, questions := s.roadmapOf(profile, "ready", 12)
	s.evaluated(profile, topics[0], scoresOf(questions[0][:4], 40))
	session, _, err := svc.Create(ctx, candidate, "candidate", focusedInput(profile, topics[0]))
	if err != nil {
		t.Fatal(err)
	}
	if n := inTopic(s.snapshot(session.ID), questions[0][:4]); n != 0 {
		t.Fatalf("%d repeated questions asked while 8 unevaluated ones exist", n)
	}
}

func TestFocusedPracticeNeverBorrowsFromOtherTopicsNorChargesWhenTheTopicIsTooSmall(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newService(t, pool, 8)
	candidate := s.user("candidate")
	s.wallet(candidate, 20)
	profile, _ := s.confirmedProfile(candidate, 0)
	topics, _ := s.roadmapOf(profile, "ready", 6, 30) // no top-up is wired in this test: 6 < 8 is refused

	_, _, err := svc.Create(ctx, candidate, "candidate", focusedInput(profile, topics[0]))
	assertCode(t, err, apperror.CodeQuestionBankNotReady)
	if s.interviewRows(profile) != 0 || s.balance(candidate) != 20 || len(s.coinRows(candidate)) != 0 {
		t.Fatal("a refused focused start left a session or a debit")
	}
}

func TestFocusedPracticeRefusesForeignTopicsAndRoadmapsThatAreNotReady(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newService(t, pool, 8)
	candidate, other := s.user("candidate"), s.user("candidate")
	s.wallet(candidate, 50)
	s.wallet(other, 50)
	mine, _ := s.confirmedProfile(candidate, 0)
	mineTopics, _ := s.roadmapOf(mine, "ready", 10)
	theirs, _ := s.confirmedProfile(other, 0)
	theirTopics, _ := s.roadmapOf(theirs, "ready", 10)
	notReady, _ := s.confirmedProfile(candidate, 0)
	notReadyTopics, _ := s.roadmapOf(notReady, "generating", 10)

	_, _, err := svc.Create(ctx, candidate, "candidate", focusedInput(mine, theirTopics[0]))
	assertCode(t, err, apperror.CodeValidation) // a topic of another profile
	_, _, err = svc.Create(ctx, candidate, "candidate", focusedInput(theirs, theirTopics[0]))
	assertCode(t, err, apperror.CodeJDNotFound) // a profile that is not the Candidate's looks like a missing one
	_, _, err = svc.Create(ctx, candidate, "candidate", focusedInput(notReady, notReadyTopics[0]))
	assertCode(t, err, apperror.CodeRoadmapNotReady)
	_, _, err = svc.Create(ctx, candidate, "candidate", focusedInput(mine, uuid.New()))
	assertCode(t, err, apperror.CodeValidation)
	if s.interviewRows(mine)+s.interviewRows(theirs)+s.interviewRows(notReady) != 0 || s.balance(candidate) != 50 || s.balance(other) != 50 {
		t.Fatal("a refused focused start left a session or a debit")
	}
	if _, _, err = svc.Create(ctx, candidate, "candidate", focusedInput(mine, mineTopics[0])); err != nil {
		t.Fatalf("the valid focus was refused: %v", err)
	}
}

func TestFocusedStartKeyReplayKeepsTheSessionAndAChangedFocusIsAConflict(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newService(t, pool, 8)
	candidate := s.user("candidate")
	s.wallet(candidate, 50)
	profile, _ := s.confirmedProfile(candidate, 0)
	topics, _ := s.roadmapOf(profile, "ready", 10, 10)

	keyed := func(input domain.CreateInput, key string) domain.CreateInput {
		input.IdempotencyKey = key
		return input
	}
	first, created, err := svc.Create(ctx, candidate, "candidate", keyed(focusedInput(profile, topics[0]), "k1"))
	if err != nil || !created {
		t.Fatalf("first: created=%v err=%v", created, err)
	}
	again, created, err := svc.Create(ctx, candidate, "candidate", keyed(focusedInput(profile, topics[0]), "k1"))
	if err != nil || created || again.ID != first.ID || again.Resumed == nil || !*again.Resumed {
		t.Fatalf("replay: id=%v created=%v resumed=%v err=%v, want the same session resumed", again.ID, created, again.Resumed, err)
	}
	_, _, err = svc.Create(ctx, candidate, "candidate", keyed(focusedInput(profile, topics[1]), "k1"))
	assertCode(t, err, apperror.CodeInterviewStartIntentConflict)
	_, _, err = svc.Create(ctx, candidate, "candidate", keyed(practiceInput(profile), "k1"))
	assertCode(t, err, apperror.CodeInterviewStartIntentConflict)
	if s.interviewRows(profile) != 1 || s.balance(candidate) != 45 || len(s.coinRows(candidate)) != 1 {
		t.Fatalf("sessions=%d balance=%d: one session and one charge of 5 expected", s.interviewRows(profile), s.balance(candidate))
	}
	snapshot := s.snapshot(first.ID)
	if len(snapshot) != 8 {
		t.Fatalf("session questions = %d", len(snapshot))
	}
}

func TestAnUnfinishedSessionKeepsItsFocusWhateverIsAskedNext(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newService(t, pool, 8)
	candidate := s.user("candidate")
	s.wallet(candidate, 50)
	profile, _ := s.confirmedProfile(candidate, 0)
	topics, _ := s.roadmapOf(profile, "ready", 10, 10)

	general, created, err := svc.Create(ctx, candidate, "candidate", practiceInput(profile))
	if err != nil || !created || general.FocusItemID != nil {
		t.Fatalf("general: created=%v focus=%v err=%v", created, general.FocusItemID, err)
	}
	before := s.snapshot(general.ID)
	// "Practice Go" while a general session is open: the same session comes back, says it is a resume and
	// carries its real focus (none), and nothing is charged or changed.
	resumed, created, err := svc.Create(ctx, candidate, "candidate", focusedInput(profile, topics[0]))
	if err != nil || created || resumed.ID != general.ID || resumed.FocusItemID != nil || resumed.Resumed == nil || !*resumed.Resumed {
		t.Fatalf("focused over general: id=%v created=%v focus=%v resumed=%v err=%v", resumed.ID, created, resumed.FocusItemID, resumed.Resumed, err)
	}
	if !slices.Equal(s.snapshot(general.ID), before) || s.balance(candidate) != 45 || s.interviewRows(profile) != 1 {
		t.Fatal("the unfinished session or the wallet changed")
	}

	s.exec(`UPDATE public.interviews SET completed_at = now() WHERE id = $1`, general.ID)
	focused, created, err := svc.Create(ctx, candidate, "candidate", focusedInput(profile, topics[0]))
	if err != nil || !created || focused.FocusItemID == nil || *focused.FocusItemID != topics[0] {
		t.Fatalf("focused after finishing: created=%v err=%v", created, err)
	}
	back, created, err := svc.Create(ctx, candidate, "candidate", practiceInput(profile))
	if err != nil || created || back.ID != focused.ID || back.FocusItemID == nil || *back.FocusItemID != topics[0] {
		t.Fatalf("general over focused: id=%v created=%v focus=%v err=%v, want the focused session back", back.ID, created, back.FocusItemID, err)
	}
}

func TestConcurrentFocusedStartsCreateOneSessionAndChargeOnce(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newService(t, pool, 8)
	candidate := s.user("candidate")
	s.wallet(candidate, 50)
	profile, _ := s.confirmedProfile(candidate, 0)
	topics, _ := s.roadmapOf(profile, "ready", 10)

	var created, resumed atomic.Int32
	ids := make(chan uuid.UUID, 8)
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			session, wasCreated, err := svc.Create(ctx, candidate, "candidate", focusedInput(profile, topics[0]))
			if err != nil {
				t.Error(err)
				return
			}
			if wasCreated {
				created.Add(1)
			} else {
				resumed.Add(1)
			}
			ids <- session.ID
		}()
	}
	wg.Wait()
	close(ids)
	var first uuid.UUID
	for id := range ids {
		if first != uuid.Nil && id != first {
			t.Fatalf("two sessions: %s and %s", first, id)
		}
		first = id
	}
	if created.Load() != 1 || resumed.Load() != 7 || s.balance(candidate) != 45 || len(s.coinRows(candidate)) != 1 {
		t.Fatalf("created=%d resumed=%d balance=%d coin rows=%d, want 1, 7, 45, 1", created.Load(), resumed.Load(), s.balance(candidate), len(s.coinRows(candidate)))
	}
}

func TestFocusedStartWithoutEnoughCoinsLeavesNothing(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newService(t, pool, 8)
	candidate := s.user("candidate")
	s.wallet(candidate, 4)
	profile, _ := s.confirmedProfile(candidate, 0)
	topics, _ := s.roadmapOf(profile, "ready", 10)
	_, _, err := svc.Create(ctx, candidate, "candidate", focusedInput(profile, topics[0]))
	assertCode(t, err, apperror.CodeInsufficientBalance)
	if s.interviewRows(profile) != 0 || s.questionRows(profile) != 0 || s.balance(candidate) != 4 || len(s.coinRows(candidate)) != 0 {
		t.Fatal("a failed charge left a session, assignments or a debit")
	}
}

func TestRecruitmentStartRefusesAFocus(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newService(t, pool, 3)
	candidate := s.user("candidate")
	profile, _ := s.confirmedProfile(candidate, 0)
	topics, _ := s.roadmapOf(profile, "ready", 10)
	application := uuid.New()
	_, _, err := svc.Create(ctx, candidate, "candidate", domain.CreateInput{Type: domain.TypeRecruitment, ApplicationID: &application, FocusItemID: &topics[0]})
	assertCode(t, err, apperror.CodeValidation)
}

func TestHistoryAndReadsCarryTheRealFocus(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newService(t, pool, 8)
	candidate := s.user("candidate")
	s.wallet(candidate, 50)
	profile, _ := s.confirmedProfile(candidate, 0)
	topics, _ := s.roadmapOf(profile, "ready", 10)
	session, _, err := svc.Create(ctx, candidate, "candidate", focusedInput(profile, topics[0]))
	if err != nil {
		t.Fatal(err)
	}
	got, err := svc.Get(ctx, candidate, "candidate", session.ID)
	if err != nil || got.FocusItemID == nil || *got.FocusItemID != topics[0] || got.Resumed != nil {
		t.Fatalf("get: focus=%v resumed=%v err=%v, want the stored focus and no resumed flag outside a start", got.FocusItemID, got.Resumed, err)
	}
	var stored uuid.UUID
	if err = pool.QueryRow(ctx, `SELECT focus_item_id FROM public.interviews WHERE id = $1`, session.ID).Scan(&stored); err != nil || stored != topics[0] {
		t.Fatalf("stored focus = %v err=%v", stored, err)
	}
}
