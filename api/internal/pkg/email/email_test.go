package email_test

import (
	"errors"
	"testing"

	"github.com/cog-forge/rolecue/api/internal/pkg/email"
)

func TestPermanentWrapping(t *testing.T) {
	base := errors.New("bad address")
	perm := email.Permanent(base)

	if !email.IsPermanent(perm) {
		t.Fatal("Permanent error not recognised")
	}
	if !errors.Is(perm, base) {
		t.Fatal("Permanent must keep the wrapped error reachable with errors.Is")
	}
	if email.IsPermanent(base) {
		t.Fatal("plain error must not be permanent")
	}
	if !email.IsPermanent(errors.Join(errors.New("ctx"), perm)) {
		t.Fatal("IsPermanent must find a permanent error inside a joined error")
	}
	if email.Permanent(nil) != nil {
		t.Fatal("Permanent(nil) must be nil")
	}
}
