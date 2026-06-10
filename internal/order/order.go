package order

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrOrderWasUploaded           = errors.New("the order number has already been uploaded by another user")
	ErrOrderAlreadyUploadedByUser = errors.New("the order number has already been uploaded by this user")
	ErrInvalidOrderNumber         = errors.New("invalid order number format")
)

type Order struct {
	Id         uuid.UUID `db:"id" json:"id"`
	UserId     uuid.UUID `db:"user_id" json:"user_id"`
	Number     string    `db:"number" json:"number"`
	Status     string    `db:"status" json:"status"`
	Accrual    int       `db:"accrual" json:"accrual"`
	UploadedAt time.Time `db:"uploaded_at" json:"uploaded_at"`
}

type OrderDTO struct {
	Number     string    `json:"number"`
	Status     string    `json:"status"`
	Accrual    *int      `json:"accrual,omitempty"`
	UploadedAt time.Time `json:"uploaded_at"`
}

func (o *Order) toDTO() OrderDTO {
	item := OrderDTO{
		Number:     o.Number,
		Status:     o.Status,
		UploadedAt: o.UploadedAt,
	}
	if o.Status == "PROCESSED" {
		accrual := o.Accrual
		item.Accrual = &accrual
	}

	return item
}
