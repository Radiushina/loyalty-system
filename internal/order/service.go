package order

import (
	"context"

	"github.com/google/uuid"
)

type (
	Service struct {
		repo    RepoProvider
		accrual AccrualEnqueuer
	}

	RepoProvider interface {
		InsertOrder(ctx context.Context, userID uuid.UUID, orderNumber string) (uuid.UUID, error)
		SelectOrders(ctx context.Context, userID uuid.UUID) ([]Order, error)
	}
)

func NewService(repo RepoProvider, accrual AccrualEnqueuer) *Service {
	return &Service{
		repo:    repo,
		accrual: accrual,
	}
}

func (s *Service) CreateOrder(ctx context.Context, userID uuid.UUID, orderNumber string) error {
	if !luhnValid(orderNumber) {
		return ErrInvalidOrderNumber
	}

	orderID, err := s.repo.InsertOrder(ctx, userID, orderNumber)
	if err != nil {
		return err
	}

	if s.accrual != nil {
		s.accrual.Enqueue(OrderJob{
			ID:     orderID,
			UserID: userID,
			Number: orderNumber,
			Status: New,
		})
	}

	return nil
}

func (s *Service) SelectOrders(ctx context.Context, userID uuid.UUID) ([]Order, error) {
	orders, err := s.repo.SelectOrders(ctx, userID)
	if err != nil {
		return nil, err
	}

	return orders, nil
}

func luhnValid(number string) bool {
	sum := 0
	parity := len(number) % 2

	for i := 0; i < len(number); i++ {
		if number[i] < '0' || number[i] > '9' {
			return false
		}

		d := int(number[i] - '0')
		if i%2 == parity {
			d *= 2
			if d > 9 {
				d -= 9
			}
		}
		sum += d
	}

	return sum%10 == 0
}
