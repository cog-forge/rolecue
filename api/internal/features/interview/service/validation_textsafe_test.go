package service

import "testing"

func TestNormalizeAnswerStripsInvisibleCharacters(t *testing.T) {
	hidden := string([]rune{0x200B, 0x202E, 0x0000, 0xFEFF})
	got, err := normalizeAnswer("  I" + hidden + " would add an index. ")
	if err != nil || got != "I would add an index." {
		t.Fatalf("normalizeAnswer() = %q, %v; want the invisible characters removed", got, err)
	}
}

func TestNormalizeAnswerAppliesNFKC(t *testing.T) {
	fullwidth := string([]rune{0xFF49, 0xFF47, 0xFF4E, 0xFF4F, 0xFF52, 0xFF45})
	got, err := normalizeAnswer(fullwidth)
	if err != nil || got != "ignore" {
		t.Fatalf("normalizeAnswer() = %q, %v; want fullwidth letters folded to ASCII", got, err)
	}
}

func TestNormalizeAnswerRejectsAnAnswerThatIsOnlyInvisible(t *testing.T) {
	hidden := string([]rune{0x200B, 0x200C, 0x202E, 0x0007})
	if _, err := normalizeAnswer(hidden + " "); err == nil {
		t.Fatal("an answer made only of invisible characters must be rejected as empty")
	}
}
