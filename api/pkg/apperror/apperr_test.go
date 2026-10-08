package apperror_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/cog-forge/rolecue/api/pkg/apperror"
)

var _ error = (*apperror.AppError)(nil)

func TestAppError(t *testing.T) {
	cause := errors.New("private database failure")
	for _, tc := range []struct {
		name      string
		err       *apperror.AppError
		message   string
		wantText  string
		wantCause error
	}{
		{"new", apperror.New(apperror.CodeValidation, "invalid input"), "invalid input", "invalid input", nil},
		{"wrapped cause", apperror.Wrap(apperror.CodeValidation, "invalid input", cause), "invalid input", "invalid input: private database failure", cause},
		{"empty message with cause", apperror.Wrap(apperror.CodeValidation, "", cause), "", cause.Error(), cause},
		{"nil cause", apperror.Wrap(apperror.CodeValidation, "invalid input", nil), "invalid input", "invalid input", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.err.Code != apperror.CodeValidation || tc.err.Message != tc.message {
				t.Fatalf("public fields lost: %+v", tc.err)
			}
			if tc.err.Error() != tc.wantText || errors.Unwrap(tc.err) != tc.wantCause || tc.err.Err != tc.wantCause {
				t.Fatalf("error text or cause lost: %#v", tc.err)
			}
			outer := fmt.Errorf("service: %w", tc.err)
			var appErr *apperror.AppError
			if !errors.As(outer, &appErr) || appErr != tc.err {
				t.Fatal("errors.As did not recover the application error")
			}
			if tc.wantCause != nil && !errors.Is(outer, tc.wantCause) {
				t.Fatal("errors.Is did not recover the cause")
			}
		})
	}
}

func TestAppErrorNilReceiver(t *testing.T) {
	var err *apperror.AppError

	if err.Error() != "<nil>" {
		t.Fatalf("nil receiver error text = %q, want %q", err.Error(), "<nil>")
	}

	if got := errors.Unwrap(err); got != nil {
		t.Fatalf("errors.Unwrap(nil AppError) = %v, want nil", got)
	}
}
