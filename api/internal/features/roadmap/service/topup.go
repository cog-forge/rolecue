package service

import (
	"context"

	questionbankdomain "github.com/cog-forge/rolecue/api/internal/features/questionbank/domain"
	"github.com/cog-forge/rolecue/api/internal/pkg/logger"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// TopUp gives one topic of a candidate's Roadmap enough bank questions for a focused practice start (KAN-347):
// it generates the missing ones in ONE model call and adds them to the bank. have is how many questions the
// topic has now and total how many the session needs. It is called before anything is charged, so a failure
// leaves the wallet and the sessions as they were.
//
// Nothing that exists changes: stored questions, their rubrics and their topic mapping stay, which keeps the
// meaning of every answered question. The call is not inside a database transaction; the questions are added
// in a short one that holds the profile's practice start lock and does nothing when the topic got enough
// questions meanwhile (a double click, a concurrent start). A regeneration that replaced the topic in between
// makes the add fail as a conflict, never as a mapping to a topic that no longer exists.
func (s *Service) TopUp(ctx context.Context, candidate, profile, topic uuid.UUID, have, total int) error {
	if total <= have {
		return nil
	}
	state, err := s.store.ProfileState(ctx, candidate, profile)
	if err != nil {
		return err
	}
	if !state.Confirmed {
		return apperror.New(apperror.CodeRequirementsNotConfirmed, "Confirm the requirements before practicing a topic.")
	}
	item, err := s.store.GetItem(ctx, candidate, profile, topic)
	if err != nil {
		return err
	}
	done := logger.Start(ctx, "roadmap.topup", logger.String("profile_id", profile.String()), logger.Int("have", have), logger.Int("total", total))
	questions, err := s.drafter.DraftTopicQuestions(ctx, candidate, state.JDID,
		questionbankdomain.RoadmapTopic{ID: item.ID, Title: item.Title, Description: item.Description}, total-have)
	if err != nil {
		done(err)
		return err
	}
	err = s.store.InPracticeLock(ctx, candidate, profile, func(ctx context.Context, tx pgx.Tx) error {
		_, appendErr := s.banks.AppendTopicQuestions(ctx, tx, state.JDID, topic, total, questions)
		return appendErr
	})
	done(err, logger.Int("generated", len(questions)))
	return err
}
