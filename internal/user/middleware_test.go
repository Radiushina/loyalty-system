package user_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Radiushina/loyalty-system/internal/auth"
	"github.com/Radiushina/loyalty-system/internal/user"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestNewAuthMiddleware(t *testing.T) {
	t.Parallel()

	jwtProvider := auth.NewJWT("test-secret", time.Hour)
	userID := uuid.New()

	validToken, err := jwtProvider.Generate(userID)
	require.NoError(t, err)

	handler := user.NewAuthMiddleware(jwtProvider)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got, ok := user.UserIDFromContext(r.Context())
		require.True(t, ok)
		require.Equal(t, userID, got)
		w.WriteHeader(http.StatusOK)
	}))

	tests := []struct {
		name           string
		authHeader     string
		wantStatus     int
		wantBodySubstr string
	}{
		{
			name:       "success",
			authHeader: "Bearer " + validToken,
			wantStatus: http.StatusOK,
		},
		{
			name:           "missing header",
			wantStatus:     http.StatusUnauthorized,
			wantBodySubstr: "unauthorized",
		},
		{
			name:           "invalid token",
			authHeader:     "Bearer invalid",
			wantStatus:     http.StatusUnauthorized,
			wantBodySubstr: "unauthorized",
		},
		{
			name:           "invalid scheme",
			authHeader:     "Basic " + validToken,
			wantStatus:     http.StatusUnauthorized,
			wantBodySubstr: "unauthorized",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodGet, "/api/user/orders", nil)
			if tc.authHeader != "" {
				req.Header.Set("Authorization", tc.authHeader)
			}

			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			require.Equal(t, tc.wantStatus, rec.Code)
			if tc.wantBodySubstr != "" {
				require.Contains(t, rec.Body.String(), tc.wantBodySubstr)
			}
		})
	}
}
