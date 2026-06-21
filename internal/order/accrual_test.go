package order_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Radiushina/loyalty-system/internal/order"
	"github.com/Radiushina/loyalty-system/pkg/accrualclient"
	"github.com/stretchr/testify/require"
)

func TestMapAccrualStatus(t *testing.T) {
	t.Parallel()

	tests := []struct {
		external accrualclient.Status
		want     order.Status
		wantErr  error
	}{
		{external: accrualclient.StatusRegistered, want: order.Processing},
		{external: accrualclient.StatusProcessing, want: order.Processing},
		{external: accrualclient.StatusInvalid, want: order.Invalid},
		{external: accrualclient.StatusProcessed, want: order.Processed},
		{external: accrualclient.Status("NEW"), wantErr: order.ErrUnknownAccrualStatus},
	}

	for _, tc := range tests {
		t.Run(string(tc.external), func(t *testing.T) {
			t.Parallel()

			got, err := order.MapAccrualStatus(tc.external)
			require.ErrorIs(t, err, tc.wantErr)
			if tc.wantErr != nil {
				return
			}
			require.Equal(t, tc.want, got)
		})
	}
}

func TestApplyAccrualInfo(t *testing.T) {
	t.Parallel()

	accrual := float64(500)
	o := &order.Order{Status: order.New}

	err := order.ApplyAccrualInfo(o, accrualclient.OrderInfo{
		Order:   "79927398713",
		Status:  accrualclient.StatusProcessed,
		Accrual: &accrual,
	})
	require.NoError(t, err)
	require.Equal(t, order.Processed, o.Status)
	require.Equal(t, float64(500), o.Accrual)
}

func TestPollOrder(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(server.Close)

	client := accrualclient.New(accrualclient.Config{Address: server.URL})

	outcome, _, _, err := order.PollOrder(t.Context(), client, "79927398713")
	require.NoError(t, err)
	require.Equal(t, order.AccrualPollNotRegistered, outcome)
}
