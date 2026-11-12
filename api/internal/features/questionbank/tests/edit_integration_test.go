//go:build integration

package tests

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cog-forge/rolecue/api/internal/config"
	moderationdomain "github.com/cog-forge/rolecue/api/internal/features/admin/moderation/domain"
	authdomain "github.com/cog-forge/rolecue/api/internal/features/auth/domain"
	"github.com/cog-forge/rolecue/api/internal/features/questionbank/domain"
	qbservice "github.com/cog-forge/rolecue/api/internal/features/questionbank/service"
	"github.com/cog-forge/rolecue/api/internal/handler"
	"github.com/cog-forge/rolecue/api/internal/middleware"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func TestGradingEditsAtomicStaleAndPrivate(t *testing.T) {
	ctx, pool := openPool(t, time.Minute)
	s := newSeed(t, ctx, pool)
	defer s.cleanup()
	f := newGradingFixture(t, ctx, s)
	other := s.user("recruiter")
	foreign := s.rubric("foreign "+s.tag, &other, true)
	archived := s.rubric("archived "+s.tag, &f.recruiter, false)
	target := s.rubric("target "+s.tag, &f.recruiter, true)
	s.later(`DELETE FROM public.core_questions WHERE jd_id = $1`, f.jd)
	saved := f.fill(t, ctx, f.goodQuestion("First?"), f.goodQuestion("Second?"))
	first := saved[0]
	patch := domain.QuestionPatch{SetRubric: true, RubricID: &target, PreviousRevision: first.Revision}
	_, err := f.bank.WriteQuestion(ctx, f.recruiter, f.posting, &first.ID, patch)
	wantCode(t, err, apperror.CodeExpectedAnswerReviewRequired)
	for _, id := range []uuid.UUID{foreign, archived, uuid.New()} {
		patch.RubricID, patch.SetExpectedAnswer = &id, true
		_, err = f.bank.WriteQuestion(ctx, f.recruiter, f.posting, &first.ID, patch)
		wantCode(t, err, apperror.CodeRubricNotFound)
	}
	if err := f.jobs.Submit(ctx, f.recruiter, f.posting); err != nil {
		t.Fatal(err)
	}
	_, err = f.bank.ApplyRubric(ctx, f.recruiter, f.posting, domain.BulkRubricRequest{RubricID: f.rubric, QuestionIDs: []uuid.UUID{first.ID}, PreviousRevisions: map[uuid.UUID]string{first.ID: first.Revision}})
	if err != nil {
		t.Fatal(err)
	}
	if _, submitted := f.state(t, ctx); !submitted {
		t.Fatal("identical bulk assignment withdrew submission")
	}
	unchanged, err := f.bank.WriteQuestion(ctx, f.recruiter, f.posting, &first.ID, domain.QuestionPatch{SetExpectedAnswer: true, ExpectedAnswer: first.ExpectedAnswer, PreviousRevision: first.Revision})
	if err != nil || unchanged.Revision != first.Revision {
		t.Fatal("identical edit changed revision")
	}
	if _, submitted := f.state(t, ctx); !submitted {
		t.Fatal("identical edit withdrew submission")
	}
	answerText := "Updated key points."
	changed, err := f.bank.WriteQuestion(ctx, f.recruiter, f.posting, &first.ID, domain.QuestionPatch{SetExpectedAnswer: true, ExpectedAnswer: &answerText, PreviousRevision: first.Revision})
	if err != nil || changed.Revision == first.Revision {
		t.Fatal("answer edit failed")
	}
	if _, submitted := f.state(t, ctx); submitted {
		t.Fatal("grading edit did not withdraw submission")
	}
	_, err = f.bank.WriteQuestion(ctx, f.recruiter, f.posting, &first.ID, domain.QuestionPatch{SetExpectedAnswer: true, PreviousRevision: first.Revision})
	wantCode(t, err, apperror.CodeQuestionStale)
	req := domain.BulkRubricRequest{RubricID: target, QuestionIDs: []uuid.UUID{first.ID, uuid.New()}, PreviousRevisions: map[uuid.UUID]string{first.ID: changed.Revision}}
	_, err = f.bank.ApplyRubric(ctx, f.recruiter, f.posting, req)
	wantCode(t, err, apperror.CodeCoreQuestionNotFound)
	listed, _ := f.bank.ListBank(ctx, f.jd, f.recruiter)
	for _, q := range listed {
		if q.RubricID == nil || *q.RubricID != f.rubric {
			t.Fatal("failed bulk partially committed")
		}
	}
	req.QuestionIDs = []uuid.UUID{first.ID, saved[1].ID}
	req.PreviousRevisions[saved[1].ID] = saved[1].Revision
	applied, err := f.bank.ApplyRubric(ctx, f.recruiter, f.posting, req)
	if err != nil {
		t.Fatal(err)
	}
	for _, q := range applied {
		if q.ExpectedAnswer != nil || !q.GradingIncomplete || *q.RubricID != target {
			t.Fatal("bulk did not clear changed answers")
		}
	}
	runtime, err := f.bank.List(ctx, f.jd)
	if err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(runtime)
	if strings.Contains(string(encoded), "rubric") || strings.Contains(string(encoded), "expected_answer") || strings.Contains(string(encoded), "revision") {
		t.Fatal("runtime leaked grading data")
	}
}

func TestApprovalRacingGradingEdit(t *testing.T) {
	ctx, pool := openPool(t, time.Minute)
	s := newSeed(t, ctx, pool)
	defer s.cleanup()
	for i := 0; i < 12; i++ {
		f := newGradingFixture(t, ctx, s)
		saved := f.fill(t, ctx, f.goodQuestion("Original?"))
		if err := f.jobs.Submit(ctx, f.recruiter, f.posting); err != nil {
			t.Fatal(err)
		}
		start := make(chan struct{})
		var wg sync.WaitGroup
		wg.Add(2)
		var approveErr, editErr error
		go func() {
			defer wg.Done()
			<-start
			approveErr = f.moderate.Decide(ctx, f.posting, moderationdomain.PostingStatusOpen, nil)
		}()
		go func() {
			defer wg.Done()
			<-start
			if i%2 == 0 {
				_, editErr = f.bank.WriteQuestion(ctx, f.recruiter, f.posting, &saved[0].ID, domain.QuestionPatch{SetExpectedAnswer: true, PreviousRevision: saved[0].Revision})
			} else {
				_, editErr = f.bank.ApplyRubric(ctx, f.recruiter, f.posting, domain.BulkRubricRequest{RubricID: uuid.MustParse("00000000-0000-4000-8000-000000000201"), QuestionIDs: []uuid.UUID{saved[0].ID}, PreviousRevisions: map[uuid.UUID]string{saved[0].ID: saved[0].Revision}})
			}
		}()
		close(start)
		wg.Wait()
		status, submitted := f.state(t, ctx)
		if approveErr == nil && codeOf(editErr) == apperror.CodeQuestionBankLocked && status == "open" {
			continue
		}
		if editErr == nil && errors.Is(approveErr, moderationdomain.ErrNotPending) && status == "pending" && !submitted {
			continue
		}
		t.Fatalf("inconsistent approval/edit outcome: %v / %v", approveErr, editErr)
	}
}

type editSession struct{ user authdomain.User }

func (s editSession) Authenticate(context.Context, *http.Request) (authdomain.User, []string, error) {
	return s.user, nil, nil
}

type unusedGenerator struct{}

func (unusedGenerator) Generate(context.Context, domain.GenerationInput, int) (domain.GenerationCandidate, error) {
	return domain.GenerationCandidate{}, errors.New("not called")
}

func TestGradingWriteHTTPContract(t *testing.T) {
	ctx, pool := openPool(t, time.Minute)
	s := newSeed(t, ctx, pool)
	defer s.cleanup()
	f := newGradingFixture(t, ctx, s)
	saved := f.fill(t, ctx, f.goodQuestion("Question?"))
	svc, err := qbservice.New(f.bank, unusedGenerator{}, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	gin.SetMode(gin.TestMode)
	h := handler.NewQuestionBankHandler(svc, time.Minute)
	router := func(role string) *gin.Engine {
		r := gin.New()
		g := r.Group("/job-postings", middleware.RequireAuth(editSession{authdomain.User{ID: f.recruiter, Role: role}}, config.CORSConfig{AllowOrigins: []string{"http://localhost:3490"}}))
		g.PATCH("/:id/questions/:questionId", h.Update)
		g.POST("/:id/questions", h.Add)
		g.POST("/:id/questions/apply-rubric", h.ApplyRubric)
		return r
	}
	path := "/job-postings/" + f.posting.String() + "/questions/" + saved[0].ID.String()
	for _, tc := range []struct {
		body, role, origin string
		status             int
	}{
		{`{}`, "recruiter", "http://localhost:3490", 400},
		{`{"unknown":true}`, "recruiter", "http://localhost:3490", 400},
		{`{"expected_answer":42}`, "recruiter", "http://localhost:3490", 400},
		{`{"expected_answer":null}`, "candidate", "http://localhost:3490", 403},
		{`{"expected_answer":null}`, "recruiter", "http://evil.example", 403},
		{`{"expected_answer":"` + strings.Repeat("x", 17<<10) + `"}`, "recruiter", "http://localhost:3490", 400},
		{`{"expected_answer":null}`, "recruiter", "http://localhost:3490", 200},
	} {
		req := httptest.NewRequest(http.MethodPatch, path, strings.NewReader(tc.body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Origin", tc.origin)
		w := httptest.NewRecorder()
		router(tc.role).ServeHTTP(w, req)
		if w.Code != tc.status {
			t.Errorf("HTTP status %d, want %d", w.Code, tc.status)
		}
	}
}
