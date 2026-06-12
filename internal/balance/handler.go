package balance

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/Radiushina/loyalty-system/internal/user"
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
		SelectWithdrawals(ctx context.Context, userID uuid.UUID) ([]Withdrawals, error)
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

	}
}

// GetBalance обрабатывает GET /api/user/balance — текущий баланс и сумма списаний за всё время.
func (h *Handler) GetBalance() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := user.UserIDFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		balance, err := h.service.SelectBalance(r.Context(), userID)
		if err != nil {
			h.log.Error("get balance", zap.Error(err))
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}
		if err := writeJSON(w, http.StatusOK, balance); err != nil {
			h.log.Error("encode response", zap.Error(err))
		}
	}
}

// GetWithdrawals обрабатывает GET /api/user/withdrawals — история списаний от новых к старым.
func (h *Handler) GetWithdrawals() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := user.UserIDFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		withdrawals, err := h.service.SelectWithdrawals(r.Context(), userID)
		if err != nil {
			h.log.Error("get orders", zap.Error(err))
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}

		if len(withdrawals) == 0 {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		if err := writeJSON(w, http.StatusOK, withdrawalsMapToDTO(withdrawals)); err != nil {
			h.log.Error("encode response", zap.Error(err))
		}
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	_ = writeJSON(w, status, map[string]string{"msg": message})
}

func writeJSON(w http.ResponseWriter, status int, v any) error {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	return json.NewEncoder(w).Encode(v)
}

func withdrawalsMapToDTO(withdrawals []Withdrawals) []WithdrawalsDTO {
	res := make([]WithdrawalsDTO, 0, len(withdrawals))
	for _, w := range withdrawals {
		res = append(res, w.toDTO())
	}
	return res
}
