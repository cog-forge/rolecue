package service

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	// maxEvidenceRunes bounds one evidence quote; a longer one is not stored.
	maxEvidenceRunes = 500
	// minEvidenceRunes keeps a trivial fragment ("the") from verifying anything. A quote shorter than
	// this is accepted only when it is a whole answer.
	minEvidenceRunes = 8
)

// normalizeEvidenceText folds text the way the interview service stores answers (trimmed, valid UTF-8)
// and a little further, so a quote the model re-typed still matches: whitespace runs become one space,
// typographic quotes and dashes become ASCII, zero-width characters go, and case is ignored. The
// mirror of the interview service's normalizeAnswer lives here because features do not import each other.
func normalizeEvidenceText(text string) string {
	var b strings.Builder
	space := false
	for _, r := range text {
		switch {
		case r == '\u200b' || r == '\u200c' || r == '\u200d' || r == '\ufeff' || r == '\u00ad':
			continue
		case unicode.IsSpace(r):
			space = true
			continue
		}
		if space && b.Len() > 0 {
			b.WriteByte(' ')
		}
		space = false
		switch r {
		case '\u2018', '\u2019', '\u201b':
			r = '\''
		case '\u201c', '\u201d', '\u201e':
			r = '"'
		case '\u2013', '\u2014', '\u2212':
			r = '-'
		}
		b.WriteRune(unicode.ToLower(r))
	}
	return b.String()
}

// verifyEvidence reports whether a quote appears verbatim, after normalisation, in one of the answers.
// An empty or oversized quote, or one that is not valid UTF-8, never verifies.
func verifyEvidence(quote string, answers []string) bool {
	if !utf8.ValidString(quote) {
		return false
	}
	quote = normalizeEvidenceText(quote)
	runes := utf8.RuneCountInString(quote)
	if runes == 0 || runes > maxEvidenceRunes {
		return false
	}
	for _, answer := range answers {
		if quote == answer {
			return true
		}
		if runes >= minEvidenceRunes && strings.Contains(answer, quote) {
			return true
		}
	}
	return false
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
