package accrualclient_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Radiushina/loyalty-system/pkg/accrualclient"
	"github.com/stretchr/testify/require"
)

func TestAccrualClient_GetOrderInfo(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		orderNumber string
		handler     http.HandlerFunc
		want        accrualclient.OrderInfo
		wantErr     error
		checkErr    func(t *testing.T, err error)
	}{
		{
			name:        "Success",
			orderNumber: "79927398713",
			handler: func(w http.ResponseWriter, r *http.Request) {
				require.Equal(t, http.MethodGet, r.Method)
				require.Equal(t, "/api/orders/79927398713", r.URL.Path)

				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"order":"79927398713","status":"PROCESSED","accrual":500}`))
			},
			want: accrualclient.OrderInfo{
				Order:  "79927398713",
				Status: accrualclient.StatusProcessed,
				Accrual: func() *float64 {
					v := float64(500)
					return &v
				}(),
			},
		},
		{
			name:        "Success without accrual",
			orderNumber: "79927398713",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"order":"79927398713","status":"REGISTERED"}`))
			},
			want: accrualclient.OrderInfo{
				Order:  "79927398713",
				Status: accrualclient.StatusRegistered,
			},
		},
		{
			name:        "Not found",
			orderNumber: "79927398713",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNoContent)
			},
			wantErr: accrualclient.ErrNotFound,
		},
		{
			name:        "Rate limited",
			orderNumber: "79927398713",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Retry-After", "60")
				w.WriteHeader(http.StatusTooManyRequests)
				_, _ = w.Write([]byte("No more than N requests per minute allowed"))
			},
			checkErr: func(t *testing.T, err error) {
				t.Helper()
				var rateErr *accrualclient.RateLimitedError
				require.ErrorAs(t, err, &rateErr)
				require.Equal(t, int64(60), int64(rateErr.RetryAfter.Seconds()))
			},
		},
		{
			name:        "Server error",
			orderNumber: "79927398713",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			},
			wantErr: accrualclient.ErrServer,
		},
		{
			name:        "Invalid JSON",
			orderNumber: "79927398713",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(`{`))
			},
			checkErr: func(t *testing.T, err error) {
				t.Helper()
				require.ErrorContains(t, err, "decode accrual response")
			},
		},
		{
			name:        "Empty order number",
			orderNumber: "   ",
			handler:     func(http.ResponseWriter, *http.Request) {},
			wantErr:     accrualclient.ErrEmptyOrderNumber,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			server := httptest.NewServer(tc.handler)
			t.Cleanup(server.Close)

			client := accrualclient.New(accrualclient.Config{Address: server.URL})

			got, err := client.GetOrderInfo(context.Background(), tc.orderNumber)
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
				return
			}
			if tc.checkErr != nil {
				tc.checkErr(t, err)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestStatus_IsFinal(t *testing.T) {
	t.Parallel()

	require.False(t, accrualclient.StatusRegistered.IsFinal())
	require.False(t, accrualclient.StatusProcessing.IsFinal())
	require.True(t, accrualclient.StatusInvalid.IsFinal())
	require.True(t, accrualclient.StatusProcessed.IsFinal())
}

func TestUnexpectedStatusError(t *testing.T) {
	t.Parallel()

	err := &accrualclient.UnexpectedStatusError{StatusCode: http.StatusBadGateway}
	require.False(t, errors.Is(err, accrualclient.ErrServer))
	require.Contains(t, err.Error(), "502")
}
