package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"sync"

	"github.com/cog-forge/rolecue/api/internal/features/evaluation/domain"
	"github.com/cog-forge/rolecue/api/internal/pkg/textsafe"
	"github.com/google/uuid"
)

const (
	// scoringConcurrency bounds the parallel model calls of one evaluation.
	scoringConcurrency = 4
	// maxQuestionRunes bounds a question sent to the scorer; a follow-up is one short question.
	maxCoreQuestionRunes     = 600
	maxFollowUpQuestionRunes = 300
	// maxGroupSummaryRunes and maxSummaryRunes keep the merged summary inside the service limit.
	maxGroupSummaryRunes = 600
	maxSummaryRunes      = 3500
)

// Score grades the interview with one model call per core question (the core question plus its follow-ups),
// so an injected answer can move the scores of its own question only and never those of another answer
// (KAN-279). Each metric's score is the mean over the questions; its evidence comes from the question that
// scored it highest. The evaluation service still verifies every quote against the stored answers.
func (s *EinoScorer) Score(ctx context.Context, in domain.ScoreInput) (domain.ScoreOutput, error) {
	groups := groupByQuestion(in.Turns)
	if len(groups) == 0 {
		return domain.ScoreOutput{}, invalidOutput(fmt.Errorf("no answers to score"))
	}
	outputs := make([]domain.ScoreOutput, len(groups))
	errs := make([]error, len(groups))
	slots := make(chan struct{}, scoringConcurrency)
	var wg sync.WaitGroup
	for i, group := range groups {
		wg.Add(1)
		slots <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-slots }()
			defer func() {
				if recovered := recover(); recovered != nil {
					errs[i] = fmt.Errorf("scorer call panicked (%T)", recovered)
				}
			}()
			outputs[i], errs[i] = s.scoreGroup(ctx, group, in)
		}()
	}
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			return domain.ScoreOutput{}, err
		}
	}
	return mergeGroups(in.Metrics, groups, outputs)
}

// groupByQuestion splits the turns by core question number, in order of first appearance.
func groupByQuestion(turns []domain.ScoreTurn) [][]domain.ScoreTurn {
	var groups [][]domain.ScoreTurn
	index := map[int32]int{}
	for _, turn := range turns {
		i, ok := index[turn.QuestionNumber]
		if !ok {
			i = len(groups)
			index[turn.QuestionNumber] = i
			groups = append(groups, nil)
		}
		groups[i] = append(groups[i], turn)
	}
	return groups
}

// promptTurn builds the fenced, redacted form of one turn for the prompt. Questions are capped; a follow-up
// is labelled as model-written because it was generated from earlier candidate answers.
func promptTurn(turn domain.ScoreTurn, names []string) scoringTurn {
	question := textsafe.Redact(turn.Question, names...)
	if turn.Kind == domain.TurnFollowUp {
		question = textsafe.Fence("UNTRUSTED_FOLLOW_UP_QUESTION", question, maxFollowUpQuestionRunes)
	} else {
		question = textsafe.Fence("UNTRUSTED_QUESTION", question, maxCoreQuestionRunes)
	}
	return scoringTurn{
		Position: turn.Position, QuestionNumber: turn.QuestionNumber, Kind: turn.Kind,
		Question: question,
		Answer:   textsafe.Fence("UNTRUSTED_ANSWER", textsafe.Redact(turn.Answer, names...), 0),
	}
}

// marshalPrompt encodes the prompt payload without HTML escaping, so the fence markers stay readable.
func marshalPrompt(value any) (string, error) {
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return "", err
	}
	return strings.TrimSpace(buf.String()), nil
}

// mergeGroups combines the per-question outputs. Every group must score each requested metric exactly once
// and may give feedback only for its own positions; anything else is an unusable reply.
func mergeGroups(metrics []domain.ScoredMetric, groups [][]domain.ScoreTurn, outputs []domain.ScoreOutput) (domain.ScoreOutput, error) {
	type tally struct {
		sum      int32
		best     int32
		evidence string
	}
	tallies := make(map[uuid.UUID]*tally, len(metrics))
	for _, metric := range metrics {
		tallies[metric.MetricID] = &tally{best: -1}
	}
	merged := domain.ScoreOutput{}
	summary := ""
	for g, output := range outputs {
		seen := map[uuid.UUID]bool{}
		for _, score := range output.Scores {
			t, ok := tallies[score.MetricID]
			if !ok || seen[score.MetricID] {
				return domain.ScoreOutput{}, invalidOutput(fmt.Errorf("a question scored an unknown or repeated metric"))
			}
			if score.Score < 0 || score.Score > 100 {
				return domain.ScoreOutput{}, invalidOutput(fmt.Errorf("a question score is out of range"))
			}
			seen[score.MetricID] = true
			t.sum += score.Score
			if score.Score > t.best || (score.Score == t.best && t.evidence == "") {
				t.best, t.evidence = score.Score, score.Evidence
			}
		}
		if len(seen) != len(metrics) {
			return domain.ScoreOutput{}, invalidOutput(fmt.Errorf("a question did not score every metric"))
		}
		positions := map[int32]bool{}
		for _, turn := range groups[g] {
			positions[turn.Position] = true
		}
		for _, question := range output.Questions {
			if !positions[question.Position] {
				return domain.ScoreOutput{}, invalidOutput(fmt.Errorf("feedback outside the scored question"))
			}
			merged.Questions = append(merged.Questions, question)
		}
		if text := textsafe.Truncate(output.Summary, maxGroupSummaryRunes); text != "" && len([]rune(summary))+len([]rune(text)) < maxSummaryRunes {
			if summary != "" {
				summary += " "
			}
			summary += text
		}
		merged.InjectionSuspected = merged.InjectionSuspected || output.InjectionSuspected
	}
	merged.Summary = summary
	for _, metric := range metrics {
		t := tallies[metric.MetricID]
		mean := int32(math.Round(float64(t.sum) / float64(len(outputs))))
		merged.Scores = append(merged.Scores, domain.MetricScore{MetricID: metric.MetricID, Score: mean, Evidence: t.evidence})
	}
	return merged, nil
}
