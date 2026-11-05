package provider

import (
	"os"
	"path/filepath"
	"testing"
)

// The opt-in promptfoo suite (api/tests/promptfoo) sends the real scoring instruction to a model. Its copy
// must not drift from the one the server uses. Run with UPDATE_PROMPTFOO=1 to rewrite the copy.
func TestPromptfooScoringPromptMatchesTheServerInstruction(t *testing.T) {
	path := filepath.Join("..", "..", "..", "..", "tests", "promptfoo", "prompts", "scoring-system.txt")
	if os.Getenv("UPDATE_PROMPTFOO") == "1" {
		if err := os.WriteFile(path, []byte(scoringInstruction), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != scoringInstruction {
		t.Fatalf("%s is out of date; run UPDATE_PROMPTFOO=1 go test ./internal/features/evaluation/provider", path)
	}
}
