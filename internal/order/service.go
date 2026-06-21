package order

import (
	"context"

	"github.com/Radiushina/loyalty-system/pkg/luhn"
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
	if !luhn.Valid(orderNumber) {
		return luhn.ErrInvalidOrderNumber
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
