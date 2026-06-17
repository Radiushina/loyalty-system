package balance_test

import (
	"testing"
	"time"

	"github.com/Radiushina/loyalty-system/internal/balance"
	"github.com/Radiushina/loyalty-system/pkg/tests/containers/postgres"
	"github.com/doug-martin/goqu/v9"
	_ "github.com/doug-martin/goqu/v9/dialect/postgres"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

var (
	testBuilder      = goqu.Dialect("postgres")
	usersTable       = goqu.T("users")
	userBalanceTable = goqu.T("user_balance")
	withdrawalsTable = goqu.T("withdrawals")
	ordersTable      = goqu.T("orders")
)

func seedUserWithID(t *testing.T, pool *pgxpool.Pool, id uuid.UUID) {
	t.Helper()

	query, args, err := testBuilder.Insert(usersTable).
		Prepared(true).
		Rows(goqu.Record{
			"id":       id,
			"login":    id.String(),
			"password": "password",
		}).
		ToSQL()
	require.NoError(t, err)

	_, err = pool.Exec(t.Context(), query, args...)
	require.NoError(t, err)
}

func seedBalance(t *testing.T, pool *pgxpool.Pool, userID uuid.UUID, current, withdrawn float64) {
	t.Helper()

	query, args, err := testBuilder.Insert(userBalanceTable).
		Prepared(true).
		Rows(goqu.Record{
			"user_id":   userID,
			"current":   current,
			"withdrawn": withdrawn,
		}).
		ToSQL()
	require.NoError(t, err)

	_, err = pool.Exec(t.Context(), query, args...)
	require.NoError(t, err)
}

func seedWithdrawal(t *testing.T, pool *pgxpool.Pool, userID uuid.UUID, order string, sum float64, processedAt time.Time) {
	t.Helper()

	query, args, err := testBuilder.Insert(withdrawalsTable).
		Prepared(true).
		Rows(goqu.Record{
			"user_id":      userID,
			"order_number": order,
			"sum":          sum,
			"processed_at": processedAt,
		}).
		ToSQL()
	require.NoError(t, err)

	_, err = pool.Exec(t.Context(), query, args...)
	require.NoError(t, err)
}

func seedOrder(t *testing.T, pool *pgxpool.Pool, orderID, userID uuid.UUID, number string) {
	t.Helper()

	query, args, err := testBuilder.Insert(ordersTable).
		Prepared(true).
		Rows(goqu.Record{
			"id":          orderID,
			"user_id":     userID,
			"number":      number,
			"status":      "PROCESSED",
			"accrual":     0,
			"uploaded_at": goqu.L("NOW()"),
		}).
		ToSQL()
	require.NoError(t, err)

	_, err = pool.Exec(t.Context(), query, args...)
	require.NoError(t, err)
}

func TestRepo_CreditAccrual(t *testing.T) {
	t.Parallel()

	userID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	orderID := uuid.MustParse("00000000-0000-0000-0000-000000000010")

	tests := []struct {
		name    string
		amount  float64
		prepare func(t *testing.T, pool *pgxpool.Pool, repo *balance.Repo)
		want    balance.UserBalance
	}{
		{
			name:   "Creates balance on first accrual",
			amount: 729.98,
			prepare: func(t *testing.T, pool *pgxpool.Pool, _ *balance.Repo) {
				t.Helper()
				seedUserWithID(t, pool, userID)
				seedOrder(t, pool, orderID, userID, "79927398713")
			},
			want: balance.UserBalance{
				UserID:    userID,
				Current:   729.98,
				Withdrawn: 0,
			},
		},
		{
			name:   "Adds to existing balance",
			amount: 500,
			prepare: func(t *testing.T, pool *pgxpool.Pool, repo *balance.Repo) {
				t.Helper()
				seedUserWithID(t, pool, userID)
				seedOrder(t, pool, orderID, userID, "79927398713")
				seedBalance(t, pool, userID, 100, 0)
			},
			want: balance.UserBalance{
				UserID:    userID,
				Current:   600,
				Withdrawn: 0,
			},
		},
		{
			name:   "Second call with same order is idempotent",
			amount: 500,
			prepare: func(t *testing.T, pool *pgxpool.Pool, repo *balance.Repo) {
				t.Helper()
				seedUserWithID(t, pool, userID)
				seedOrder(t, pool, orderID, userID, "79927398713")
				require.NoError(t, repo.CreditAccrual(t.Context(), userID, orderID, 500))
			},
			want: balance.UserBalance{
				UserID:    userID,
				Current:   500,
				Withdrawn: 0,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			pool, _, _ := postgres.New(t)
			repo := balance.NewRepository(pool)

			tc.prepare(t, pool, repo)

			require.NoError(t, repo.CreditAccrual(t.Context(), userID, orderID, tc.amount))

			got, err := repo.SelectBalance(t.Context(), userID)
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestRepo_SelectBalance(t *testing.T) {
	t.Parallel()

	userID := uuid.MustParse("00000000-0000-0000-0000-000000000001")

	tests := []struct {
		name    string
		prepare func(t *testing.T, pool *pgxpool.Pool)
		want    balance.UserBalance
	}{
		{
			name: "Returns zero balance when row is missing",
			prepare: func(t *testing.T, pool *pgxpool.Pool) {
				t.Helper()
				seedUserWithID(t, pool, userID)
			},
			want: balance.UserBalance{
				UserID:    userID,
				Current:   0,
				Withdrawn: 0,
			},
		},
		{
			name: "Returns stored balance",
			prepare: func(t *testing.T, pool *pgxpool.Pool) {
				t.Helper()
				seedUserWithID(t, pool, userID)
				seedBalance(t, pool, userID, 1000, 42)
			},
			want: balance.UserBalance{
				UserID:    userID,
				Current:   1000,
				Withdrawn: 42,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			pool, _, _ := postgres.New(t)
			repo := balance.NewRepository(pool)

			tc.prepare(t, pool)

			got, err := repo.SelectBalance(t.Context(), userID)
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestRepo_WithdrawBalance(t *testing.T) {
	t.Parallel()

	userID := uuid.MustParse("00000000-0000-0000-0000-000000000001")

	tests := []struct {
		name    string
		opt     balance.WithdrawOpt
		prepare func(t *testing.T, pool *pgxpool.Pool, repo *balance.Repo)
		wantErr error
		assert  func(t *testing.T, repo *balance.Repo)
	}{
		{
			name: "Success withdraw updates balance and creates withdrawal",
			opt: balance.WithdrawOpt{
				Order: "79927398713",
				Sum:   751,
			},
			prepare: func(t *testing.T, pool *pgxpool.Pool, _ *balance.Repo) {
				t.Helper()
				seedUserWithID(t, pool, userID)
				seedBalance(t, pool, userID, 1000, 42)
			},
			wantErr: nil,
			assert: func(t *testing.T, repo *balance.Repo) {
				t.Helper()

				gotBalance, err := repo.SelectBalance(t.Context(), userID)
				require.NoError(t, err)
				require.Equal(t, float64(249), gotBalance.Current)
				require.Equal(t, float64(793), gotBalance.Withdrawn)

				withdrawals, err := repo.SelectWithdrawals(t.Context(), userID)
				require.NoError(t, err)
				require.Len(t, withdrawals, 1)
				require.Equal(t, "79927398713", withdrawals[0].Order)
				require.Equal(t, float64(751), withdrawals[0].Sum)
				require.False(t, withdrawals[0].ProcessedAt.IsZero())
			},
		},
		{
			name: "Insufficient funds when balance is too low",
			opt: balance.WithdrawOpt{
				Order: "33763345",
				Sum:   751,
			},
			prepare: func(t *testing.T, pool *pgxpool.Pool, _ *balance.Repo) {
				t.Helper()
				seedUserWithID(t, pool, userID)
				seedBalance(t, pool, userID, 100, 0)
			},
			wantErr: balance.ErrInsufficientFunds,
		},
		{
			name: "Insufficient funds when balance row is missing",
			opt: balance.WithdrawOpt{
				Order: "33763346",
				Sum:   100,
			},
			prepare: func(t *testing.T, pool *pgxpool.Pool, _ *balance.Repo) {
				t.Helper()
				seedUserWithID(t, pool, userID)
			},
			wantErr: balance.ErrInsufficientFunds,
		},
		{
			name: "Withdrawal already exists for order",
			opt: balance.WithdrawOpt{
				Order: "79927398713",
				Sum:   100,
			},
			prepare: func(t *testing.T, pool *pgxpool.Pool, repo *balance.Repo) {
				t.Helper()
				seedUserWithID(t, pool, userID)
				seedBalance(t, pool, userID, 1000, 0)

				err := repo.WithdrawBalance(t.Context(), userID, balance.WithdrawOpt{
					Order: "79927398713",
					Sum:   100,
				})
				require.NoError(t, err)
			},
			wantErr: balance.ErrWithdrawalAlreadyExists,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			pool, _, _ := postgres.New(t)
			repo := balance.NewRepository(pool)

			tc.prepare(t, pool, repo)

			err := repo.WithdrawBalance(t.Context(), userID, tc.opt)
			require.ErrorIs(t, err, tc.wantErr)
			if err != nil {
				return
			}

			if tc.assert != nil {
				tc.assert(t, repo)
			}
		})
	}
}

func TestRepo_SelectWithdrawals(t *testing.T) {
	t.Parallel()

	userID := uuid.MustParse("00000000-0000-0000-0000-000000000001")

	pool, _, _ := postgres.New(t)
	repo := balance.NewRepository(pool)

	seedUserWithID(t, pool, userID)

	older := time.Date(2024, 6, 1, 10, 0, 0, 0, time.UTC)
	newer := time.Date(2024, 6, 2, 10, 0, 0, 0, time.UTC)

	seedWithdrawal(t, pool, userID, "79927398713", 100, older)
	seedWithdrawal(t, pool, userID, "33763345", 200, newer)

	withdrawals, err := repo.SelectWithdrawals(t.Context(), userID)
	require.NoError(t, err)
	require.Len(t, withdrawals, 2)
	require.Equal(t, "33763345", withdrawals[0].Order)
	require.Equal(t, float64(200), withdrawals[0].Sum)
	require.Equal(t, newer, withdrawals[0].ProcessedAt.UTC())
	require.Equal(t, "79927398713", withdrawals[1].Order)
	require.Equal(t, float64(100), withdrawals[1].Sum)
	require.Equal(t, older, withdrawals[1].ProcessedAt.UTC())
}
