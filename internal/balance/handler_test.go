package balance_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Radiushina/loyalty-system/internal/balance"
	"github.com/Radiushina/loyalty-system/internal/balance/balance_mocks"
	"github.com/Radiushina/loyalty-system/internal/user"
	"github.com/Radiushina/loyalty-system/pkg/luhn"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func newHandler(svc *balance_mocks.ServiceProvider) *balance.Handler {
	return balance.NewHandler(svc, zap.NewNop())
}

func requestWithUser(t *testing.T, method, target, body string, userID uuid.UUID) *http.Request {
	t.Helper()

	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	return req.WithContext(user.WithUserID(req.Context(), userID))
}

func TestHandler_WithdrawBalance(t *testing.T) {
	t.Parallel()

	userID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	validBody := `{"order":"79927398713","sum":751}`
	withdrawOpt := balance.WithdrawOpt{Order: "79927398713", Sum: 751}

	tests := []struct {
		name       string
		body       string
		withUser   bool
		prepare    func(svc *balance_mocks.ServiceProvider)
		wantStatus int
		wantMsg    string
	}{
		{
			name:       "Unauthorized",
			body:       validBody,
			withUser:   false,
			wantStatus: http.StatusUnauthorized,
			wantMsg:    "unauthorized",
		},
		{
			name:       "Invalid JSON",
			body:       `{`,
			withUser:   true,
			wantStatus: http.StatusBadRequest,
			wantMsg:    "invalid request format",
		},
		{
			name:     "Success",
			body:     validBody,
			withUser: true,
			prepare: func(svc *balance_mocks.ServiceProvider) {
				svc.EXPECT().
					WithdrawBalance(mock.Anything, userID, withdrawOpt).
					Return(nil)
			},
			wantStatus: http.StatusOK,
		},
		{
			name:     "Invalid order number",
			body:     `{"order":"123","sum":751}`,
			withUser: true,
			prepare: func(svc *balance_mocks.ServiceProvider) {
				svc.EXPECT().
					WithdrawBalance(mock.Anything, userID, balance.WithdrawOpt{Order: "123", Sum: 751}).
					Return(luhn.ErrInvalidOrderNumber)
			},
			wantStatus: http.StatusUnprocessableEntity,
			wantMsg:    "invalid order number format",
		},
		{
			name:     "Insufficient funds",
			body:     validBody,
			withUser: true,
			prepare: func(svc *balance_mocks.ServiceProvider) {
				svc.EXPECT().
					WithdrawBalance(mock.Anything, userID, withdrawOpt).
					Return(balance.ErrInsufficientFunds)
			},
			wantStatus: http.StatusPaymentRequired,
			wantMsg:    "not enough funds",
		},
		{
			name:     "Withdrawal already exists",
			body:     validBody,
			withUser: true,
			prepare: func(svc *balance_mocks.ServiceProvider) {
				svc.EXPECT().
					WithdrawBalance(mock.Anything, userID, withdrawOpt).
					Return(balance.ErrWithdrawalAlreadyExists)
			},
			wantStatus: http.StatusConflict,
			wantMsg:    "withdrawal for this order already exists",
		},
		{
			name:     "Internal server error",
			body:     validBody,
			withUser: true,
			prepare: func(svc *balance_mocks.ServiceProvider) {
				svc.EXPECT().
					WithdrawBalance(mock.Anything, userID, withdrawOpt).
					Return(errors.New("db error"))
			},
			wantStatus: http.StatusInternalServerError,
			wantMsg:    "internal server error",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			svc := balance_mocks.NewServiceProvider(t)
			if tc.prepare != nil {
				tc.prepare(svc)
			}

			h := newHandler(svc)
			rec := httptest.NewRecorder()

			var req *http.Request
			if tc.withUser {
				req = requestWithUser(t, http.MethodPost, "/api/user/balance/withdraw", tc.body, userID)
			} else {
				req = httptest.NewRequest(http.MethodPost, "/api/user/balance/withdraw", strings.NewReader(tc.body))
				req.Header.Set("Content-Type", "application/json")
			}

			h.WithdrawBalance()(rec, req)

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

func TestHandler_GetBalance(t *testing.T) {
	t.Parallel()

	userID := uuid.MustParse("00000000-0000-0000-0000-000000000001")

	tests := []struct {
		name       string
		withUser   bool
		prepare    func(svc *balance_mocks.ServiceProvider)
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
			name:     "Success",
			withUser: true,
			prepare: func(svc *balance_mocks.ServiceProvider) {
				svc.EXPECT().
					SelectBalance(mock.Anything, userID).
					Return(balance.UserBalance{
						UserID:    userID,
						Current:   500.5,
						Withdrawn: 42,
					}, nil)
			},
			wantStatus: http.StatusOK,
			wantBody: func(t *testing.T, body []byte) {
				t.Helper()

				var got balance.UserBalance
				require.NoError(t, json.Unmarshal(body, &got))
				require.Equal(t, userID, got.UserID)
				require.Equal(t, 500.5, got.Current)
				require.Equal(t, float64(42), got.Withdrawn)
			},
		},
		{
			name:     "Internal server error",
			withUser: true,
			prepare: func(svc *balance_mocks.ServiceProvider) {
				svc.EXPECT().
					SelectBalance(mock.Anything, userID).
					Return(balance.UserBalance{}, errors.New("db error"))
			},
			wantStatus: http.StatusInternalServerError,
			wantBody: func(t *testing.T, body []byte) {
				t.Helper()
				require.Contains(t, string(body), "internal server error")
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			svc := balance_mocks.NewServiceProvider(t)
			if tc.prepare != nil {
				tc.prepare(svc)
			}

			h := newHandler(svc)
			rec := httptest.NewRecorder()

			var req *http.Request
			if tc.withUser {
				req = requestWithUser(t, http.MethodGet, "/api/user/balance", "", userID)
			} else {
				req = httptest.NewRequest(http.MethodGet, "/api/user/balance", nil)
			}

			h.GetBalance()(rec, req)

			require.Equal(t, tc.wantStatus, rec.Code)
			tc.wantBody(t, rec.Body.Bytes())
		})
	}
}
