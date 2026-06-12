package order_test

import (
	"os"
	"testing"
	"time"

	"github.com/Radiushina/loyalty-system/internal/order"
	"github.com/Radiushina/loyalty-system/pkg/tests/containers/postgres"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

var migrationsPath = os.Getenv("MIGRATIONS_PATH")

// seedUserWithID создаёт пользователя с заданным id.
// Нужен только из-за FK orders.user_id → users.id; user-репозиторий здесь не тестируем.
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

func TestRepo_Insert(t *testing.T) {
	t.Parallel()

	userA := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	userB := uuid.MustParse("00000000-0000-0000-0000-000000000002")

	tests := []struct {
		name        string
		userID      uuid.UUID
		orderNumber string
		prepare     func(t *testing.T, pool *pgxpool.Pool, repo *order.OrdersRepo)
		wantErr     error
	}{
		{
			name:        "Success insert order",
			userID:      userA,
			orderNumber: "33763345",
			prepare: func(t *testing.T, pool *pgxpool.Pool, _ *order.OrdersRepo) {
				t.Helper()

				// Создаём пользователя — без этого InsertOrder упадёт на FK.
				seedUserWithID(t, pool, userA)
			},
			wantErr: nil,
		},
		{
			name:        "Order already uploaded by user",
			userID:      userA,
			orderNumber: "33763345",
			prepare: func(t *testing.T, pool *pgxpool.Pool, repo *order.OrdersRepo) {
				t.Helper()

				// 1. Пользователь уже загрузил этот номер заказа.
				seedUserWithID(t, pool, userA)
				_, err := repo.InsertOrder(t.Context(), userA, "33763345")
				require.NoError(t, err)
			},
			wantErr: order.ErrOrderAlreadyUploadedByUser,
		},
		{
			name:        "Order uploaded by another user",
			userID:      userB,
			orderNumber: "33763345",
			prepare: func(t *testing.T, pool *pgxpool.Pool, repo *order.OrdersRepo) {
				t.Helper()

				// 1. userA уже загрузил номер заказа.
				seedUserWithID(t, pool, userA)
				_, err := repo.InsertOrder(t.Context(), userA, "33763345")
				require.NoError(t, err)

				// 2. userB пытается загрузить тот же номер.
				seedUserWithID(t, pool, userB)
			},
			wantErr: order.ErrOrderWasUploaded,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// 1. Поднимаем Postgres и накатываем миграции.
			pool, _, _ := postgres.New(t)
			repo := order.NewRepository(pool)

			// 2. Готовим данные для сценария (пользователи, уже существующие заказы).
			tc.prepare(t, pool, repo)

			// 3. Вызываем InsertOrder.
			_, err := repo.InsertOrder(t.Context(), tc.userID, tc.orderNumber)
			require.ErrorIs(t, err, tc.wantErr)
			if err != nil {
				return
			}

			// 4. Проверяем, что заказ сохранился — читаем через SelectOrders.
			orders, err := repo.SelectOrders(t.Context(), tc.userID)
			require.NoError(t, err)
			require.Len(t, orders, 1)
			require.Equal(t, tc.userID, orders[0].UserId)
			require.Equal(t, tc.orderNumber, orders[0].Number)
			require.Equal(t, order.New, orders[0].Status)
			require.Equal(t, float32(0), orders[0].Accrual)
			require.NotEmpty(t, orders[0].Id)
			require.False(t, orders[0].UploadedAt.IsZero())
		})
	}
}

func TestRepo_Select(t *testing.T) {
	t.Parallel()

	userA := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	userB := uuid.MustParse("00000000-0000-0000-0000-000000000002")

	tests := []struct {
		name    string
		userID  uuid.UUID
		prepare func(t *testing.T, pool *pgxpool.Pool, repo *order.OrdersRepo)
		want    []order.Order
	}{
		{
			name:   "Success select orders",
			userID: userA,
			prepare: func(t *testing.T, pool *pgxpool.Pool, repo *order.OrdersRepo) {
				t.Helper()

				// 1. Создаём пользователя в users — без этого InsertOrder упадёт на FK.
				seedUserWithID(t, pool, userA)

				// 2. Вставляем два заказа через репозиторий orders.
				_, err := repo.InsertOrder(t.Context(), userA, "33763345")
				require.NoError(t, err)
				time.Sleep(10 * time.Millisecond) // uploaded_at различается → проверяем сортировку DESC.
				_, err = repo.InsertOrder(t.Context(), userA, "33763346")
				require.NoError(t, err)
			},
			want: []order.Order{
				{Number: "33763346", Status: order.New, Accrual: 0},
				{Number: "33763345", Status: order.New, Accrual: 0},
			},
		},
		{
			name:   "No data",
			userID: userB,
			prepare: func(t *testing.T, pool *pgxpool.Pool, _ *order.OrdersRepo) {
				t.Helper()

				// Пользователь есть, но заказов для него не создаём.
				seedUserWithID(t, pool, userB)
			},
			want: []order.Order{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// 1. Поднимаем Postgres и накатываем миграции.
			pool, _, _ := postgres.New(t)
			repo := order.NewRepository(pool)

			// 2. Готовим данные: пользователь + заказы (если нужны для кейса).
			tc.prepare(t, pool, repo)

			// 3. Вызываем SelectOrders — тестируем только этот метод репозитория.
			got, err := repo.SelectOrders(t.Context(), tc.userID)
			require.NoError(t, err)
			require.Len(t, got, len(tc.want))

			// 4. Сравниваем поля заказа с ожиданием.
			for i, want := range tc.want {
				require.Equal(t, tc.userID, got[i].UserId)
				require.Equal(t, want.Number, got[i].Number)
				require.Equal(t, want.Status, got[i].Status)
				require.Equal(t, want.Accrual, got[i].Accrual)
				require.NotEmpty(t, got[i].Id)
				require.False(t, got[i].UploadedAt.IsZero())
			}
		})
	}
}
