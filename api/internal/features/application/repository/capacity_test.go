package repository

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var (
	refundRecruiter = uuid.MustParse("11111111-1111-4111-8111-111111111111")
	refundPosting   = uuid.MustParse("22222222-2222-4222-8222-222222222222")
)

// refundDB answers the three reads of the refund and records the coin rows the ledger writes. It
// stands in for the pgx transaction the refund runs in, so the arithmetic is checked without a
// database; the lock and the commit are covered by the integration suite.
type refundDB struct {
	slot       *int32
	consumed   int64
	credits    []int32
	walletOK   bool
	statements []string
}

// rowFunc is a pgx.Row whose Scan is the given function.
type rowFunc func(dest ...any) error

func (f rowFunc) Scan(dest ...any) error { return f(dest...) }

func (d *refundDB) QueryRow(_ context.Context, sql string, _ ...any) pgx.Row {
	d.statements = append(d.statements, sql)
	switch {
	case strings.Contains(sql, "SELECT recruiter_id, interview_slot"):
		return rowFunc(func(dest ...any) error {
			*(dest[0].(*uuid.UUID)) = refundRecruiter
			*(dest[1].(**int32)) = d.slot
			return nil
		})
	case strings.Contains(sql, "count(*)::bigint AS consumed"):
		return rowFunc(func(dest ...any) error {
			*(dest[0].(*int64)) = d.consumed
			return nil
		})
	case strings.Contains(sql, "SELECT id, balance FROM public.wallets"):
		return rowFunc(func(dest ...any) error {
			if !d.walletOK {
				return pgx.ErrNoRows
			}
			*(dest[0].(*uuid.UUID)) = uuid.MustParse("33333333-3333-4333-8333-333333333333")
			*(dest[1].(*int32)) = 0
			return nil
		})
	}
	return rowFunc(func(...any) error { return errors.New("unexpected query: " + sql) })
}

func (d *refundDB) Exec(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	d.statements = append(d.statements, sql)
	switch {
	case strings.Contains(sql, "INSERT INTO public.wallets"):
		d.walletOK = true
	case strings.Contains(sql, "INSERT INTO public.transactions"):
		d.credits = append(d.credits, args[2].(int32))
	}
	return pgconn.NewCommandTag("OK 1"), nil
}

func (d *refundDB) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return nil, errors.New("refund does not query rows")
}

func int32Ptr(n int32) *int32 { return &n }

func TestRefundCreditsUnusedSlots(t *testing.T) {
	cases := []struct {
		name     string
		slot     *int32
		consumed int64
		want     []int32
	}{
		{"funded five, none consumed credits five", int32Ptr(5), 0, []int32{5}},
		{"funded five, two consumed credits three", int32Ptr(5), 2, []int32{3}},
		{"all consumed credits nothing", int32Ptr(5), 5, nil},
		{"more consumed than funded still credits nothing", int32Ptr(2), 3, nil},
		{"unfunded posting creates no row and no wallet", nil, 0, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db := &refundDB{slot: tc.slot, consumed: tc.consumed}
			if err := refundUnusedCapacity(context.Background(), New(db), refundPosting); err != nil {
				t.Fatal(err)
			}
			if len(db.credits) != len(tc.want) || (len(tc.want) == 1 && db.credits[0] != tc.want[0]) {
				t.Fatalf("credits = %v, want %v", db.credits, tc.want)
			}
			if tc.want == nil && db.walletOK {
				t.Fatalf("a wallet was created for a posting with nothing to refund: %v", db.statements)
			}
		})
	}
}
