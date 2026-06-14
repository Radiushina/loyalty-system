package order

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"

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
		CreateOrder(ctx context.Context, userID uuid.UUID, orderNumber string) error
		SelectOrders(ctx context.Context, userID uuid.UUID) ([]Order, error)
	}
)

func NewHandler(service ServiceProvider, log *zap.Logger) *Handler {
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
			httputil.WriteError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			httputil.WriteError(w, http.StatusBadRequest, "failed to read body")
			return
		}

		number := strings.TrimSpace(string(body))
		if number == "" {
			httputil.WriteError(w, http.StatusBadRequest, "invalid request format")
			return
		}

		err = h.service.CreateOrder(r.Context(), userID, number)
		if err != nil {
			switch {
			case errors.Is(err, luhn.ErrInvalidOrderNumber):
				httputil.WriteError(w, http.StatusUnprocessableEntity, "invalid order number format")
			case errors.Is(err, ErrOrderWasUploaded):
				httputil.WriteError(w, http.StatusConflict, "the order number has already been uploaded by another user")
			case errors.Is(err, ErrOrderAlreadyUploadedByUser):
				w.WriteHeader(http.StatusOK)
			default:
				h.log.Error("create order", zap.Error(err))
				httputil.WriteError(w, http.StatusInternalServerError, "internal server error")
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
			httputil.WriteError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		orders, err := h.service.SelectOrders(r.Context(), userID)
		if err != nil {
			h.log.Error("get orders", zap.Error(err))
			httputil.WriteError(w, http.StatusInternalServerError, "internal server error")
			return
		}

		if len(orders) == 0 {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		if err := httputil.WriteJSON(w, http.StatusOK, ordersMapToDTO(orders)); err != nil {
			h.log.Error("encode response", zap.Error(err))
		}
	}
}

func ordersMapToDTO(orders []Order) []OrderDTO {
	res := make([]OrderDTO, 0, len(orders))
	for _, o := range orders {
		res = append(res, o.toDTO())
	}
	return res
}
