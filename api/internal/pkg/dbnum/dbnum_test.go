package dbnum

import (
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
)

func TestFloat(t *testing.T) {
	var n pgtype.Numeric
	if err := n.Scan("62.50"); err != nil {
		t.Fatal(err)
	}
	if got := Float(n); got == nil || *got != 62.5 {
		t.Fatalf("Float(62.50) = %v, want 62.5", got)
	}
	if got := Float(pgtype.Numeric{}); got != nil {
		t.Fatalf("Float(NULL) = %v, want nil", *got)
	}
}
