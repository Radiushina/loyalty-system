package balance

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrInsufficientFunds       = errors.New("insufficient funds")
	ErrWithdrawalAlreadyExists = errors.New("withdrawal for this order already exists")
)

type UserBalance struct {
	UserID    uuid.UUID `db:"user_id" json:"user_id"`
	Current   float64   `db:"current" json:"current"`
	Withdrawn float64   `db:"withdrawn" json:"withdrawn"`
}

type Withdrawals struct {
	Order       string    `db:"order_number" json:"order_number"`
	Sum         float64   `db:"sum" json:"sum"`
	ProcessedAt time.Time `db:"processed_at" json:"processed_at"`
}

type WithdrawOpt struct {
	Order string  `json:"order"`
	Sum   float64 `json:"sum"`
}

type WithdrawalsDTO struct {
	Order       string    `json:"order"`
	Sum         float64   `json:"sum"`
	ProcessedAt time.Time `json:"processed_at"`
}

func (w *Withdrawals) toDTO() WithdrawalsDTO {
	item := WithdrawalsDTO{
		Order:       w.Order,
		Sum:         w.Sum,
		ProcessedAt: w.ProcessedAt,
	}
	return item
}
