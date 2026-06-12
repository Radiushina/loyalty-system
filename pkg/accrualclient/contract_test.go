package accrualclient_test

import (
	"encoding/json"
	"testing"

	"github.com/Radiushina/loyalty-system/pkg/accrualclient"
	"github.com/stretchr/testify/require"
)

func TestStatus_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		status  accrualclient.Status
		wantErr error
	}{
		{status: accrualclient.StatusRegistered},
		{status: accrualclient.StatusProcessing},
		{status: accrualclient.StatusInvalid},
		{status: accrualclient.StatusProcessed},
		{status: accrualclient.Status("UNKNOWN"), wantErr: accrualclient.ErrUnknownStatus},
	}

	for _, tc := range tests {
		t.Run(string(tc.status), func(t *testing.T) {
			t.Parallel()
			err := tc.status.Validate()
			require.ErrorIs(t, err, tc.wantErr)
		})
	}
}

func TestOrderInfo_JSONContract(t *testing.T) {
	t.Parallel()

	raw := `{"order":"79927398713","status":"PROCESSED","accrual":500}`

	var info accrualclient.OrderInfo
	require.NoError(t, json.Unmarshal([]byte(raw), &info))
	require.Equal(t, "79927398713", info.Order)
	require.Equal(t, accrualclient.StatusProcessed, info.Status)
	require.NotNil(t, info.Accrual)
	require.Equal(t, float32(500), *info.Accrual)
}

func TestOrderInfo_JSONContractFloatAccrual(t *testing.T) {
	t.Parallel()

	raw := `{"order":"202780839","status":"PROCESSED","accrual":729.98}`

	var info accrualclient.OrderInfo
	require.NoError(t, json.Unmarshal([]byte(raw), &info))
	require.Equal(t, accrualclient.StatusProcessed, info.Status)
	require.NotNil(t, info.Accrual)
	require.InDelta(t, 729.98, float64(*info.Accrual), 0.01)
}

func TestOrderInfo_JSONContractWithoutAccrual(t *testing.T) {
	t.Parallel()

	raw := `{"order":"79927398713","status":"REGISTERED"}`

	var info accrualclient.OrderInfo
	require.NoError(t, json.Unmarshal([]byte(raw), &info))
	require.Equal(t, accrualclient.StatusRegistered, info.Status)
	require.Nil(t, info.Accrual)
}
