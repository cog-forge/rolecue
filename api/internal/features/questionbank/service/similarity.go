package service

import (
	"strings"
	"unicode"

	"github.com/cog-forge/rolecue/api/internal/features/questionbank/domain"
)

// Near-duplicate questions (KAN-384). Two questions are near duplicates when their informative tokens mostly
// coincide: the text is lowercased, split on everything that is not a letter or digit, function and
// question-scaffolding words are dropped, and each remaining token is cut to its first stemRunes runes as a
// crude stem (cancel / cancellation, channel / channels). The measure is Jaccard similarity of the two token
// sets, and the pair must also share at least minSharedTokens tokens, so two short questions that only share
// a technical term ("What is context in Go?", "How do you cancel with context in Go?") stay distinct. No model
// is involved and no text is logged.

const (
	stemRunes       = 6
	minSharedTokens = 3
)

// stopWords are dropped before comparing. Besides English function words they hold the scaffolding of an
// interview question (explain, describe, difference, choose ...), which says nothing about its subject.
var stopWords = func() map[string]struct{} {
	words := strings.Fields(`a an the and or but nor of in on at to for from by with without into onto over under
		about as than then so if else is are was were be been being am do does did done doing have has had having
		can could will would shall should may might must i you your yours we our they their them it its this that
		these those there here what which who whom whose how why when where while me my some any each every all
		both either neither not no yes vs versus between among use uses using used instead rather prefer
		explain describe describes difference differences different differ differs compare comparing tell walk
		through give example examples choose pick decide approach way ways situation situations scenario
		scenarios case cases would like good better best typical typically usually common commonly`)
	set := make(map[string]struct{}, len(words))
	for _, w := range words {
		set[w] = struct{}{}
	}
	return set
}()

// questionTokens returns the informative token stems of a question.
func questionTokens(question string) map[string]struct{} {
	out := make(map[string]struct{})
	for _, word := range strings.FieldsFunc(strings.ToLower(question), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	}) {
		if _, stop := stopWords[word]; stop {
			continue
		}
		if runes := []rune(word); len(runes) > stemRunes {
			word = string(runes[:stemRunes])
		}
		out[word] = struct{}{}
	}
	return out
}

// similarity is the Jaccard similarity of two token sets; two empty sets are not similar.
func similarity(a, b map[string]struct{}) float64 {
	shared := sharedTokens(a, b)
	union := len(a) + len(b) - shared
	if union == 0 {
		return 0
	}
	return float64(shared) / float64(union)
}

func sharedTokens(a, b map[string]struct{}) int {
	if len(b) < len(a) {
		a, b = b, a
	}
	shared := 0
	for token := range a {
		if _, ok := b[token]; ok {
			shared++
		}
	}
	return shared
}

// nearDuplicate reports whether two questions' tokens are near duplicates at threshold.
func nearDuplicate(a, b map[string]struct{}, threshold float64) bool {
	return sharedTokens(a, b) >= minSharedTokens && similarity(a, b) >= threshold
}

// duplicateThreshold is the input's threshold, or the default when unset.
func duplicateThreshold(in domain.GenerationInput) float64 {
	if in.DuplicateThreshold <= 0 {
		return domain.DefaultDuplicateThreshold
	}
	return in.DuplicateThreshold
}
