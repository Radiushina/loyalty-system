package order

import (
	"context"

	"github.com/google/uuid"
)

type (
	Service struct {
		repo RepoProvider
	}

	RepoProvider interface {
		InsertOrder(ctx context.Context, userID uuid.UUID, orderNumber string) error
		SelectOrders(ctx context.Context, userID uuid.UUID) ([]Order, error)
	}
)

func NewService(repo RepoProvider) *Service {
	return &Service{
		repo: repo,
	}
}

func (s *Service) CreateOrder(ctx context.Context, userID uuid.UUID, orderNumber string) error {
	if !luhnValid(orderNumber) {
		return ErrInvalidOrderNumber
	}

	return s.repo.InsertOrder(ctx, userID, orderNumber)
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
