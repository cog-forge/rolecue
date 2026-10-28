//go:build integration

package tests

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/cog-forge/rolecue/api/internal/features/wallet/domain"
	"github.com/cog-forge/rolecue/api/internal/features/wallet/payos"
	"github.com/cog-forge/rolecue/api/internal/features/wallet/repository"
	"github.com/cog-forge/rolecue/api/internal/features/wallet/service"
	"github.com/cog-forge/rolecue/api/internal/pkg/ledger"
	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

// fakePayOS stands in for the gateway. A link is recorded even when CreatePaymentLink returns an error,
// because a timeout on our side can happen after PayOS created it.
type fakePayOS struct {
	mu        sync.Mutex
	createErr error
	links     map[int64]*payos.PaymentLink
}

func (f *fakePayOS) CreatePaymentLink(_ context.Context, req payos.CreateLinkRequest) (payos.CreateLinkResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.links[req.OrderCode] = &payos.PaymentLink{OrderCode: req.OrderCode, Amount: req.Amount, Status: payos.PaymentLinkStatusPending}
	if f.createErr != nil {
		return payos.CreateLinkResponse{}, f.createErr
	}
	return payos.CreateLinkResponse{CheckoutURL: fmt.Sprintf("https://pay.test/%d", req.OrderCode), OrderCode: req.OrderCode}, nil
}

func (f *fakePayOS) VerifyWebhookData(body []byte) (*payos.WebhookData, error) {
	var data payos.WebhookData
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, err
	}
	return &data, nil
}

func (f *fakePayOS) GetPaymentLink(_ context.Context, orderCode int64) (*payos.PaymentLink, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	link, ok := f.links[orderCode]
	if !ok {
		return nil, errors.New("payos: order not found")
	}
	copied := *link
	return &copied, nil
}

// settle sets what PayOS reports for the order from now on.
func (f *fakePayOS) settle(orderCode int64, status payos.PaymentLinkStatus, amountPaid int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.links[orderCode].Status = status
	f.links[orderCode].AmountPaid = amountPaid
}

type topUpEnv struct {
	ctx   context.Context
	pool  *pgxpool.Pool
	repo  *repository.Repository
	svc   *service.Service
	payos *fakePayOS
}

func newTopUpEnv(t *testing.T) *topUpEnv {
	t.Helper()
	ctx, pool := openIsolatedPool(t)
	repo := repository.NewRepository(pool)
	fake := &fakePayOS{links: map[int64]*payos.PaymentLink{}}
	return &topUpEnv{ctx: ctx, pool: pool, repo: repo, svc: service.NewService(repo, fake), payos: fake}
}

func (e *topUpEnv) newUser(t *testing.T) uuid.UUID {
	t.Helper()
	id := uuid.New()
	_, err := e.pool.Exec(e.ctx, `INSERT INTO public.users (id, name, email, role, email_verified)
		VALUES ($1, 'Top Up', $2, 'candidate', true)`, id, id.String()+"@example.com")
	require.NoError(t, err)
	return id
}

func (e *topUpEnv) newPackage(t *testing.T, priceVND, coins int32) uuid.UUID {
	t.Helper()
	id := uuid.New()
	_, err := e.pool.Exec(e.ctx, `INSERT INTO public.coin_packages (id, name, price, description, amount, currency)
		VALUES ($1, $2, $3, 'test', $4, 'VND')`, id, "Pack "+id.String()[:8], priceVND, coins)
	require.NoError(t, err)
	return id
}

// orders returns the order codes of the user's PayOS rows, oldest first.
func (e *topUpEnv) orders(t *testing.T, user uuid.UUID) []int64 {
	t.Helper()
	rows, err := e.pool.Query(e.ctx, `SELECT payos_order_code::bigint FROM public.transactions
		WHERE "to" = $1 AND payos_order_code IS NOT NULL ORDER BY created_at, id`, user.String())
	require.NoError(t, err)
	defer rows.Close()
	var codes []int64
	for rows.Next() {
		var code int64
		require.NoError(t, rows.Scan(&code))
		codes = append(codes, code)
	}
	require.NoError(t, rows.Err())
	return codes
}

func (e *topUpEnv) onlyOrder(t *testing.T, user uuid.UUID) int64 {
	t.Helper()
	codes := e.orders(t, user)
	require.Len(t, codes, 1)
	return codes[0]
}

// checkoutPaid runs a checkout that succeeds and has PayOS report the order as paid in full.
func (e *topUpEnv) checkoutPaid(t *testing.T, user, pkg uuid.UUID, priceVND int) int64 {
	t.Helper()
	resp, err := e.svc.CreateDepositCheckout(e.ctx, user, pkg, "https://app/return", "https://app/cancel")
	require.NoError(t, err)
	e.payos.settle(resp.OrderCode, payos.PaymentLinkStatusPaid, priceVND)
	return resp.OrderCode
}

func (e *topUpEnv) webhook(orderCode int64) error {
	return e.svc.HandlePayOSWebhook(e.ctx, []byte(fmt.Sprintf(`{"code":"00","success":true,"data":{"orderCode":%d}}`, orderCode)))
}

func (e *topUpEnv) status(t *testing.T, orderCode int64) string {
	t.Helper()
	var status string
	require.NoError(t, e.pool.QueryRow(e.ctx,
		`SELECT status::text FROM public.transactions WHERE payos_order_code = $1`, fmt.Sprint(orderCode)).Scan(&status))
	return status
}

// balance returns the wallet balance and whether the user has a wallet at all.
func (e *topUpEnv) balance(t *testing.T, user uuid.UUID) (int32, bool) {
	t.Helper()
	rows, err := e.pool.Query(e.ctx, `SELECT balance FROM public.wallets WHERE user_id = $1`, user)
	require.NoError(t, err)
	defer rows.Close()
	if !rows.Next() {
		require.NoError(t, rows.Err())
		return 0, false
	}
	var balance int32
	require.NoError(t, rows.Scan(&balance))
	return balance, true
}

func (e *topUpEnv) rowCount(t *testing.T, user uuid.UUID) int {
	t.Helper()
	var n int
	require.NoError(t, e.pool.QueryRow(e.ctx,
		`SELECT count(*) FROM public.transactions WHERE "to" = $1 OR "from" = $1`, user.String()).Scan(&n))
	return n
}

func TestTopUp_AmbiguousPayOSErrorThenPaidWebhookCreditsOnce(t *testing.T) {
	e := newTopUpEnv(t)
	user, pkg := e.newUser(t), e.newPackage(t, 50000, 100)

	e.payos.createErr = errors.New("context deadline exceeded")
	_, err := e.svc.CreateDepositCheckout(e.ctx, user, pkg, "", "")
	require.Error(t, err)
	require.True(t, apperror.IsCode(err, apperror.CodePayOSError))

	order := e.onlyOrder(t, user)
	require.Equal(t, "pending", e.status(t, order), "an ambiguous PayOS error must not close the order")

	// PayOS had created the link, and the user paid it.
	e.payos.settle(order, payos.PaymentLinkStatusPaid, 50000)
	require.NoError(t, e.webhook(order))

	balance, ok := e.balance(t, user)
	require.True(t, ok)
	require.Equal(t, int32(100), balance)
	require.Equal(t, "success", e.status(t, order))
	require.Equal(t, 1, e.rowCount(t, user), "the top-up must stay a single row")
}

func TestTopUp_DuplicateWebhookCreditsOnce(t *testing.T) {
	e := newTopUpEnv(t)
	user, pkg := e.newUser(t), e.newPackage(t, 50000, 100)
	order := e.checkoutPaid(t, user, pkg, 50000)

	require.NoError(t, e.webhook(order))
	require.NoError(t, e.webhook(order))

	var wg sync.WaitGroup
	errs := make([]error, 10)
	for i := range errs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs[i] = e.webhook(order)
		}()
	}
	wg.Wait()
	for _, err := range errs {
		require.NoError(t, err)
	}

	balance, _ := e.balance(t, user)
	require.Equal(t, int32(100), balance)
	require.Equal(t, 1, e.rowCount(t, user))
}

func TestTopUp_WebhookRacingSyncCreditsOnce(t *testing.T) {
	e := newTopUpEnv(t)
	user, pkg := e.newUser(t), e.newPackage(t, 50000, 100)

	const rounds = 5
	for round := 1; round <= rounds; round++ {
		order := e.checkoutPaid(t, user, pkg, 50000)

		start := make(chan struct{})
		var wg sync.WaitGroup
		var webhookErr, syncErr error
		wg.Add(2)
		go func() {
			defer wg.Done()
			<-start
			webhookErr = e.webhook(order)
		}()
		go func() {
			defer wg.Done()
			<-start
			_, syncErr = e.svc.SyncDepositStatus(e.ctx, user, order)
		}()
		close(start)
		wg.Wait()

		require.NoError(t, webhookErr)
		require.NoError(t, syncErr)
		require.Equal(t, "success", e.status(t, order))
		balance, _ := e.balance(t, user)
		require.Equal(t, int32(100*round), balance, "round %d credited more or less than once", round)
	}
}

func TestTopUp_NotPaidDoesNotCredit(t *testing.T) {
	cases := []struct {
		name       string
		payosState payos.PaymentLinkStatus
		amountPaid int
		wantStatus string
		wantErr    bool
	}{
		{"cancelled", payos.PaymentLinkStatusCancelled, 0, "cancelled", false},
		{"expired", payos.PaymentLinkStatusExpired, 0, "failed", false},
		{"underpaid status", payos.PaymentLinkStatusUnderpaid, 20000, "failed", false},
		{"paid below price", payos.PaymentLinkStatusPaid, 20000, "pending", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newTopUpEnv(t)
			user, pkg := e.newUser(t), e.newPackage(t, 50000, 100)
			resp, err := e.svc.CreateDepositCheckout(e.ctx, user, pkg, "", "")
			require.NoError(t, err)
			e.payos.settle(resp.OrderCode, tc.payosState, tc.amountPaid)

			err = e.webhook(resp.OrderCode)
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			require.Equal(t, tc.wantStatus, e.status(t, resp.OrderCode))
			balance, hasWallet := e.balance(t, user)
			require.Zero(t, balance)
			require.False(t, hasWallet, "an unpaid order must not even create a wallet")
			require.Equal(t, 1, e.rowCount(t, user))
		})
	}
}

func TestTopUp_MissingWalletIsCreatedOnSync(t *testing.T) {
	e := newTopUpEnv(t)
	user, pkg := e.newUser(t), e.newPackage(t, 50000, 100)
	order := e.checkoutPaid(t, user, pkg, 50000)

	_, hasWallet := e.balance(t, user)
	require.False(t, hasWallet)

	tx, err := e.svc.SyncDepositStatus(e.ctx, user, order)
	require.NoError(t, err)
	require.Equal(t, domain.StatusSuccess, tx.Status)

	balance, hasWallet := e.balance(t, user)
	require.True(t, hasWallet)
	require.Equal(t, int32(100), balance)
}

func TestTopUp_BalanceReconcilesWithRows(t *testing.T) {
	e := newTopUpEnv(t)
	user := e.newUser(t)
	small, large := e.newPackage(t, 50000, 100), e.newPackage(t, 100000, 250)

	require.NoError(t, ledger.Credit(e.ctx, e.pool, user, 30, "Refund of unused interview capacity"))
	require.NoError(t, ledger.Debit(e.ctx, e.pool, user, 5, "Practice interview"))
	require.NoError(t, e.webhook(e.checkoutPaid(t, user, small, 50000)))
	require.NoError(t, e.webhook(e.checkoutPaid(t, user, large, 100000)))
	cancelled, err := e.svc.CreateDepositCheckout(e.ctx, user, small, "", "")
	require.NoError(t, err)
	e.payos.settle(cancelled.OrderCode, payos.PaymentLinkStatusCancelled, 0)
	require.NoError(t, e.webhook(cancelled.OrderCode))

	var fromRows int64
	require.NoError(t, e.pool.QueryRow(e.ctx, `
		SELECT COALESCE(SUM(CASE
			WHEN currency = 'VND' AND status = 'success' AND "to" = $1 THEN amount
			WHEN currency = 'COIN' AND "to" = $1 THEN amount
			WHEN currency = 'COIN' AND "from" = $1 THEN -amount
			ELSE 0 END), 0)
		FROM public.transactions`, user.String()).Scan(&fromRows))

	balance, _ := e.balance(t, user)
	require.Equal(t, int32(30-5+100+250), balance)
	require.Equal(t, int64(balance), fromRows, "wallet balance must equal what the rows add up to")
}

// A top-up must read as a top-up everywhere: no extra COIN row from the system that the admin queries and
// the user history would call a refund.
func TestTopUp_AdminAndHistoryClassificationUnchanged(t *testing.T) {
	e := newTopUpEnv(t)
	user, pkg := e.newUser(t), e.newPackage(t, 50000, 100)

	// One genuine refund and one spend sit next to the top-up, so every class has a known count.
	require.NoError(t, ledger.Credit(e.ctx, e.pool, user, 7, "Refund of unused interview capacity"))
	require.NoError(t, e.webhook(e.checkoutPaid(t, user, pkg, 50000)))
	require.NoError(t, ledger.Debit(e.ctx, e.pool, user, 3, "Practice interview"))

	list := func(txType string) domain.AdminTransactionPage {
		page, err := e.svc.ListBillingTransactions(e.ctx, domain.AdminTransactionFilter{Type: txType})
		require.NoError(t, err)
		return page
	}
	topups := list("topup")
	require.EqualValues(t, 1, topups.Total)
	require.Equal(t, "topup", topups.Items[0].Type)
	require.Equal(t, domain.StatusSuccess, topups.Items[0].Status)
	require.Equal(t, "VND", topups.Items[0].Currency)
	require.EqualValues(t, 50000, topups.Items[0].PaidVND)
	require.EqualValues(t, 100, topups.Items[0].CoinAmount)

	refunds := list("refund")
	require.EqualValues(t, 1, refunds.Total, "the top-up must not add a refund row")
	require.Contains(t, refunds.Items[0].Description, "Refund")
	require.EqualValues(t, 1, list("spend").Total)

	summary, err := e.svc.GetBillingSummary(e.ctx, domain.AdminTransactionFilter{})
	require.NoError(t, err)
	require.EqualValues(t, 50000, summary.TotalRevenue)
	require.EqualValues(t, 100, summary.CoinsSold)
	require.EqualValues(t, 7, summary.CoinsRefunded)
	require.EqualValues(t, 3, summary.CoinsSpent)

	history, total, err := e.svc.ListTransactions(e.ctx, user, 1, 20)
	require.NoError(t, err)
	require.EqualValues(t, 3, total)
	var topup *domain.Transaction
	for i := range history {
		if history[i].Currency == "VND" {
			topup = &history[i]
		}
	}
	require.NotNil(t, topup)
	// The history screen calls a row a refund when it has no sender and no order code.
	require.Nil(t, topup.From)
	require.NotNil(t, topup.PayOSOrderCode)
}

func TestTopUp_CheckoutRowIsFromSystem(t *testing.T) {
	e := newTopUpEnv(t)
	user, pkg := e.newUser(t), e.newPackage(t, 50000, 100)
	_, err := e.svc.CreateDepositCheckout(e.ctx, user, pkg, "", "")
	require.NoError(t, err)

	var from string
	require.NoError(t, e.pool.QueryRow(e.ctx,
		`SELECT "from" FROM public.transactions WHERE "to" = $1`, user.String()).Scan(&from))
	require.Equal(t, ledger.Counterparty, from)
}

func TestTopUp_PayOSOrderCodeIsUnique(t *testing.T) {
	e := newTopUpEnv(t)
	user := e.newUser(t)

	// The migration reverses cleanly and applies again.
	indexCount := func() int {
		var n int
		require.NoError(t, e.pool.QueryRow(e.ctx,
			`SELECT count(*) FROM pg_indexes WHERE indexname = 'transactions_payos_order_code_key'`).Scan(&n))
		return n
	}
	migrationsDir := filepath.Join("..", "..", "..", "..", "migrations")
	run := func(file string) {
		sql, err := os.ReadFile(filepath.Join(migrationsDir, file))
		require.NoError(t, err)
		_, err = e.pool.Exec(e.ctx, string(sql))
		require.NoError(t, err, file)
	}
	require.Equal(t, 1, indexCount())
	run("000015_payos_order_code_unique.down.sql")
	require.Equal(t, 0, indexCount())
	run("000015_payos_order_code_unique.up.sql")
	require.Equal(t, 1, indexCount())

	insert := func(code any) error {
		_, err := e.pool.Exec(e.ctx, `INSERT INTO public.transactions ("from", "to", amount, currency, status, payos_order_code)
			VALUES ('system', $1, 10, 'VND', 'pending', $2)`, user.String(), code)
		return err
	}
	require.NoError(t, insert("123456789012"))
	var pgErr *pgconn.PgError
	require.ErrorAs(t, insert("123456789012"), &pgErr)
	require.Equal(t, "23505", pgErr.Code)

	// Rows without an order code, such as the internal COIN movements, are not restricted.
	require.NoError(t, insert(nil))
	require.NoError(t, insert(nil))
	require.NoError(t, insert(""))
	require.NoError(t, insert(""))
}
