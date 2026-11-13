package provider

import (
	"fmt"
	"strings"

	"github.com/cog-forge/rolecue/api/internal/features/questionbank/domain"
	"github.com/cog-forge/rolecue/api/internal/pkg/textsafe"
)

const (
	// maxNoteRunes and maxEvidenceRunes cap the job-description text that goes into the prompt.
	maxNoteRunes     = 2000
	maxEvidenceRunes = 4000
	// maxTopicRunes caps one topic summary; a generated topic is at most about 500 characters.
	maxTopicRunes = 1000
)

// fencedRequirements copies the requirements with each evidence_text, a JD excerpt written by a person,
// cleaned, capped and fenced as untrusted data (KAN-300). It never returns nil.
func fencedRequirements(in []domain.ConfirmedRequirement) []domain.ConfirmedRequirement {
	out := make([]domain.ConfirmedRequirement, 0, len(in))
	for _, requirement := range in {
		if requirement.EvidenceText != nil {
			fenced := textsafe.Fence("UNTRUSTED_JD_EVIDENCE", *requirement.EvidenceText, maxEvidenceRunes)
			requirement.EvidenceText = &fenced
		}
		out = append(out, requirement)
	}
	return out
}

// promptRubric is one catalog entry as the model sees it. The id and the question type (a slug the database
// restricts) are plain; every other word, all written by Admins or Recruiters, sits in Summary, which is
// cleaned, capped and fenced as untrusted data.
type promptRubric struct {
	ID           string `json:"id"`
	QuestionType string `json:"question_type"`
	Summary      string `json:"summary"`
}

// promptCatalog turns the allowed rubrics into prompt entries, bounded in count, criteria and text size
// even if the caller passed more. It never returns nil, so an empty catalog serializes as [].
func promptCatalog(rubrics []domain.RubricOption) []promptRubric {
	out := make([]promptRubric, 0, min(len(rubrics), domain.MaxPromptRubrics))
	for _, rubric := range rubrics {
		if len(out) == domain.MaxPromptRubrics {
			break
		}
		var summary strings.Builder
		fmt.Fprintf(&summary, "Name: %s\nDescription: %s\nCriteria:", oneLine(rubric.Name), oneLine(rubric.Description))
		for i, criterion := range rubric.Criteria {
			if i == domain.MaxPromptRubricCriteria {
				fmt.Fprintf(&summary, "\n- (%d more criteria)", len(rubric.Criteria)-i)
				break
			}
			fmt.Fprintf(&summary, "\n- %s [%s, %d points]", oneLine(criterion.Name), oneLine(criterion.Metric), criterion.MaxPoints)
		}
		out = append(out, promptRubric{
			ID:           rubric.ID.String(),
			QuestionType: oneLine(rubric.QuestionType),
			Summary:      textsafe.Fence("UNTRUSTED_RUBRIC", summary.String(), domain.MaxPromptRubricTextRunes),
		})
	}
	return out
}

func oneLine(text string) string {
	return strings.Join(strings.Fields(textsafe.Clean(text)), " ")
}

// promptTopic is one Roadmap topic as the model sees it. The id and the question count are the backend's; the
// title and description were generated from an untrusted job description, so they sit in one cleaned, capped
// and fenced Summary like a rubric does.
type promptTopic struct {
	ID        string `json:"id"`
	Summary   string `json:"summary"`
	Questions int    `json:"questions"`
}

// promptTopics turns the allowed topics into prompt entries, bounded in text size. It never returns nil.
func promptTopics(topics []domain.RoadmapTopic) []promptTopic {
	out := make([]promptTopic, 0, len(topics))
	for _, topic := range topics {
		summary := fmt.Sprintf("Title: %s\nDescription: %s", oneLine(topic.Title), oneLine(topic.Description))
		out = append(out, promptTopic{ID: topic.ID.String(), Questions: topic.Quota,
			Summary: textsafe.Fence("UNTRUSTED_TOPIC", summary, maxTopicRunes)})
	}
	return out
}
