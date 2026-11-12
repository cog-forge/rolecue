// Package dbnum converts the numeric(5,2) score columns (KAN-335) to the float64 the domain uses. Scores have
// two decimals and at most 100, so a float64 holds every stored value exactly enough to print it again.
package dbnum

import "github.com/jackc/pgx/v5/pgtype"

// Float returns the value of a nullable numeric column, or nil when it is NULL (or not a finite number).
func Float(value pgtype.Numeric) *float64 {
	if !value.Valid {
		return nil
	}
	f, err := value.Float64Value()
	if err != nil || !f.Valid {
		return nil
	}
	v := f.Float64
	return &v
}
