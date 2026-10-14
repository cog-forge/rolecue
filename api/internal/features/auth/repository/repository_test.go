package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/cog-forge/rolecue/api/internal/features/auth/domain"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type roleDB struct {
	DBTX
	user                 domain.User
	updateErr, lookupErr error
}

func (d roleDB) QueryRow(_ context.Context, query string, _ ...any) pgx.Row {
	if query == chooseOnboardingRole {
		return roleRow{user: d.user, err: d.updateErr}
	}
	return roleRow{user: d.user, err: d.lookupErr}
}

type roleRow struct {
	user domain.User
	err  error
}

func (r roleRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	*dest[0].(*uuid.UUID) = r.user.ID
	*dest[1].(*string) = r.user.Email
	*dest[2].(*string) = r.user.FullName
	*dest[3].(*string) = r.user.Role
	*dest[4].(*bool) = r.user.EmailVerified
	*dest[5].(*bool) = r.user.IsLocked
	*dest[6].(**string) = r.user.Image
	*dest[7].(*bool) = r.user.OnboardingRoleSelected
	*dest[8].(*bool) = r.user.OnboardingCompleted
	return nil
}

func TestSelectRoleDistinguishesConflictFromIneligibility(t *testing.T) {
	selected := domain.User{ID: uuid.New(), Role: "recruiter", EmailVerified: true, OnboardingRoleSelected: true}
	for _, tc := range []struct {
		name   string
		role   string
		change func(*roleDB)
		code   apperror.Code
	}{
		{"different selected role", "candidate", nil, apperror.CodeOnboardingRoleConflict},
		{"already completed", "recruiter", func(d *roleDB) { d.user.OnboardingCompleted = true }, apperror.CodeOnboardingRoleConflict},
		{"locked with different role", "candidate", func(d *roleDB) { d.user.IsLocked = true }, apperror.CodeForbidden},
		{"unverified with different role", "candidate", func(d *roleDB) { d.user.EmailVerified = false }, apperror.CodeForbidden},
		{"admin", "candidate", func(d *roleDB) { d.user.Role = "admin" }, apperror.CodeForbidden},
		{"missing account", "candidate", func(d *roleDB) { d.lookupErr = pgx.ErrNoRows }, apperror.CodeForbidden},
		{"lookup failure", "candidate", func(d *roleDB) { d.lookupErr = errors.New("database unavailable") }, apperror.CodeInternal},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db := roleDB{user: selected, updateErr: pgx.ErrNoRows}
			if tc.change != nil {
				tc.change(&db)
			}
			_, err := NewRepository(db).SelectRole(context.Background(), selected.ID, tc.role)
			if !apperror.IsCode(err, tc.code) {
				t.Fatalf("error=%v, want code %s", err, tc.code)
			}
		})
	}
}

func TestSelectRoleSameRoleRetrySucceeds(t *testing.T) {
	user := domain.User{ID: uuid.New(), Role: "recruiter", EmailVerified: true, OnboardingRoleSelected: true}
	got, err := NewRepository(roleDB{user: user}).SelectRole(context.Background(), user.ID, user.Role)
	if err != nil || got.ID != user.ID || got.Role != user.Role || !got.OnboardingRoleSelected {
		t.Fatalf("user=%+v error=%v", got, err)
	}
}
