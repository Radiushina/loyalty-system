package order_test

import (
	"testing"

	"github.com/Radiushina/loyalty-system/internal/order"
	"github.com/Radiushina/loyalty-system/internal/order/order_mocks"
	"github.com/Radiushina/loyalty-system/pkg/luhn"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestService_CreateOrder(t *testing.T) {
	t.Parallel()

	userID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	validOrderNumber := "79927398713" // проходит проверку Луна

	tests := []struct {
		name        string
		orderNumber string
		prepare     func(repo *order_mocks.RepoProvider)
		wantErr     error
	}{
		{
			name:        "Success insert order",
			orderNumber: validOrderNumber,
			prepare: func(repo *order_mocks.RepoProvider) {
				repo.EXPECT().
					InsertOrder(mock.Anything, userID, validOrderNumber).
					Return(uuid.MustParse("00000000-0000-0000-0000-000000000010"), nil)
			},
		},
		{
			name:        "Invalid order number — not digits",
			orderNumber: "invalid",
			wantErr:     luhn.ErrInvalidOrderNumber,
		},
		{
			name:        "Invalid order number — failed luhn check",
			orderNumber: "123",
			wantErr:     luhn.ErrInvalidOrderNumber,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			repo := order_mocks.NewRepoProvider(t)
			if tc.prepare != nil {
				tc.prepare(repo)
			}

			svc := order.NewService(repo, nil)

			err := svc.CreateOrder(t.Context(), userID, tc.orderNumber)
			require.ErrorIs(t, err, tc.wantErr)
			if err != nil {
				return
			}
		})
	}
}
