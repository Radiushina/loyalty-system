package balance_test

import (
	"testing"
	"time"

	"github.com/Radiushina/loyalty-system/internal/balance"
	"github.com/Radiushina/loyalty-system/pkg/tests/containers/postgres"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func seedUserWithID(t *testing.T, pool *pgxpool.Pool, id uuid.UUID) {
	t.Helper()

	_, err := pool.Exec(
		t.Context(),
		`INSERT INTO users (id, login, password) VALUES ($1, $2, $3)`,
		id,
		id.String(),
		"password",
	)
	require.NoError(t, err)
}

func seedBalance(t *testing.T, pool *pgxpool.Pool, userID uuid.UUID, current, withdrawn float64) {
	t.Helper()

	_, err := pool.Exec(
		t.Context(),
		`INSERT INTO user_balance (user_id, current, withdrawn) VALUES ($1, $2, $3)`,
		userID,
		current,
		withdrawn,
	)
	require.NoError(t, err)
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
	seedBalance(t, pool, userID, 1000, 0)

	first := balance.WithdrawOpt{Order: "79927398713", Sum: 100}
	second := balance.WithdrawOpt{Order: "33763345", Sum: 200}

	require.NoError(t, repo.WithdrawBalance(t.Context(), userID, first))
	time.Sleep(10 * time.Millisecond)
	require.NoError(t, repo.WithdrawBalance(t.Context(), userID, second))

	withdrawals, err := repo.SelectWithdrawals(t.Context(), userID)
	require.NoError(t, err)
	require.Len(t, withdrawals, 2)
	require.Equal(t, "33763345", withdrawals[0].Order)
	require.Equal(t, float64(200), withdrawals[0].Sum)
	require.Equal(t, "79927398713", withdrawals[1].Order)
	require.Equal(t, float64(100), withdrawals[1].Sum)
}
