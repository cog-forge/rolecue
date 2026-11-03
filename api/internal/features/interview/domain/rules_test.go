package domain

import (
	"errors"
	"testing"

	"github.com/cog-forge/rolecue/api/pkg/apperror"
)

func TestClassifyTurn(t *testing.T) {
	answered := []Turn{{Position: 1, Kind: TurnCore, Question: "Q1", Answer: "A1"}}
	tests := []struct {
		name      string
		turns     []Turn
		position  int32
		answer    string
		unchanged bool
		wantCode  apperror.Code
	}{
		{name: "first answer", position: 1, answer: "A"},
		{name: "next answer after one", turns: answered, position: 2, answer: "A2"},
		{name: "retry of the same answer", turns: answered, position: 1, answer: "A1", unchanged: true},
		{name: "same position other answer", turns: answered, position: 1, answer: "other", wantCode: apperror.CodeInterviewTurnConflict},
		{name: "position skips the next", turns: answered, position: 3, answer: "A3", wantCode: apperror.CodeInterviewTurnConflict},
		{name: "position zero", position: 0, answer: "A", wantCode: apperror.CodeInterviewTurnConflict},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			unchanged, err := ClassifyTurn(tt.turns, TurnCore, tt.position, tt.answer)
			if tt.wantCode != "" {
				if !apperror.IsCode(err, tt.wantCode) {
					t.Fatalf("error = %v, want %s", err, tt.wantCode)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if unchanged != tt.unchanged {
				t.Fatalf("unchanged = %v, want %v", unchanged, tt.unchanged)
			}
		})
	}
}

func TestCheckRecruitmentStart(t *testing.T) {
	slot := func(n int32) *int32 { return &n }
	tests := []struct {
		name           string
		status         string
		withinDeadline bool
		slot           *int32
		consumed       int64
		wantCode       apperror.Code
	}{
		{name: "eligible with a free slot", status: "interview_eligible", withinDeadline: true, slot: slot(2), consumed: 1},
		{name: "pending application", status: "pending", withinDeadline: true, slot: slot(2), wantCode: apperror.CodeApplicationNotEligible},
		{name: "deadline passed", status: "interview_eligible", withinDeadline: false, slot: slot(2), wantCode: apperror.CodeApplicationNotEligible},
		{name: "capacity not funded", status: "interview_eligible", withinDeadline: true, wantCode: apperror.CodeInterviewCapacityNotFunded},
		{name: "every slot taken", status: "interview_eligible", withinDeadline: true, slot: slot(2), consumed: 2, wantCode: apperror.CodeInterviewCapacityFull},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CheckRecruitmentStart(tt.status, tt.withinDeadline, tt.slot, tt.consumed)
			if tt.wantCode == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			var appErr *apperror.AppError
			if !errors.As(err, &appErr) || appErr.Code != tt.wantCode {
				t.Fatalf("error = %v, want %s", err, tt.wantCode)
			}
		})
	}
}

func TestReasonForTellsCompletionFromLeavingAndAbandonment(t *testing.T) {
	cases := []struct {
		cause                   EndCause
		answered, questionCount int32
		want                    EndReason
	}{
		{EndByCandidate, 8, 8, EndCompleted},
		{EndByCandidate, 2, 8, EndLeft},
		{EndByCandidate, 0, 8, EndLeft},
		{EndBySystem, 2, 8, EndAbandoned},
		{EndBySystem, 8, 8, EndAbandoned},
	}
	for _, c := range cases {
		if got := ReasonFor(c.cause, c.answered, c.questionCount); got != c.want {
			t.Errorf("ReasonFor(%v, %d, %d) = %s, want %s", c.cause, c.answered, c.questionCount, got, c.want)
		}
	}
}

func TestAnsweredCoreSkipsFollowUps(t *testing.T) {
	turns := []Turn{{Kind: TurnCore}, {Kind: TurnFollowUp}, {Kind: TurnCore}}
	if got := AnsweredCore(turns); got != 2 {
		t.Fatalf("AnsweredCore = %d, want 2", got)
	}
}
