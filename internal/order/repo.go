package order

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
	ordersTable = goqu.T("orders")
)

type OrdersRepo struct {
	db      *pgxpool.Pool
	builder goqu.DialectWrapper
}

func NewRepository(db *pgxpool.Pool) *OrdersRepo {
	return &OrdersRepo{
		db:      db,
		builder: goqu.Dialect("postgres"),
	}
}

func (r *OrdersRepo) InsertOrder(ctx context.Context, userID uuid.UUID, orderNumber string) error {
	query, args, err := r.builder.Insert(ordersTable).
		Prepared(true).
		Rows(goqu.Record{
			"user_id":     userID,
			"number":      orderNumber,
			"status":      New,
			"uploaded_at": goqu.L("NOW()"),
		}).
		ToSQL()
	if err != nil {
		return fmt.Errorf("failed to build insert order query: %w", err)
	}

	if _, err = r.db.Exec(ctx, query, args...); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation {
			existing, lookupErr := r.getOrderByNumber(ctx, orderNumber)
			if lookupErr != nil {
				return fmt.Errorf("failed to get existing order: %w", lookupErr)
			}
			if existing.UserId == userID {
				return ErrOrderAlreadyUploadedByUser
			}
			return ErrOrderWasUploaded
		}
		return fmt.Errorf("failed to insert order: %w", err)
	}

	return nil
}

func (r *OrdersRepo) getOrderByNumber(ctx context.Context, orderNumber string) (Order, error) {
	query, args, err := r.builder.From(ordersTable).
		Select(
			goqu.C("id"),
			goqu.C("user_id"),
			goqu.C("number"),
			goqu.C("status"),
			goqu.L("COALESCE(accrual, 0)").As("accrual"),
			goqu.C("uploaded_at")).
		Prepared(true).
		Where(goqu.Ex{"number": orderNumber}).
		ToSQL()
	if err != nil {
		return Order{}, fmt.Errorf("failed to build get order query: %w", err)
	}

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return Order{}, fmt.Errorf("failed to query get order: %w", err)
	}

	order, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[Order])
	if err != nil {
		return Order{}, fmt.Errorf("failed to collect order: %w", err)
	}

	return order, nil
}

func (r *OrdersRepo) SelectOrders(ctx context.Context, userID uuid.UUID) ([]Order, error) {
	query, args, err := r.builder.From(ordersTable).
		Select(
			goqu.C("id"),
			goqu.C("user_id"),
			goqu.C("number"),
			goqu.C("status"),
			goqu.L("COALESCE(accrual, 0)").As("accrual"),
			goqu.C("uploaded_at"),
		).
		Prepared(true).
		Where(goqu.Ex{"user_id": userID}).
		Order(goqu.C("uploaded_at").Desc()).
		ToSQL()
	if err != nil {
		return nil, fmt.Errorf("failed to build get orders query: %w", err)
	}

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query get orders: %w", err)
	}

	orders, err := pgx.CollectRows(rows, pgx.RowToStructByName[Order])
	if err != nil {
		return nil, fmt.Errorf("failed to collect orders: %w", err)
	}

	return orders, nil
}
