package balance

import (
	"context"

	"github.com/Radiushina/loyalty-system/pkg/luhn"
	"github.com/google/uuid"
)

type (
	Service struct {
		repo RepoProvider
	}

	RepoProvider interface {
		WithdrawBalance(ctx context.Context, userID uuid.UUID, opt WithdrawOpt) error
		SelectBalance(ctx context.Context, userID uuid.UUID) (UserBalance, error)
		SelectWithdrawals(ctx context.Context, userID uuid.UUID) ([]Withdrawal, error)
		CreditAccrual(ctx context.Context, userID, orderID uuid.UUID, amount float64) error
	}
)

// NewService создаёт сервис работы с балансом пользователя.
func NewService(repo RepoProvider) *Service {
	return &Service{
		repo: repo,
	}
}

// WithdrawBalance списывает баллы с накопительного счёта в счёт оплаты заказа.
func (s *Service) WithdrawBalance(ctx context.Context, userID uuid.UUID, opt WithdrawOpt) error {
	if !luhn.Valid(opt.Order) {
		return luhn.ErrInvalidOrderNumber
	}

	return s.repo.WithdrawBalance(ctx, userID, opt)
}

// CreditAccrual начисляет баллы на счёт пользователя за обработанный заказ.
func (s *Service) CreditAccrual(ctx context.Context, userID, orderID uuid.UUID, amount float64) error {
	return s.repo.CreditAccrual(ctx, userID, orderID, amount)
}

// SelectBalance возвращает текущий баланс и общую сумму списаний пользователя.
func (s *Service) SelectBalance(ctx context.Context, userID uuid.UUID) (UserBalance, error) {
	balance, err := s.repo.SelectBalance(ctx, userID)
	if err != nil {
		return UserBalance{}, err
	}
	return balance, nil
}

// SelectWithdrawals возвращает историю списаний пользователя, отсортированную по дате убывания.
func (s *Service) SelectWithdrawals(ctx context.Context, userID uuid.UUID) ([]Withdrawal, error) {
	withdrawals, err := s.repo.SelectWithdrawals(ctx, userID)
	if err != nil {
		return nil, err
	}

	return withdrawals, nil
}
