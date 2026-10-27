package service

import (
	"strings"
	"testing"

	"github.com/cog-forge/rolecue/api/internal/features/jobposting/domain"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
)

func TestRefinementNoteUsesUnicodeLimitAndPatchClearing(t *testing.T) {
	for _, value := range []string{strings.Repeat("界", 2001), "a\x00b", string([]byte{0xff})} {
		_, err := refinementNote(value)
		assertCode(t, err, apperror.CodeValidation)
	}
	valid := "  " + strings.Repeat("界", 2000) + "  "
	value, err := refinementNote(valid)
	if err != nil || value != strings.TrimSpace(valid) {
		t.Fatalf("valid note: %q %v", value, err)
	}
	patch, err := validatePatch(domain.Edit{RefinementNote: domain.StringChange{Present: true}})
	if err != nil || !patch.RefinementNote.Present || patch.RefinementNote.Value == nil || *patch.RefinementNote.Value != "" {
		t.Fatalf("null must explicitly clear the note: %+v %v", patch, err)
	}
}
