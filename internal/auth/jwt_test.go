package auth_test

import (
	"testing"
	"time"

	"github.com/Radiushina/loyalty-system/internal/auth"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestJWT_Generate(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	provider := auth.NewJWT("test-secret", time.Hour)

	token, err := provider.Generate(userID)
	require.NoError(t, err)
	require.NotEmpty(t, token)

	parsed, err := jwt.Parse(token, func(token *jwt.Token) (any, error) {
		return []byte("test-secret"), nil
	})
	require.NoError(t, err)
	require.True(t, parsed.Valid)

	claims, ok := parsed.Claims.(jwt.MapClaims)
	require.True(t, ok)
	require.Equal(t, userID.String(), claims["sub"])
}
