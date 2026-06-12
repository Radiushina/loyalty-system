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

func (r *OrdersRepo) InsertOrder(ctx context.Context, userID uuid.UUID, orderNumber string) (uuid.UUID, error) {
	query, args, err := r.builder.Insert(ordersTable).
		Prepared(true).
		Rows(goqu.Record{
			"user_id":     userID,
			"number":      orderNumber,
			"status":      New,
			"uploaded_at": goqu.L("NOW()"),
		}).
		Returning(goqu.C("id")).
		ToSQL()
	if err != nil {
		return uuid.Nil, fmt.Errorf("failed to build insert order query: %w", err)
	}

	var orderID uuid.UUID
	if err = r.db.QueryRow(ctx, query, args...).Scan(&orderID); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation {
			existing, lookupErr := r.getOrderByNumber(ctx, orderNumber)
			if lookupErr != nil {
				return uuid.Nil, fmt.Errorf("failed to get existing order: %w", lookupErr)
			}
			if existing.UserId == userID {
				return uuid.Nil, ErrOrderAlreadyUploadedByUser
			}
			return uuid.Nil, ErrOrderWasUploaded
		}
		return uuid.Nil, fmt.Errorf("failed to insert order: %w", err)
	}

	return orderID, nil
}

func (r *OrdersRepo) GetOrderByID(ctx context.Context, orderID uuid.UUID) (Order, error) {
	query, args, err := r.builder.From(ordersTable).
		Select(
			goqu.C("id"),
			goqu.C("user_id"),
			goqu.C("number"),
			goqu.C("status"),
			goqu.L("COALESCE(accrual, 0)").As("accrual"),
			goqu.C("uploaded_at")).
		Prepared(true).
		Where(goqu.Ex{"id": orderID}).
		ToSQL()
	if err != nil {
		return Order{}, fmt.Errorf("failed to build get order by id query: %w", err)
	}

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return Order{}, fmt.Errorf("failed to query get order by id: %w", err)
	}

	order, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[Order])
	if err != nil {
		return Order{}, fmt.Errorf("failed to collect order by id: %w", err)
	}

	return order, nil
}

func (r *OrdersRepo) SelectPendingOrders(ctx context.Context, limit int) ([]Order, error) {
	if limit <= 0 {
		limit = 1
	}

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
		Where(goqu.Ex{"status": []Status{New, Processing}}).
		Order(goqu.C("uploaded_at").Asc()).
		Limit(uint(limit)).
		ToSQL()
	if err != nil {
		return nil, fmt.Errorf("failed to build select pending orders query: %w", err)
	}

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query pending orders: %w", err)
	}

	orders, err := pgx.CollectRows(rows, pgx.RowToStructByName[Order])
	if err != nil {
		return nil, fmt.Errorf("failed to collect pending orders: %w", err)
	}

	return orders, nil
}

func (r *OrdersRepo) UpdateOrderAccrual(ctx context.Context, orderID uuid.UUID, status Status, accrual float32) error {
	query, args, err := r.builder.Update(ordersTable).
		Prepared(true).
		Set(goqu.Record{
			"status":  status,
			"accrual": accrual,
		}).
		Where(goqu.Ex{"id": orderID}).
		ToSQL()
	if err != nil {
		return fmt.Errorf("failed to build update order accrual query: %w", err)
	}

	if _, err = r.db.Exec(ctx, query, args...); err != nil {
		return fmt.Errorf("failed to update order accrual: %w", err)
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
