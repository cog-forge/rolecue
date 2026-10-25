//go:build integration

// Package tests holds the Admin interview oversight integration tests. They need a migrated,
// disposable PostgreSQL database and skip when DATABASE_URL is not set:
//
//	cd api && set -a && . ../.env && set +a
//	go test -tags=integration ./internal/features/admin/interviews/tests -count=1
package tests

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/cog-forge/rolecue/api/internal/features/admin/interviews/domain"
	"github.com/cog-forge/rolecue/api/internal/features/admin/interviews/repository"
	"github.com/cog-forge/rolecue/api/internal/features/admin/interviews/service"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func openPool(t *testing.T) (context.Context, *pgxpool.Pool) {
	t.Helper()
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL is not set (load ../.env)")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	t.Cleanup(cancel)
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := pool.Ping(ctx); err != nil {
		t.Skipf("database not reachable: %v", err)
	}
	return ctx, pool
}

// seed inserts rows and deletes exactly those rows afterwards, newest registration first, so a
// child row is always removed before the parent it references.
type seed struct {
	t    *testing.T
	ctx  context.Context
	pool *pgxpool.Pool
	undo []func()
}

func newSeed(t *testing.T, ctx context.Context, pool *pgxpool.Pool) *seed {
	s := &seed{t: t, ctx: ctx, pool: pool}
	t.Cleanup(func() {
		for i := len(s.undo) - 1; i >= 0; i-- {
			s.undo[i]()
		}
	})
	return s
}

func (s *seed) exec(sql string, args ...any) {
	s.t.Helper()
	if _, err := s.pool.Exec(s.ctx, sql, args...); err != nil {
		s.t.Fatalf("seed %q: %v", sql, err)
	}
}

func (s *seed) later(sql string, args ...any) {
	s.undo = append(s.undo, func() {
		if _, err := s.pool.Exec(context.Background(), sql, args...); err != nil {
			s.t.Errorf("cleanup %q: %v", sql, err)
		}
	})
}

func (s *seed) user(role string) uuid.UUID {
	id := uuid.New()
	s.exec(`INSERT INTO public.users (id, name, email, role, email_verified) VALUES ($1, $2, $3, $4, true)`,
		id, "interview "+role, fmt.Sprintf("interview-%s@example.test", id), role)
	s.later(`DELETE FROM public.users WHERE id = $1`, id)
	return id
}

func (s *seed) levelID() uuid.UUID {
	s.t.Helper()
	var id uuid.UUID
	if err := s.pool.QueryRow(s.ctx, `SELECT id FROM public.levels ORDER BY id LIMIT 1`).Scan(&id); err != nil {
		s.t.Fatalf("no seniority level seeded: %v", err)
	}
	return id
}

// jobDescription adds a JD, which both a Role Profile and a Job Posting need (each takes its own).
func (s *seed) jobDescription() uuid.UUID {
	id := uuid.New()
	s.exec(`INSERT INTO public.job_descriptions (id, content_path, level_id) VALUES ($1, $2, $3)`,
		id, "job-descriptions/"+id.String()+".pdf", s.levelID())
	s.later(`DELETE FROM public.job_descriptions WHERE id = $1`, id)
	return id
}

// roleProfile adds a Role Profile owned by the Candidate.
func (s *seed) roleProfile(candidateID uuid.UUID) uuid.UUID {
	jdID := s.jobDescription()
	id := uuid.New()
	s.exec(`INSERT INTO public.role_profiles (id, jd_id, candidate_id, title) VALUES ($1, $2, $3, $4)`,
		id, jdID, candidateID, "role "+id.String())
	s.later(`DELETE FROM public.role_profiles WHERE id = $1`, id)
	return id
}

// coreQuestions adds n core questions under a fresh JD and returns their IDs.
func (s *seed) coreQuestions(n int) []uuid.UUID {
	jdID := s.jobDescription()
	ids := make([]uuid.UUID, 0, n)
	for i := 0; i < n; i++ {
		id := uuid.New()
		s.exec(`INSERT INTO public.core_questions (id, jd_id, content) VALUES ($1, $2, $3)`, id, jdID, "core question")
		s.later(`DELETE FROM public.core_questions WHERE id = $1`, id)
		ids = append(ids, id)
	}
	return ids
}

// posting adds an open Job Posting owned by the Recruiter.
func (s *seed) posting(recruiterID uuid.UUID, title string) uuid.UUID {
	id := uuid.New()
	s.exec(`INSERT INTO public.job_postings (id, status, jd_id, recruiter_id, title, interview_slot)
		VALUES ($1, 'open', $2, $3, $4, 3)`, id, s.jobDescription(), recruiterID, title)
	s.later(`DELETE FROM public.job_postings WHERE id = $1`, id)
	return id
}

// application adds a pending Application from the Candidate to the posting.
func (s *seed) application(postingID, candidateID uuid.UUID) uuid.UUID {
	id := uuid.New()
	s.exec(`INSERT INTO public.applications (id, job_posting_id, candidate_id, cv_path) VALUES ($1, $2, $3, 'cvs/test.pdf')`,
		id, postingID, candidateID)
	s.later(`DELETE FROM public.applications WHERE id = $1`, id)
	return id
}

// avatar adds an avatar in a fresh inventory owned by the Admin.
func (s *seed) avatar(ownerID uuid.UUID, name string) uuid.UUID {
	inventoryID, id := uuid.New(), uuid.New()
	s.exec(`INSERT INTO public.inventories (id, user_id, slot) VALUES ($1, $2, 1)`, inventoryID, ownerID)
	s.later(`DELETE FROM public.inventories WHERE id = $1`, inventoryID)
	s.exec(`INSERT INTO public.avatars (id, inventory_id, name, model_path, personality) VALUES ($1, $2, $3, 'avatars/test.glb', 'calm')`,
		id, inventoryID, name)
	s.later(`DELETE FROM public.avatars WHERE id = $1`, id)
	return id
}

func (s *seed) voice(name string) uuid.UUID {
	id := uuid.New()
	s.exec(`INSERT INTO public.voices (id, name) VALUES ($1, $2)`, id, name)
	s.later(`DELETE FROM public.voices WHERE id = $1`, id)
	return id
}

// interviewSpec describes one seeded interview. Nil fields are stored as NULL.
type interviewSpec struct {
	roleProfileID uuid.UUID
	interviewType string
	applicationID *uuid.UUID
	avatarID      *uuid.UUID
	voiceID       *uuid.UUID
	score         *int32
	feedback      *string
	recordPath    *string
}

func (s *seed) interview(spec interviewSpec) uuid.UUID {
	id := uuid.New()
	s.exec(`INSERT INTO public.interviews (id, role_profile_id, type, application_id, avatar_id, voice_id, score, feedback, record_path)
		VALUES ($1, $2, $3::public.interview_type, $4, $5, $6, $7, $8, $9)`,
		id, spec.roleProfileID, spec.interviewType, spec.applicationID, spec.avatarID, spec.voiceID, spec.score, spec.feedback, spec.recordPath)
	s.later(`DELETE FROM public.interviews WHERE id = $1`, id)
	return id
}

// turn adds one conversation turn; the turn and question texts are what must never leave the API.
func (s *seed) turn(interviewID uuid.UUID, position int, question, answer string) {
	id := uuid.New()
	s.exec(`INSERT INTO public.conversation_turns (id, interview_id, position, question, answer, feedback)
		VALUES ($1, $2, $3, $4, $5, 'turn-feedback-secret')`, id, interviewID, position, question, answer)
	s.later(`DELETE FROM public.conversation_turns WHERE id = $1`, id)
}

// question links a core question to the interview at a position.
func (s *seed) question(interviewID, coreQuestionID uuid.UUID, position int) {
	s.exec(`INSERT INTO public.interview_questions (interview_id, position, core_question_id) VALUES ($1, $2, $3)`,
		interviewID, position, coreQuestionID)
	s.later(`DELETE FROM public.interview_questions WHERE interview_id = $1 AND position = $2`, interviewID, position)
}

func strPtr(v string) *string { return &v }
func i32Ptr(v int32) *int32   { return &v }
func uuidPtr(v uuid.UUID) *uuid.UUID {
	return &v
}

func newService(t *testing.T, pool *pgxpool.Pool) *service.Service {
	t.Helper()
	svc, err := service.New(repository.NewRepository(pool))
	if err != nil {
		t.Fatal(err)
	}
	return svc
}

func codeOf(err error) apperror.Code {
	var appErr *apperror.AppError
	if errors.As(err, &appErr) {
		return appErr.Code
	}
	return ""
}

func assertCode(t *testing.T, err error, want apperror.Code) {
	t.Helper()
	if got := codeOf(err); got != want {
		t.Fatalf("error = %v, want code %s", err, want)
	}
}

// listAll returns the page for a filter, scoped by the caller so counts stay exact on a shared DB.
func listAll(t *testing.T, svc *service.Service, filter domain.ListFilter) domain.InterviewPage {
	t.Helper()
	if filter.Page == 0 {
		filter.Page = 1
	}
	if filter.PageSize == 0 {
		filter.PageSize = domain.DefaultPageSize
	}
	page, err := svc.List(context.Background(), "admin", filter)
	if err != nil {
		t.Fatalf("List(%+v) error = %v", filter, err)
	}
	return page
}

func rowIDs(items []domain.InterviewRow) []uuid.UUID {
	out := make([]uuid.UUID, 0, len(items))
	for _, item := range items {
		out = append(out, item.ID)
	}
	return out
}

func sameSet(got []uuid.UUID, want ...uuid.UUID) bool {
	if len(got) != len(want) {
		return false
	}
	seen := map[uuid.UUID]bool{}
	for _, id := range got {
		seen[id] = true
	}
	for _, id := range want {
		if !seen[id] {
			return false
		}
	}
	return true
}

func TestListFiltersByTypeCandidateAndJobPosting(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newService(t, pool)
	recruiter := s.user("recruiter")
	c1, c2 := s.user("candidate"), s.user("candidate")
	rp1, rp2 := s.roleProfile(c1), s.roleProfile(c2)
	p1, p2 := s.posting(recruiter, "posting one"), s.posting(recruiter, "posting two")
	a1, a2 := s.application(p1, c1), s.application(p2, c2)
	practice1 := s.interview(interviewSpec{roleProfileID: rp1, interviewType: domain.TypePractice})
	recruit1 := s.interview(interviewSpec{roleProfileID: rp1, interviewType: domain.TypeRecruitment, applicationID: uuidPtr(a1)})
	recruit2 := s.interview(interviewSpec{roleProfileID: rp2, interviewType: domain.TypeRecruitment, applicationID: uuidPtr(a2)})

	cases := []struct {
		name   string
		filter domain.ListFilter
		want   []uuid.UUID
	}{
		{"candidate only", domain.ListFilter{CandidateID: uuidPtr(c1)}, []uuid.UUID{practice1, recruit1}},
		{"candidate and practice", domain.ListFilter{CandidateID: uuidPtr(c1), Type: domain.TypePractice}, []uuid.UUID{practice1}},
		{"candidate and recruitment", domain.ListFilter{CandidateID: uuidPtr(c1), Type: domain.TypeRecruitment}, []uuid.UUID{recruit1}},
		{"job posting two", domain.ListFilter{JobPostingID: uuidPtr(p2)}, []uuid.UUID{recruit2}},
		{"job posting one with other candidate", domain.ListFilter{JobPostingID: uuidPtr(p1), CandidateID: uuidPtr(c2)}, nil},
		{"type recruitment for second candidate", domain.ListFilter{CandidateID: uuidPtr(c2), Type: domain.TypeRecruitment}, []uuid.UUID{recruit2}},
	}
	for _, tc := range cases {
		page := listAll(t, svc, tc.filter)
		if int(page.Total) != len(tc.want) || !sameSet(rowIDs(page.Items), tc.want...) {
			t.Fatalf("%s: got ids %v (total %d), want %v", tc.name, rowIDs(page.Items), page.Total, tc.want)
		}
	}

	_, err := svc.List(ctx, "admin", domain.ListFilter{Type: "video", Page: 1, PageSize: 20})
	assertCode(t, err, apperror.CodeValidation)
}

func TestListPaginationTotals(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newService(t, pool)
	c := s.user("candidate")
	rp := s.roleProfile(c)
	var all []uuid.UUID
	for i := 0; i < 5; i++ {
		all = append(all, s.interview(interviewSpec{roleProfileID: rp, interviewType: domain.TypePractice}))
	}

	var collected []uuid.UUID
	for page := int32(1); page <= 3; page++ {
		got := listAll(t, svc, domain.ListFilter{CandidateID: uuidPtr(c), Page: page, PageSize: 2})
		if got.Total != 5 || got.Page != page || got.PageSize != 2 {
			t.Fatalf("page %d = total %d page %d size %d, want total 5", page, got.Total, got.Page, got.PageSize)
		}
		want := 2
		if page == 3 {
			want = 1
		}
		if len(got.Items) != want {
			t.Fatalf("page %d has %d items, want %d", page, len(got.Items), want)
		}
		collected = append(collected, rowIDs(got.Items)...)
	}
	if !sameSet(collected, all...) {
		t.Fatalf("pages returned %v, want every interview once (%v)", collected, all)
	}

	beyond := listAll(t, svc, domain.ListFilter{CandidateID: uuidPtr(c), Page: 4, PageSize: 2})
	if beyond.Total != 5 || len(beyond.Items) != 0 {
		t.Fatalf("page past the end = %+v, want total 5 and no items", beyond)
	}

	ctxBad := context.Background()
	for _, filter := range []domain.ListFilter{
		{Page: 0, PageSize: 20},
		{Page: 1, PageSize: 0},
		{Page: 1, PageSize: domain.MaxPageSize + 1},
	} {
		_, err := svc.List(ctxBad, "admin", filter)
		assertCode(t, err, apperror.CodeValidation)
	}
}

func TestRecruitmentShowsItsJobPostingPracticeShowsNull(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newService(t, pool)
	recruiter, c := s.user("recruiter"), s.user("candidate")
	rp := s.roleProfile(c)
	postingID := s.posting(recruiter, "Go engineer posting")
	appID := s.application(postingID, c)
	practice := s.interview(interviewSpec{roleProfileID: rp, interviewType: domain.TypePractice})
	recruit := s.interview(interviewSpec{roleProfileID: rp, interviewType: domain.TypeRecruitment, applicationID: uuidPtr(appID)})

	page := listAll(t, svc, domain.ListFilter{CandidateID: uuidPtr(c)})
	for _, row := range page.Items {
		switch row.ID {
		case recruit:
			if row.JobPosting == nil || row.JobPosting.ID != postingID || row.JobPosting.Title != "Go engineer posting" {
				t.Fatalf("recruitment row job_posting = %+v, want the posting", row.JobPosting)
			}
		case practice:
			if row.JobPosting != nil {
				t.Fatalf("practice row job_posting = %+v, want nil", row.JobPosting)
			}
		}
	}
	if !strings.Contains(marshalJSON(t, page), `"job_posting":null`) {
		t.Fatal("practice interview must serialize job_posting as null")
	}

	detail, err := svc.Get(ctx, "admin", recruit)
	if err != nil {
		t.Fatal(err)
	}
	if detail.JobPosting == nil || detail.JobPosting.ID != postingID || detail.ApplicationID == nil || *detail.ApplicationID != appID {
		t.Fatalf("recruitment detail = %+v, want its job posting and application", detail)
	}
	detailPractice, err := svc.Get(ctx, "admin", practice)
	if err != nil {
		t.Fatal(err)
	}
	if detailPractice.JobPosting != nil || detailPractice.ApplicationID != nil {
		t.Fatalf("practice detail = %+v, want no job posting or application", detailPractice)
	}
}

func TestHasRecordingIsABooleanAndNoStoredTextLeaks(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newService(t, pool)
	c := s.user("candidate")
	rp := s.roleProfile(c)
	recordPath := "recordings/" + uuid.NewString() + "/private-take.webm"
	feedbackText := "feedback-secret-" + uuid.NewString()
	questionText := "question-secret-" + uuid.NewString()
	answerText := "answer-secret-" + uuid.NewString()

	withRecording := s.interview(interviewSpec{
		roleProfileID: rp, interviewType: domain.TypePractice,
		score: i32Ptr(72), feedback: strPtr(feedbackText), recordPath: strPtr(recordPath),
	})
	s.turn(withRecording, 1, questionText, answerText)
	withoutRecording := s.interview(interviewSpec{roleProfileID: rp, interviewType: domain.TypePractice})

	page := listAll(t, svc, domain.ListFilter{CandidateID: uuidPtr(c)})
	if len(page.Items) != 2 {
		t.Fatalf("items = %d, want 2", len(page.Items))
	}
	for _, row := range page.Items {
		switch row.ID {
		case withRecording:
			if !row.HasRecording || !row.HasReport || row.Score == nil || *row.Score != 72 {
				t.Fatalf("row with recording = %+v, want has_recording and has_report true", row)
			}
		case withoutRecording:
			if row.HasRecording || row.HasReport {
				t.Fatalf("row without recording = %+v, want both false", row)
			}
		}
	}

	body := marshalJSON(t, page)
	for _, secret := range []string{recordPath, "private-take", feedbackText, questionText, answerText, "turn-feedback-secret"} {
		if strings.Contains(body, secret) {
			t.Fatalf("list JSON leaked %q: %s", secret, body)
		}
	}
	for _, key := range []string{`"record_path"`, `"recording_path"`, `"feedback"`, `"transcript"`, `"answer"`, `"audio"`, `"video"`} {
		if strings.Contains(body, key) {
			t.Fatalf("list JSON has a forbidden key %s: %s", key, body)
		}
	}

	detail, err := svc.Get(ctx, "admin", withRecording)
	if err != nil {
		t.Fatal(err)
	}
	detailBody := marshalJSON(t, detail)
	for _, secret := range []string{recordPath, feedbackText, questionText, answerText} {
		if strings.Contains(detailBody, secret) {
			t.Fatalf("detail JSON leaked %q: %s", secret, detailBody)
		}
	}
	for _, key := range []string{`"record_path"`, `"feedback"`, `"transcript"`, `"answer"`, `"audio"`, `"video"`} {
		if strings.Contains(detailBody, key) {
			t.Fatalf("detail JSON has a forbidden key %s: %s", key, detailBody)
		}
	}
}

func TestDetailCountsMatchSeededRows(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newService(t, pool)
	admin, c := s.user("admin"), s.user("candidate")
	rp := s.roleProfile(c)
	avatarID := s.avatar(admin, "Ava")
	voiceID := s.voice("Nova")
	questions := s.coreQuestions(3)
	id := s.interview(interviewSpec{
		roleProfileID: rp, interviewType: domain.TypePractice,
		avatarID: uuidPtr(avatarID), voiceID: uuidPtr(voiceID), score: i32Ptr(60),
	})
	for i, q := range questions {
		s.question(id, q, i+1)
	}
	for i := 1; i <= 4; i++ {
		s.turn(id, i, fmt.Sprintf("question %d", i), fmt.Sprintf("answer %d", i))
	}

	got, err := svc.Get(ctx, "admin", id)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != id || got.Type != domain.TypePractice || got.Owner.ID != c {
		t.Fatalf("detail identity = %+v", got)
	}
	if got.QuestionCount != 3 || got.TurnCount != 4 {
		t.Fatalf("counts = questions %d turns %d, want 3 and 4", got.QuestionCount, got.TurnCount)
	}
	if got.RoleProfile.ID != rp || got.Avatar == nil || got.Avatar.Name != "Ava" || got.Voice == nil || got.Voice.Name != "Nova" {
		t.Fatalf("detail references = role profile %+v avatar %+v voice %+v", got.RoleProfile, got.Avatar, got.Voice)
	}
	if got.Score == nil || *got.Score != 60 {
		t.Fatalf("score = %v, want 60", got.Score)
	}

	bare := s.interview(interviewSpec{roleProfileID: rp, interviewType: domain.TypePractice})
	empty, err := svc.Get(ctx, "admin", bare)
	if err != nil {
		t.Fatal(err)
	}
	if empty.Avatar != nil || empty.Voice != nil || empty.QuestionCount != 0 || empty.TurnCount != 0 {
		t.Fatalf("bare detail = %+v, want no avatar, voice or rows", empty)
	}
}

func TestUnknownInterviewIsNotFoundAndOnlyAdminsRead(t *testing.T) {
	ctx, pool := openPool(t)
	s := newSeed(t, ctx, pool)
	svc := newService(t, pool)
	c := s.user("candidate")
	id := s.interview(interviewSpec{roleProfileID: s.roleProfile(c), interviewType: domain.TypePractice})

	_, err := svc.Get(ctx, "admin", uuid.New())
	assertCode(t, err, apperror.CodeInterviewNotFound)

	for _, role := range []string{"candidate", "recruiter"} {
		_, err := svc.Get(ctx, role, id)
		assertCode(t, err, apperror.CodeForbidden)
		_, err = svc.List(ctx, role, domain.ListFilter{Page: 1, PageSize: 20})
		assertCode(t, err, apperror.CodeForbidden)
	}
}

// marshalJSON returns the serialized body a client would receive.
func marshalJSON(t *testing.T, v any) string {
	t.Helper()
	body, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}
