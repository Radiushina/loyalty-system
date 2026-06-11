package order_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Radiushina/loyalty-system/internal/order"
	"github.com/Radiushina/loyalty-system/internal/order/order_mocks"
	"github.com/Radiushina/loyalty-system/internal/user"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func newHandler(svc *order_mocks.ServiceProvider) *order.Handler {
	return order.NewHandel(svc, zap.NewNop())
}

func requestWithUser(t *testing.T, method, target, body string, userID uuid.UUID) *http.Request {
	t.Helper()

	req := httptest.NewRequest(method, target, strings.NewReader(body))
	return req.WithContext(user.WithUserID(req.Context(), userID))
}

func TestHandler_CreateOrder(t *testing.T) {
	t.Parallel()

	userID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	validOrderNumber := "79927398713"

	tests := []struct {
		name       string
		body       string
		withUser   bool
		prepare    func(svc *order_mocks.ServiceProvider)
		wantStatus int
		wantMsg    string
	}{
		{
			name:       "Unauthorized",
			body:       validOrderNumber,
			withUser:   false,
			wantStatus: http.StatusUnauthorized,
			wantMsg:    "unauthorized",
		},
		{
			name:       "Empty body",
			body:       "   ",
			withUser:   true,
			wantStatus: http.StatusBadRequest,
			wantMsg:    "invalid request format",
		},
		{
			name:     "Success",
			body:     validOrderNumber,
			withUser: true,
			prepare: func(svc *order_mocks.ServiceProvider) {
				svc.EXPECT().
					CreateOrder(mock.Anything, userID, validOrderNumber).
					Return(nil)
			},
			wantStatus: http.StatusAccepted,
		},
		{
			name:     "Invalid order number",
			body:     "invalid",
			withUser: true,
			prepare: func(svc *order_mocks.ServiceProvider) {
				svc.EXPECT().
					CreateOrder(mock.Anything, userID, "invalid").
					Return(order.ErrInvalidOrderNumber)
			},
			wantStatus: http.StatusUnprocessableEntity,
			wantMsg:    "invalid order number format",
		},
		{
			name:     "Order uploaded by another user",
			body:     validOrderNumber,
			withUser: true,
			prepare: func(svc *order_mocks.ServiceProvider) {
				svc.EXPECT().
					CreateOrder(mock.Anything, userID, validOrderNumber).
					Return(order.ErrOrderWasUploaded)
			},
			wantStatus: http.StatusConflict,
			wantMsg:    "the order number has already been uploaded by another user",
		},
		{
			name:     "Order already uploaded by user",
			body:     validOrderNumber,
			withUser: true,
			prepare: func(svc *order_mocks.ServiceProvider) {
				svc.EXPECT().
					CreateOrder(mock.Anything, userID, validOrderNumber).
					Return(order.ErrOrderAlreadyUploadedByUser)
			},
			wantStatus: http.StatusOK,
		},
		{
			name:     "Internal server error",
			body:     validOrderNumber,
			withUser: true,
			prepare: func(svc *order_mocks.ServiceProvider) {
				svc.EXPECT().
					CreateOrder(mock.Anything, userID, validOrderNumber).
					Return(errors.New("db error"))
			},
			wantStatus: http.StatusInternalServerError,
			wantMsg:    "internal server error",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			svc := order_mocks.NewServiceProvider(t)
			if tc.prepare != nil {
				tc.prepare(svc)
			}

			h := newHandler(svc)
			rec := httptest.NewRecorder()

			var req *http.Request
			if tc.withUser {
				req = requestWithUser(t, http.MethodPost, "/api/user/orders", tc.body, userID)
			} else {
				req = httptest.NewRequest(http.MethodPost, "/api/user/orders", strings.NewReader(tc.body))
			}

			h.CreateOrder()(rec, req)

			require.Equal(t, tc.wantStatus, rec.Code)
			if tc.wantMsg == "" {
				return
			}

			var resp struct {
				Msg string `json:"msg"`
			}
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
			require.Equal(t, tc.wantMsg, resp.Msg)
		})
	}
}

func TestHandler_GetOrders(t *testing.T) {
	t.Parallel()

	userID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	uploadedAt := time.Date(2026, 6, 11, 10, 0, 0, 0, time.UTC)

	tests := []struct {
		name       string
		withUser   bool
		prepare    func(svc *order_mocks.ServiceProvider)
		wantStatus int
		wantBody   func(t *testing.T, body []byte)
	}{
		{
			name:       "Unauthorized",
			withUser:   false,
			wantStatus: http.StatusUnauthorized,
			wantBody: func(t *testing.T, body []byte) {
				t.Helper()
				require.Contains(t, string(body), "unauthorized")
			},
		},
		{
			name:     "Internal server error",
			withUser: true,
			prepare: func(svc *order_mocks.ServiceProvider) {
				svc.EXPECT().
					SelectOrders(mock.Anything, userID).
					Return(nil, errors.New("db error"))
			},
			wantStatus: http.StatusInternalServerError,
			wantBody: func(t *testing.T, body []byte) {
				t.Helper()
				require.Contains(t, string(body), "internal server error")
			},
		},
		{
			name:     "No content",
			withUser: true,
			prepare: func(svc *order_mocks.ServiceProvider) {
				svc.EXPECT().
					SelectOrders(mock.Anything, userID).
					Return([]order.Order{}, nil)
			},
			wantStatus: http.StatusNoContent,
			wantBody: func(t *testing.T, body []byte) {
				t.Helper()
				require.Empty(t, body)
			},
		},
		{
			name:     "Success",
			withUser: true,
			prepare: func(svc *order_mocks.ServiceProvider) {
				svc.EXPECT().
					SelectOrders(mock.Anything, userID).
					Return([]order.Order{
						{
							Number:     "79927398713",
							Status:     "NEW",
							UploadedAt: uploadedAt,
						},
					}, nil)
			},
			wantStatus: http.StatusOK,
			wantBody: func(t *testing.T, body []byte) {
				t.Helper()

				var got []order.OrderDTO
				require.NoError(t, json.Unmarshal(body, &got))
				require.Len(t, got, 1)
				require.Equal(t, "79927398713", got[0].Number)
				require.Equal(t, "NEW", got[0].Status)
				require.Equal(t, uploadedAt, got[0].UploadedAt)
				require.Nil(t, got[0].Accrual)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			svc := order_mocks.NewServiceProvider(t)
			if tc.prepare != nil {
				tc.prepare(svc)
			}

			h := newHandler(svc)
			rec := httptest.NewRecorder()

			var req *http.Request
			if tc.withUser {
				req = requestWithUser(t, http.MethodGet, "/api/user/orders", "", userID)
			} else {
				req = httptest.NewRequest(http.MethodGet, "/api/user/orders", nil)
			}

			h.GetOrders()(rec, req)

			require.Equal(t, tc.wantStatus, rec.Code)
			tc.wantBody(t, rec.Body.Bytes())
		})
	}
}
