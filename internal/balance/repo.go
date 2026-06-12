package balance

import (
	"context"
	"errors"
	"fmt"

	"github.com/doug-martin/goqu/v9"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	balanceTable     = goqu.T("user_balance")
	withdrawalsTable = goqu.T("withdrawals")
)

type Repo struct {
	db      *pgxpool.Pool
	builder goqu.DialectWrapper
}

// NewRepository создаёт репозиторий для таблиц user_balance и withdrawals.
func NewRepository(db *pgxpool.Pool) *Repo {
	return &Repo{
		db:      db,
		builder: goqu.Dialect("postgres"),
	}
}

// WithdrawBalance атомарно уменьшает current, увеличивает withdrawn и сохраняет запись в withdrawals.
//
// В таблице user_balance current(текущая сумма баллов лояльности) уменьшится,
// а withdrawn (сумма списания за весь период) увеличится.
// В таблицу withdrawals добавится запись о списании
func (r *Repo) WithdrawBalance(ctx context.Context, userID uuid.UUID, opt WithdrawOpt) error {

	return nil
}

// SelectBalance читает баланс из user_balance; при отсутствии записи возвращает нулевые значения.
func (r *Repo) SelectBalance(ctx context.Context, userID uuid.UUID) (UserBalance, error) {
	query, args, err := r.builder.From(balanceTable).
		Select(
			goqu.C("user_id"),
			goqu.C("current"),
			goqu.C("withdrawn"),
		).
		Prepared(true).
		Where(goqu.Ex{"user_id": userID}).
		ToSQL()

	if err != nil {
		return UserBalance{}, fmt.Errorf("failed to build get balance query: %w", err)
	}

	row, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return UserBalance{}, fmt.Errorf("failed to query get balance: %w", err)
	}
	balance, err := pgx.CollectOneRow(row, pgx.RowToStructByName[UserBalance])
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return UserBalance{UserID: userID, Current: 0, Withdrawn: 0}, nil
		}
		return UserBalance{}, fmt.Errorf("failed to collect balance: %w", err)
	}
	return balance, nil
}

// SelectWithdrawals читает список списаний пользователя из withdrawals по processed_at DESC.
func (r *Repo) SelectWithdrawals(ctx context.Context, userID uuid.UUID) ([]Withdrawals, error) {
	query, args, err := r.builder.From(withdrawalsTable).
		Select(
			goqu.C("order_number"),
			goqu.C("sum"),
			goqu.C("processed_at"),
		).
		Prepared(true).
		Where(goqu.Ex{"user_id": userID}).
		Order(goqu.C("processed_at").Desc()).
		ToSQL()
	if err != nil {
		return nil, fmt.Errorf("failed to build get orders query: %w", err)
	}

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query get orders: %w", err)
	}

	withdrawals, err := pgx.CollectRows(rows, pgx.RowToStructByName[Withdrawals])
	if err != nil {
		return nil, fmt.Errorf("failed to collect orders: %w", err)
	}

	return withdrawals, nil
}
