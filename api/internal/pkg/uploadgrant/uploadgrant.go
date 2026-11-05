// Package uploadgrant binds an uploaded PDF's storage key to the user it was issued to and makes it
// single-use. Object keys are bare UUIDs, so without this anyone who learns a key could attach
// another user's upload and read it back through the document link.
//
// Keys uploaded before upload_grants existed have no row and are refused: the upload windows are
// minutes long (a 5-minute presigned PUT, or an immediate create after the server-side upload), so
// a compatibility path is not worth the hole it would leave. The client repeats the upload.
package uploadgrant

import (
	"context"

	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
)

const (
	PurposeRoleProfilePDF = "role_profile_pdf"
	PurposeJobPostingPDF  = "job_posting_pdf"
)

// ErrUnavailable means the upload cannot be attached by this user.
var ErrUnavailable = apperror.New(apperror.CodeInvalidPDF, "This upload was not found or was already used. Upload the PDF again.")

// Execer is satisfied by pgx pools and transactions.
type Execer interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// Record stores a grant for a freshly issued key.
func Record(ctx context.Context, db Execer, owner, key uuid.UUID, purpose string) error {
	_, err := db.Exec(ctx, `INSERT INTO public.upload_grants (key, owner_id, purpose) VALUES ($1, $2, $3)`, key, owner, purpose)
	return err
}

// Consume claims the key in one statement. A key that is foreign, unknown, issued for another
// purpose or already used all fail the same way, so the response does not reveal which.
func Consume(ctx context.Context, db Execer, owner uuid.UUID, key, purpose string) error {
	id, err := uuid.Parse(key)
	if err != nil {
		return ErrUnavailable
	}
	tag, err := db.Exec(ctx, `UPDATE public.upload_grants SET consumed_at = CURRENT_TIMESTAMP
 WHERE key = $1 AND owner_id = $2 AND purpose = $3 AND consumed_at IS NULL`, id, owner, purpose)
	if err != nil {
		return apperror.Wrap(apperror.CodeInternal, "Could not verify the upload.", err)
	}
	if tag.RowsAffected() != 1 {
		return ErrUnavailable
	}
	return nil
}
