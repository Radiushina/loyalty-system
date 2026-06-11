package order

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

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
		CreateOrder(ctx context.Context, userID uuid.UUID, orderNumber string) error
		SelectOrders(ctx context.Context, userID uuid.UUID) ([]Order, error)
	}
)

func NewHandel(service ServiceProvider, log *zap.Logger) *Handler {
	return &Handler{
		service: service,
		log:     log,
	}
}

func (h *Handler) CreateOrder() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer func() { _ = r.Body.Close() }()

		userID, ok := user.UserIDFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			writeError(w, http.StatusBadRequest, "failed to read body")
			return
		}

		number := strings.TrimSpace(string(body))
		if number == "" {
			writeError(w, http.StatusBadRequest, "invalid request format")
			return
		}

		err = h.service.CreateOrder(r.Context(), userID, number)
		if err != nil {
			switch {
			case errors.Is(err, ErrInvalidOrderNumber):
				writeError(w, http.StatusUnprocessableEntity, "invalid order number format")
			case errors.Is(err, ErrOrderWasUploaded):
				writeError(w, http.StatusConflict, "the order number has already been uploaded by another user")
			case errors.Is(err, ErrOrderAlreadyUploadedByUser):
				w.WriteHeader(http.StatusOK)
			default:
				h.log.Error("create order", zap.Error(err))
				writeError(w, http.StatusInternalServerError, "internal server error")
			}
			return
		}

		w.WriteHeader(http.StatusAccepted)
	}
}

func (h *Handler) GetOrders() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := user.UserIDFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		orders, err := h.service.SelectOrders(r.Context(), userID)
		if err != nil {
			h.log.Error("get orders", zap.Error(err))
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}

		if len(orders) == 0 {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		if err := writeJSON(w, http.StatusOK, ordersMapToDTO(orders)); err != nil {
			h.log.Error("encode response", zap.Error(err))
		}
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) error {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	return json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, message string) {
	_ = writeJSON(w, status, map[string]string{"msg": message})
}

func ordersMapToDTO(orders []Order) []OrderDTO {
	res := make([]OrderDTO, 0, len(orders))
	for _, o := range orders {
		res = append(res, o.toDTO())
	}
	return res
}
