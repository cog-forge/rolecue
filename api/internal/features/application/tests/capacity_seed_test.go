//go:build integration

package tests

import (
	"context"
	"io"
	"testing"

	"github.com/cog-forge/rolecue/api/internal/features/application/repository"
	"github.com/cog-forge/rolecue/api/internal/features/application/service"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// applyCVStore stands in for object storage on the apply path. It hands out a fresh key per upload
// and keeps the link signing of stubCVLinker.
type applyCVStore struct {
	stubCVLinker
}

func (s *applyCVStore) Upload(_ context.Context, _ io.ReadSeeker, _ string) (string, error) {
	return "cvs/" + uuid.NewString() + ".pdf", nil
}

func (s *applyCVStore) Delete(_ context.Context, _ string) error { return nil }

// validCV is the smallest content domain.ValidateCV accepts.
var validCV = []byte("%PDF-1.4 capacity test")

// newServiceWithCVStore builds the service with an apply-capable CV store, so Apply can run.
func newServiceWithCVStore(t *testing.T, pool *pgxpool.Pool) *service.Service {
	t.Helper()
	svc, err := service.New(repository.NewRepository(pool), &applyCVStore{})
	if err != nil {
		t.Fatal(err)
	}
	return svc
}

// wallet gives the user a wallet with the balance. The wallet row and every coin row the user is
// part of are removed afterwards, before the user itself.
func (s *seed) wallet(userID uuid.UUID, balance int32) {
	s.t.Helper()
	s.exec(`INSERT INTO public.wallets (user_id, balance) VALUES ($1, $2)
		ON CONFLICT (user_id) DO UPDATE SET balance = EXCLUDED.balance`, userID, balance)
	s.later(`DELETE FROM public.wallets WHERE user_id = $1`, userID)
	s.later(`DELETE FROM public.transactions WHERE "from" = $1 OR "to" = $1`, userID.String())
}

// setWalletBalance overwrites the balance of a wallet the test already created.
func (s *seed) setWalletBalance(userID uuid.UUID, balance int32) {
	s.t.Helper()
	s.exec(`UPDATE public.wallets SET balance = $2 WHERE user_id = $1`, userID, balance)
}

// balance reads the wallet balance of the user, failing when there is no wallet.
func (s *seed) balance(userID uuid.UUID) int32 {
	s.t.Helper()
	var balance int32
	if err := s.pool.QueryRow(s.ctx, `SELECT balance FROM public.wallets WHERE user_id = $1`, userID).Scan(&balance); err != nil {
		s.t.Fatalf("wallet of %s: %v", userID, err)
	}
	return balance
}

// walletRows counts the wallet rows of the given users. A Candidate must have none.
func (s *seed) walletRows(userIDs ...uuid.UUID) int {
	s.t.Helper()
	total := 0
	for _, id := range userIDs {
		var n int
		if err := s.pool.QueryRow(s.ctx, `SELECT count(*) FROM public.wallets WHERE user_id = $1`, id).Scan(&n); err != nil {
			s.t.Fatal(err)
		}
		total += n
	}
	return total
}

// ledgerRow is one transactions row written by the coin ledger.
type ledgerRow struct {
	amount   int32
	from, to string
	currency string
	status   string
}

// postingRows returns the coin rows whose description names the posting. Funding and refund both
// name it, so this is every coin movement of that posting.
func (s *seed) postingRows(postingID uuid.UUID) []ledgerRow {
	s.t.Helper()
	rows, err := s.pool.Query(s.ctx,
		`SELECT amount, "from", "to", currency, status::text FROM public.transactions
		 WHERE description LIKE '%' || $1 || '%'`, postingID.String())
	if err != nil {
		s.t.Fatal(err)
	}
	defer rows.Close()
	var out []ledgerRow
	for rows.Next() {
		var r ledgerRow
		if err := rows.Scan(&r.amount, &r.from, &r.to, &r.currency, &r.status); err != nil {
			s.t.Fatal(err)
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		s.t.Fatal(err)
	}
	return out
}

// hasRow reports whether a row with exactly these parties and amount is in rows.
func hasRow(rows []ledgerRow, from, to string, amount int32) bool {
	for _, r := range rows {
		if r.from == from && r.to == to && r.amount == amount {
			return true
		}
	}
	return false
}

// ledgerNet is the coins the user received minus the coins they paid, over all their rows. It must
// equal the change of their wallet balance.
func (s *seed) ledgerNet(userID uuid.UUID) int64 {
	s.t.Helper()
	var net int64
	err := s.pool.QueryRow(s.ctx,
		`SELECT COALESCE(SUM(CASE WHEN "to" = $1 THEN amount ELSE 0 END), 0)
		      - COALESCE(SUM(CASE WHEN "from" = $1 THEN amount ELSE 0 END), 0)
		 FROM public.transactions WHERE "to" = $1 OR "from" = $1`, userID.String()).Scan(&net)
	if err != nil {
		s.t.Fatal(err)
	}
	return net
}

// postingState reads the status and interview slot of a posting straight from the table.
func (s *seed) postingState(postingID uuid.UUID) (string, *int32) {
	s.t.Helper()
	var status string
	var slot *int32
	if err := s.pool.QueryRow(s.ctx,
		`SELECT status::text, interview_slot FROM public.job_postings WHERE id = $1`, postingID).Scan(&status, &slot); err != nil {
		s.t.Fatal(err)
	}
	return status, slot
}

// setApplicationStatus moves an Application to a status, as the interview outcome will (KAN-115).
func (s *seed) setApplicationStatus(applicationID uuid.UUID, status string) {
	s.t.Helper()
	s.exec(`UPDATE public.applications SET status = $2::public.application_status WHERE id = $1`, applicationID, status)
}

// interview adds the recruitment interview of an Application: the row KAN-117 creates at the first
// successful start, and the only thing that consumes a slot (KTD5). It needs a Role Profile, which
// needs a Job Description. All three are removed afterwards, interview first.
func (s *seed) interview(applicationID uuid.UUID) uuid.UUID {
	s.t.Helper()
	var candidateID, levelID uuid.UUID
	if err := s.pool.QueryRow(s.ctx, `SELECT candidate_id FROM public.applications WHERE id = $1`, applicationID).Scan(&candidateID); err != nil {
		s.t.Fatal(err)
	}
	if err := s.pool.QueryRow(s.ctx, `SELECT id FROM public.levels ORDER BY id LIMIT 1`).Scan(&levelID); err != nil {
		s.t.Fatalf("no seniority level seeded: %v", err)
	}
	jdID, roleID, interviewID := uuid.New(), uuid.New(), uuid.New()
	s.exec(`INSERT INTO public.job_descriptions (id, content_path, level_id) VALUES ($1, $2, $3)`,
		jdID, "job-postings/"+jdID.String()+".pdf", levelID)
	s.later(`DELETE FROM public.job_descriptions WHERE id = $1`, jdID)
	s.exec(`INSERT INTO public.role_profiles (id, jd_id, candidate_id, title) VALUES ($1, $2, $3, 'capacity test')`,
		roleID, jdID, candidateID)
	s.later(`DELETE FROM public.role_profiles WHERE id = $1`, roleID)
	s.exec(`INSERT INTO public.interviews (id, role_profile_id, type, application_id) VALUES ($1, $2, 'recruitment', $3)`,
		interviewID, roleID, applicationID)
	s.later(`DELETE FROM public.interviews WHERE id = $1`, interviewID)
	return interviewID
}
