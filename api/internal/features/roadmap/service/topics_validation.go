package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/cog-forge/rolecue/api/internal/features/roadmap/domain"
	"github.com/cog-forge/rolecue/api/internal/pkg/textsafe"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
)

// badTopicOutput is an invalid-output error, which the retry loop may retry. The cause names the rule and
// counts only; topic text is never placed in an error.
func badTopicOutput(message string, cause error) error {
	return apperror.Wrap(apperror.CodeInvalidRoadmapGenerationOutput, message, cause)
}

// titleKey folds case and whitespace so "Go  Concurrency" and "go concurrency" are one topic.
func titleKey(title string) string { return strings.ToLower(strings.Join(strings.Fields(title), " ")) }

func cleanTopicText(raw string, maxRunes int, required bool) (string, error) {
	if !utf8.ValidString(raw) {
		return "", errors.New("text is not valid UTF-8")
	}
	if strings.ContainsRune(raw, 0) {
		return "", errors.New("text contains a null character")
	}
	text := strings.TrimSpace(textsafe.SanitizeOutput(raw))
	if required && text == "" {
		return "", errors.New("text is empty")
	}
	if utf8.RuneCountInString(text) > maxRunes {
		return "", fmt.Errorf("text is longer than %d characters", maxRunes)
	}
	if textsafe.LooksLikeInjection(text) {
		return "", errors.New("text looks like an instruction")
	}
	return text, nil
}

// validateTopics turns an untrusted model proposal into the topics to store. Titles are required, bounded and
// cleaned; a duplicate title is dropped; fewer than MinTopics distinct topics is invalid output and more than
// MaxTopics keeps the first MaxTopics (the model orders by importance). The backend assigns every position:
// nothing the model sends other than the two texts is used.
func validateTopics(candidate domain.TopicCandidate) ([]domain.NewItem, error) {
	if candidate.Topics == nil {
		return nil, badTopicOutput("roadmap generation returned no topics", errors.New("topics array is missing"))
	}
	seen := make(map[string]struct{}, len(candidate.Topics))
	items := make([]domain.NewItem, 0, len(candidate.Topics))
	for index, raw := range candidate.Topics {
		title, err := cleanTopicText(raw.Title, domain.MaxTopicTitleRunes, true)
		if err != nil {
			return nil, badTopicOutput("roadmap generation returned an unusable topic", fmt.Errorf("topic %d title: %w", index+1, err))
		}
		description, err := cleanTopicText(raw.Description, domain.MaxTopicDescriptionRunes, false)
		if err != nil {
			return nil, badTopicOutput("roadmap generation returned an unusable topic", fmt.Errorf("topic %d description: %w", index+1, err))
		}
		key := titleKey(title)
		if _, duplicate := seen[key]; duplicate {
			continue
		}
		seen[key] = struct{}{}
		items = append(items, domain.NewItem{Title: title, Description: description})
	}
	if len(items) < domain.MinTopics {
		return nil, badTopicOutput("roadmap generation returned too few distinct topics",
			fmt.Errorf("%d distinct topics, at least %d required", len(items), domain.MinTopics))
	}
	if len(items) > domain.MaxTopics {
		items = items[:domain.MaxTopics]
	}
	for i := range items {
		items[i].Position = i
	}
	return items, nil
}

// ValidateTopics checks a topic proposal the way the generation service does. The AI task adapter calls it
// inside every routed attempt, so an unusable proposal is retried or falls back like any invalid output.
func ValidateTopics(candidate domain.TopicCandidate) ([]domain.NewItem, error) {
	return validateTopics(candidate)
}

func isInvalidTopicOutput(err error) bool {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	var appErr *apperror.AppError
	return errors.As(err, &appErr) && appErr != nil && appErr.Code == apperror.CodeInvalidRoadmapGenerationOutput
}
