package balance

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/Radiushina/loyalty-system/internal/httputil"
	"github.com/Radiushina/loyalty-system/internal/user"
	"github.com/Radiushina/loyalty-system/pkg/luhn"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type (
	Handler struct {
		service ServiceProvider
		log     *zap.Logger
	}

	ServiceProvider interface {
		WithdrawBalance(ctx context.Context, userID uuid.UUID, opt WithdrawOpt) error
		SelectBalance(ctx context.Context, userID uuid.UUID) (UserBalance, error)
		SelectWithdrawals(ctx context.Context, userID uuid.UUID) ([]Withdrawal, error)
	}
)

// NewHandler создаёт HTTP-обработчик баланса и списаний.
func NewHandler(service ServiceProvider, log *zap.Logger) *Handler {
	return &Handler{
		service: service,
		log:     log,
	}
}

// WithdrawBalance обрабатывает POST /api/user/balance/withdraw — списание баллов в счёт оплаты заказа.
func (h *Handler) WithdrawBalance() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer func() { _ = r.Body.Close() }()

		userID, err := user.UserIDFromContext(r.Context())
		if errors.Is(err, user.ErrUnauthorized) {
			httputil.WriteError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		var opt WithdrawOpt
		if err := json.NewDecoder(r.Body).Decode(&opt); err != nil {
			httputil.WriteError(w, http.StatusBadRequest, "invalid request format")
			return
		}

		err = h.service.WithdrawBalance(r.Context(), userID, opt)
		if err != nil {
			switch {
			case errors.Is(err, luhn.ErrInvalidOrderNumber):
				httputil.WriteError(w, http.StatusUnprocessableEntity, "invalid order number format")
			case errors.Is(err, ErrInsufficientFunds):
				httputil.WriteError(w, http.StatusPaymentRequired, "not enough funds")
			case errors.Is(err, ErrWithdrawalAlreadyExists):
				httputil.WriteError(w, http.StatusConflict, "withdrawal for this order already exists")
			default:
				h.log.Error("withdraw balance", zap.Error(err))
				httputil.WriteError(w, http.StatusInternalServerError, "internal server error")
			}
			return
		}

		w.WriteHeader(http.StatusOK)
	}
}

// GetBalance обрабатывает GET /api/user/balance — текущий баланс и сумма списаний за всё время.
func (h *Handler) GetBalance() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, err := user.UserIDFromContext(r.Context())
		if errors.Is(err, user.ErrUnauthorized) {
			httputil.WriteError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		balance, err := h.service.SelectBalance(r.Context(), userID)
		if err != nil {
			h.log.Error("get balance", zap.Error(err))
			httputil.WriteError(w, http.StatusInternalServerError, "internal server error")
			return
		}
		if err := httputil.WriteJSON(w, http.StatusOK, balance); err != nil {
			h.log.Error("encode response", zap.Error(err))
		}
	}
}

// GetWithdrawals обрабатывает GET /api/user/withdrawals — история списаний от новых к старым.
func (h *Handler) GetWithdrawals() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, err := user.UserIDFromContext(r.Context())
		if errors.Is(err, user.ErrUnauthorized) {
			httputil.WriteError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		withdrawals, err := h.service.SelectWithdrawals(r.Context(), userID)
		if err != nil {
			h.log.Error("get orders", zap.Error(err))
			httputil.WriteError(w, http.StatusInternalServerError, "internal server error")
			return
		}

		if len(withdrawals) == 0 {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		if err := httputil.WriteJSON(w, http.StatusOK, withdrawalsMapToDTO(withdrawals)); err != nil {
			h.log.Error("encode response", zap.Error(err))
		}
	}
}

func withdrawalsMapToDTO(withdrawals []Withdrawal) []WithdrawalDTO {
	res := make([]WithdrawalDTO, 0, len(withdrawals))
	for _, w := range withdrawals {
		res = append(res, w.toDTO())
	}
	return res
}
