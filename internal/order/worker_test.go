package order_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Radiushina/loyalty-system/internal/order"
	"github.com/Radiushina/loyalty-system/internal/order/order_mocks"
	"github.com/Radiushina/loyalty-system/pkg/accrualclient"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestService_CreateOrder_EnqueuesJob(t *testing.T) {
	t.Parallel()

	userID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	orderID := uuid.MustParse("00000000-0000-0000-0000-000000000010")
	number := "79927398713"

	repo := order_mocks.NewRepoProvider(t)
	repo.EXPECT().
		InsertOrder(mock.Anything, userID, number).
		Return(orderID, nil)

	queue := order_mocks.NewAccrualEnqueuer(t)
	queue.EXPECT().
		Enqueue(order.OrderJob{
			ID:     orderID,
			UserID: userID,
			Number: number,
			Status: order.New,
		})

	svc := order.NewService(repo, queue)
	require.NoError(t, svc.CreateOrder(t.Context(), userID, number))
}

func TestAccrualWorkerPool_ProcessesEnqueuedOrder(t *testing.T) {
	t.Parallel()

	orderID := uuid.MustParse("00000000-0000-0000-0000-000000000010")
	userID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	number := "79927398713"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"order":"79927398713","status":"PROCESSED","accrual":500}`))
	}))
	t.Cleanup(server.Close)

	repo := order_mocks.NewWorkerRepoProvider(t)
	repo.EXPECT().
		GetOrderByID(mock.Anything, orderID).
		Return(order.Order{
			ID:     orderID,
			UserID: userID,
			Number: number,
			Status: order.New,
		}, nil)
	repo.EXPECT().
		UpdateOrderAccrual(mock.Anything, orderID, order.Processed, float64(500)).
		Return(nil)

	done := make(chan struct{})

	balance := order_mocks.NewBalanceProvider(t)
	balance.EXPECT().
		CreditAccrual(mock.Anything, userID, orderID, float64(500)).
		Run(func(context.Context, uuid.UUID, uuid.UUID, float64) { close(done) }).
		Return(nil)

	pool := order.NewAccrualWorkerPool(
		order.WorkerPoolConfig{Workers: 1, PollInterval: time.Hour},
		repo,
		accrualclient.New(accrualclient.Config{Address: server.URL}),
		balance,
		zap.NewNop(),
	)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	go pool.Run(ctx)
	pool.Enqueue(order.OrderJob{ID: orderID, UserID: userID, Number: number, Status: order.New})

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("worker did not process order in time")
	}
}
