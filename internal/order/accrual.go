package order

import (
	"errors"
	"fmt"

	"github.com/Radiushina/loyalty-system/pkg/accrualclient"
)

var ErrUnknownAccrualStatus = errors.New("unknown accrual status")

// MapAccrualStatus переводит status из внешней системы расчёта
// во внутренний статус заказа (хранится в БД и отдаётся в API).
//
// Внешний REGISTERED в нашей БД нет — заказ считается находящимся в обработке.
//
// | Внешняя система | Наш сервис (orders.status) |
// |-----------------|----------------------------|
// | REGISTERED      | PROCESSING                 |
// | PROCESSING      | PROCESSING                 |
// | INVALID         | INVALID                    |
// | PROCESSED       | PROCESSED                  |
//
// Статус NEW выставляется только при загрузке заказа пользователем,
// из внешней системы не приходит.
func MapAccrualStatus(external accrualclient.Status) (Status, error) {
	if err := external.Validate(); err != nil {
		return "", fmt.Errorf("%w: %q", ErrUnknownAccrualStatus, external)
	}

	switch external {
	case accrualclient.StatusRegistered, accrualclient.StatusProcessing:
		return Processing, nil
	case accrualclient.StatusInvalid:
		return Invalid, nil
	case accrualclient.StatusProcessed:
		return Processed, nil
	default:
		return "", fmt.Errorf("%w: %q", ErrUnknownAccrualStatus, external)
	}
}

// ApplyAccrualInfo обновляет поля заказа по ответу внешней системы (HTTP 200).
func ApplyAccrualInfo(o *Order, info accrualclient.OrderInfo) error {
	status, err := MapAccrualStatus(info.Status)
	if err != nil {
		return err
	}

	o.Status = status
	if info.Accrual != nil {
		o.Accrual = *info.Accrual
	}

	return nil
}

// AccrualPollOutcome — что делать воркеру после опроса внешней системы.
type AccrualPollOutcome string

const (
	// AccrualPollUpdated — получен ответ 200, заказ обновлён через ApplyAccrualInfo.
	AccrualPollUpdated AccrualPollOutcome = "updated"
	// AccrualPollNotRegistered — 204: заказ ещё не во внешней системе,
	// статус в БД не меняем (остаётся NEW или PROCESSING), повторить позже.
	AccrualPollNotRegistered AccrualPollOutcome = "not_registered"
	// AccrualPollRateLimited — ответ 429: подождать Retry-After и повторить.
	AccrualPollRateLimited AccrualPollOutcome = "rate_limited"
	// AccrualPollTransientError — 500 или сеть: повторить позже без смены статуса.
	AccrualPollTransientError AccrualPollOutcome = "transient_error"
)
