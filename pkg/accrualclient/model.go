package accrualclient

// Status — статус расчёта начисления во внешней системе.
type Status string

const (
	StatusRegistered Status = "REGISTERED"
	StatusInvalid    Status = "INVALID"
	StatusProcessing Status = "PROCESSING"
	StatusProcessed  Status = "PROCESSED"
)

// OrderInfo — ответ GET /api/orders/{number} при коде 200.
type OrderInfo struct {
	Order   string   `json:"order"`
	Status  Status   `json:"status"`
	Accrual *float32 `json:"accrual,omitempty"`
}

func (s Status) IsFinal() bool {
	return s == StatusInvalid || s == StatusProcessed
}
