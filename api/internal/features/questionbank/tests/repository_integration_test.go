//go:build integration

package tests

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/cog-forge/rolecue/api/internal/features/questionbank/domain"
	qbrepository "github.com/cog-forge/rolecue/api/internal/features/questionbank/repository"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

func TestQuestionBankRepositoryAgainstPostgres(t *testing.T) {
	ctx, pool := openPool(t, 60*time.Second)
	s := newSeed(t, ctx, pool)
	defer s.cleanup()

	levelID := s.level("qb-"+s.tag, "qb level description")

	recruiter, otherRecruiter := s.user("recruiter"), s.user("recruiter")
	candidate, otherCandidate := s.user("candidate"), s.user("candidate")

	jd := s.jd(levelID, "skip Docker")
	postingID := s.posting(jd, recruiter)
	s.exec(`INSERT INTO public.requirements (jd_id, name, target, type) VALUES ($1, 'Go', 'backend', 'required'), ($1, 'Docker', 'ops', 'preferred')`, jd)
	s.later(`DELETE FROM public.requirements WHERE jd_id = $1`, jd)
	s.later(`DELETE FROM public.core_questions WHERE jd_id = $1`, jd)

	repo := qbrepository.NewRepository(pool)

	t.Run("posting is owner scoped", func(t *testing.T) {
		got, err := repo.Posting(ctx, recruiter, postingID)
		if err != nil || got.JDID != jd || got.Status != domain.PostingPending {
			t.Fatalf("got %+v, %v", got, err)
		}
		_, err = repo.Posting(ctx, otherRecruiter, postingID)
		wantCode(t, err, apperror.CodeJobPostingNotFound)
		_, err = repo.Posting(ctx, recruiter, uuid.New())
		wantCode(t, err, apperror.CodeJobPostingNotFound)
	})

	t.Run("generation input has requirements and the refinement note", func(t *testing.T) {
		in, err := repo.GenerationInput(ctx, jd)
		if err != nil || len(in.Requirements) != 2 || in.RefinementNote != "skip Docker" {
			t.Fatalf("got %+v, %v", in, err)
		}
		if in.Seniority.Name != "qb-"+s.tag || in.Seniority.Description == nil || *in.Seniority.Description != "qb level description" {
			t.Fatalf("seniority level was not read from the JD's level: %+v", in.Seniority)
		}
	})

	t.Run("save once, then exists", func(t *testing.T) {
		saved, err := repo.SaveForPosting(ctx, recruiter, postingID, []string{"Q1?", "Q2?", "Q3?"})
		if err != nil || len(saved) != 3 {
			t.Fatalf("got %v, %v", saved, err)
		}
		_, err = repo.SaveForPosting(ctx, recruiter, postingID, []string{"Other?"})
		wantCode(t, err, apperror.CodeQuestionBankExists)
		listed, err := repo.List(ctx, jd)
		if err != nil || len(listed) != 3 {
			t.Fatalf("bank must be unchanged by the refused save: %v, %v", listed, err)
		}
	})

	t.Run("another recruiter cannot save or edit", func(t *testing.T) {
		_, err := repo.SaveForPosting(ctx, otherRecruiter, postingID, []string{"X?"})
		wantCode(t, err, apperror.CodeJobPostingNotFound)
		_, err = repo.Add(ctx, otherRecruiter, postingID, "X?")
		wantCode(t, err, apperror.CodeJobPostingNotFound)
	})

	t.Run("add, update and delete while pending", func(t *testing.T) {
		added, err := repo.Add(ctx, recruiter, postingID, "Added?")
		if err != nil || added.Content != "Added?" {
			t.Fatalf("got %+v, %v", added, err)
		}
		updated, err := repo.Update(ctx, recruiter, postingID, added.ID, "Edited?")
		if err != nil || updated.Content != "Edited?" {
			t.Fatalf("got %+v, %v", updated, err)
		}
		_, err = repo.Update(ctx, recruiter, postingID, uuid.New(), "Nope?")
		wantCode(t, err, apperror.CodeCoreQuestionNotFound)
		_, err = repo.Update(ctx, otherRecruiter, postingID, added.ID, "Hijack?")
		wantCode(t, err, apperror.CodeJobPostingNotFound)
		if err := repo.Delete(ctx, recruiter, postingID, added.ID); err != nil {
			t.Fatal(err)
		}
		wantCode(t, repo.Delete(ctx, recruiter, postingID, added.ID), apperror.CodeCoreQuestionNotFound)
	})

	t.Run("a question of another JD cannot be touched through my posting", func(t *testing.T) {
		otherJD := s.jd(levelID, "")
		otherPosting := s.posting(otherJD, recruiter)
		s.later(`DELETE FROM public.core_questions WHERE jd_id = $1`, otherJD)
		foreign, err := repo.Add(ctx, recruiter, otherPosting, "Belongs to the other posting?")
		if err != nil {
			t.Fatal(err)
		}
		// postingID is mine and pending, but the question belongs to otherJD.
		_, err = repo.Update(ctx, recruiter, postingID, foreign.ID, "Hijacked?")
		wantCode(t, err, apperror.CodeCoreQuestionNotFound)
		wantCode(t, repo.Delete(ctx, recruiter, postingID, foreign.ID), apperror.CodeCoreQuestionNotFound)
		still, err := repo.List(ctx, otherJD)
		if err != nil || len(still) != 1 || still[0].Content != "Belongs to the other posting?" {
			t.Fatalf("the foreign question was altered: %v, %v", still, err)
		}
	})

	t.Run("a question used by an interview cannot be deleted", func(t *testing.T) {
		listed, err := repo.List(ctx, jd)
		if err != nil || len(listed) == 0 {
			t.Fatalf("got %v, %v", listed, err)
		}
		practiceJD := s.jd(levelID, "")
		profile := s.roleProfile(practiceJD, candidate)
		interviewID := uuid.New()
		s.exec(`INSERT INTO public.interviews (id, role_profile_id, type) VALUES ($1, $2, 'practice')`, interviewID, profile)
		s.later(`DELETE FROM public.interviews WHERE id = $1`, interviewID)
		s.exec(`INSERT INTO public.interview_questions (interview_id, position, core_question_id) VALUES ($1, 1, $2)`, interviewID, listed[0].ID)
		s.later(`DELETE FROM public.interview_questions WHERE interview_id = $1`, interviewID)
		wantCode(t, repo.Delete(ctx, recruiter, postingID, listed[0].ID), apperror.CodeCoreQuestionInUse)
	})

	t.Run("a locked posting refuses every change", func(t *testing.T) {
		s.exec(`UPDATE public.job_postings SET status = 'open' WHERE id = $1`, postingID)
		listed, err := repo.List(ctx, jd)
		if err != nil || len(listed) == 0 {
			t.Fatalf("got %v, %v", listed, err)
		}
		_, err = repo.Add(ctx, recruiter, postingID, "Late?")
		wantCode(t, err, apperror.CodeQuestionBankLocked)
		_, err = repo.Update(ctx, recruiter, postingID, listed[0].ID, "Late?")
		wantCode(t, err, apperror.CodeQuestionBankLocked)
		wantCode(t, repo.Delete(ctx, recruiter, postingID, listed[0].ID), apperror.CodeQuestionBankLocked)
		_, err = repo.SaveForPosting(ctx, recruiter, postingID, []string{"Late?"})
		wantCode(t, err, apperror.CodeQuestionBankLocked)
		after, _ := repo.List(ctx, jd)
		if len(after) != len(listed) {
			t.Fatalf("a refused change altered the bank: %d -> %d", len(listed), len(after))
		}
	})

	t.Run("concurrent generation creates exactly one bank", func(t *testing.T) {
		jd2 := s.jd(levelID, "")
		posting2 := s.posting(jd2, recruiter)
		s.later(`DELETE FROM public.core_questions WHERE jd_id = $1`, jd2)
		const workers = 6
		var wg sync.WaitGroup
		results := make(chan error, workers)
		for i := 0; i < workers; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				_, err := repo.SaveForPosting(ctx, recruiter, posting2, []string{fmt.Sprintf("A%d?", i), fmt.Sprintf("B%d?", i)})
				results <- err
			}(i)
		}
		wg.Wait()
		close(results)
		ok, exists := 0, 0
		for err := range results {
			switch {
			case err == nil:
				ok++
			case codeOf(err) == apperror.CodeQuestionBankExists:
				exists++
			default:
				t.Fatalf("unexpected error: %v", err)
			}
		}
		listed, _ := repo.List(ctx, jd2)
		if ok != 1 || exists != workers-1 || len(listed) != 2 {
			t.Fatalf("ok=%d exists=%d questions=%d; want 1, %d, 2", ok, exists, len(listed), workers-1)
		}
	})

	t.Run("practice bank belongs to the candidate", func(t *testing.T) {
		practiceJD := s.jd(levelID, "")
		s.roleProfile(practiceJD, candidate)
		s.later(`DELETE FROM public.core_questions WHERE jd_id = $1`, practiceJD)
		if err := repo.CandidateOwnsJD(ctx, candidate, practiceJD); err != nil {
			t.Fatal(err)
		}
		wantCode(t, repo.CandidateOwnsJD(ctx, otherCandidate, practiceJD), apperror.CodeJDNotFound)
		_, err := repo.SaveForPractice(ctx, otherCandidate, practiceJD, []string{"X?"})
		wantCode(t, err, apperror.CodeJDNotFound)
		saved, err := repo.SaveForPractice(ctx, candidate, practiceJD, []string{"P1?", "P2?"})
		if err != nil || len(saved) != 2 {
			t.Fatalf("got %v, %v", saved, err)
		}
		_, err = repo.SaveForPractice(ctx, candidate, practiceJD, []string{"P3?"})
		wantCode(t, err, apperror.CodeQuestionBankExists)
	})
}
