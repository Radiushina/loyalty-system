package user

import (
	"context"
	"errors"
	"fmt"

	"github.com/doug-martin/goqu/v9"
	_ "github.com/doug-martin/goqu/v9/dialect/postgres"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	usersTable = goqu.T("users")
)

type UsersRepo struct {
	db      *pgxpool.Pool
	builder goqu.DialectWrapper
}

func NewRepository(db *pgxpool.Pool) *UsersRepo {
	return &UsersRepo{
		db:      db,
		builder: goqu.Dialect("postgres"),
	}
}

func (r *UsersRepo) CreateUser(ctx context.Context, login, password string) (User, error) {
	query, args, err := r.builder.Insert(usersTable).
		Prepared(true).
		Rows(goqu.Record{
			"login":    login,
			"password": goqu.L("crypt(?, gen_salt('bf'))", password),
		}).
		Returning(goqu.C("id"), goqu.C("login")).
		ToSQL()
	if err != nil {
		return User{}, fmt.Errorf("failed to build insert user query: %w", err)
	}

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return User{}, fmt.Errorf("failed to query insert user: %w", err)
	}

	u, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[User])
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation {
			return User{}, fmt.Errorf("%w: %w", ErrUserAlreadyExists, err)
		}
		return User{}, fmt.Errorf("failed to create user: %w", err)
	}
	return u, nil
}

func (r *UsersRepo) GetByLogin(ctx context.Context, login, password string) (User, error) {
	existsQuery, existsArgs, err := r.builder.From(usersTable).
		Select(goqu.L("1")).
		Prepared(true).
		Where(goqu.Ex{"login": login}).
		ToSQL()
	if err != nil {
		return User{}, fmt.Errorf("failed to build find user query: %w", err)
	}

	existsRows, err := r.db.Query(ctx, existsQuery, existsArgs...)
	if err != nil {
		return User{}, fmt.Errorf("failed to query find user: %w", err)
	}

	if _, err = pgx.CollectOneRow(existsRows, pgx.RowTo[int32]); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return User{}, fmt.Errorf("%w: %w", ErrUserNotFound, err)
		}
		return User{}, fmt.Errorf("failed to find user: %w", err)
	}

	query, args, err := r.builder.From(usersTable).
		Select(goqu.C("id"), goqu.C("login")).
		Prepared(true).
		Where(goqu.Ex{"login": login}).
		Where(goqu.L("password = crypt(?, password)", password)).
		ToSQL()
	if err != nil {
		return User{}, fmt.Errorf("failed to build get user query: %w", err)
	}

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return User{}, fmt.Errorf("failed to query get user: %w", err)
	}

	u, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[User])
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return User{}, fmt.Errorf("%w: %w", ErrInvalidCredentials, err)
		}
		return User{}, fmt.Errorf("failed to get user: %w", err)
	}

	return u, nil
}
