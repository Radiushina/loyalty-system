package balance

import (
	"context"
	"errors"
	"fmt"

	"github.com/doug-martin/goqu/v9"
	_ "github.com/doug-martin/goqu/v9/dialect/postgres"
	"github.com/google/uuid"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	balanceTable         = goqu.T("user_balance")
	withdrawalsTable     = goqu.T("withdrawals")
	balanceAccrualsTable = goqu.T("balance_accruals")
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
func (r *Repo) WithdrawBalance(ctx context.Context, userID uuid.UUID, opt WithdrawOpt) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	// 1) В таблице user_balance current(текущая сумма баллов лояльности) уменьшится,
	// а withdrawn (сумма списания за весь период) увеличится.
	updateQuery, updateArgs, err := r.builder.Update(balanceTable).
		Prepared(true).
		Set(goqu.Record{
			"current":   goqu.L("current - ?", opt.Sum),
			"withdrawn": goqu.L("withdrawn + ?", opt.Sum),
		}).
		Where(goqu.Ex{"user_id": userID}).
		Where(goqu.L("current >= ?", opt.Sum)).
		ToSQL()
	if err != nil {
		return fmt.Errorf("failed to build update user balance query: %w", err)
	}

	tag, err := tx.Exec(ctx, updateQuery, updateArgs...)
	if err != nil {
		return fmt.Errorf("failed to update user balance: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrInsufficientFunds
	}

	// 2) В таблицу withdrawals добавится запись о списании
	query, args, err := r.builder.Insert(withdrawalsTable).
		Rows(goqu.Record{
			"user_id":      userID,
			"order_number": opt.Order,
			"sum":          opt.Sum,
			"processed_at": goqu.L("NOW()"),
		}).
		Prepared(true).
		ToSQL()
	if err != nil {
		return fmt.Errorf("failed to build insert withdrawal query: %w", err)
	}

	if _, err = tx.Exec(ctx, query, args...); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation {
			return ErrWithdrawalAlreadyExists
		}
		return fmt.Errorf("failed to insert withdrawal: %w", err)
	}

	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// CreditAccrual идемпотентно начисляет баллы за заказ: повторный вызов с тем же orderID не меняет баланс.
func (r *Repo) CreditAccrual(ctx context.Context, userID, orderID uuid.UUID, amount float64) error {
	if amount <= 0 {
		return nil
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	insertQuery, insertArgs, err := r.builder.Insert(balanceAccrualsTable).
		Prepared(true).
		Rows(goqu.Record{
			"order_id":    orderID,
			"user_id":     userID,
			"amount":      amount,
			"credited_at": goqu.L("NOW()"),
		}).
		ToSQL()
	if err != nil {
		return fmt.Errorf("failed to build insert balance accrual query: %w", err)
	}

	tag, err := tx.Exec(ctx, insertQuery, insertArgs...)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation {
			return nil
		}
		return fmt.Errorf("failed to insert balance accrual: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return nil
	}

	upsertQuery, upsertArgs, err := r.builder.Insert(balanceTable).
		Prepared(true).
		Rows(goqu.Record{
			"user_id":   userID,
			"current":   amount,
			"withdrawn": 0,
		}).
		OnConflict(goqu.DoUpdate("user_id", goqu.Record{
			"current": goqu.L("user_balance.current + ?", amount),
		})).
		ToSQL()
	if err != nil {
		return fmt.Errorf("failed to build upsert user balance query: %w", err)
	}

	if _, err = tx.Exec(ctx, upsertQuery, upsertArgs...); err != nil {
		return fmt.Errorf("failed to upsert user balance: %w", err)
	}

	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

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
