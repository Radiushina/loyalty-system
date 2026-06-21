package user_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Radiushina/loyalty-system/internal/user"
	"github.com/Radiushina/loyalty-system/internal/user/user_mocks"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func newHandler(svc *user_mocks.ServiceProvider) *user.Handler {
	return user.NewHandler(svc, zap.NewNop())
}

func TestHandler_CreateUser(t *testing.T) {
	t.Parallel()

	userID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	login := "user1"
	password := "test"
	token := "Test123"
	session := user.AuthUserRes{
		User: user.UserResponse{
			ID:    userID,
			Login: login,
		},
		Token: token,
	}

	tests := []struct {
		name           string
		body           string
		prepare        func(svc *user_mocks.ServiceProvider)
		wantStatus     int
		wantSession    *user.AuthUserRes
		wantAuthHeader string
	}{
		{
			name: "Success create user",
			body: `{"login":"user1","password":"test"}`,
			prepare: func(svc *user_mocks.ServiceProvider) {
				svc.EXPECT().
					CreateUser(mock.Anything, login, password).
					Return(session, nil)
			},
			wantStatus:     http.StatusOK,
			wantSession:    &session,
			wantAuthHeader: "Bearer " + token,
		},
		{
			name:       "Invalid JSON",
			body:       `{`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "User already exists",
			body: `{"login":"user1","password":"test"}`,
			prepare: func(svc *user_mocks.ServiceProvider) {
				svc.EXPECT().
					CreateUser(mock.Anything, login, password).
					Return(user.AuthUserRes{}, user.ErrUserAlreadyExists)
			},
			wantStatus: http.StatusConflict,
		},
		{
			name: "Invalid credentials",
			body: `{"login":"user1","password":"short"}`,
			prepare: func(svc *user_mocks.ServiceProvider) {
				svc.EXPECT().
					CreateUser(mock.Anything, login, "short").
					Return(user.AuthUserRes{}, user.ErrInvalidCredentials)
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "Internal server error",
			body: `{"login":"user1","password":"test"}`,
			prepare: func(svc *user_mocks.ServiceProvider) {
				svc.EXPECT().
					CreateUser(mock.Anything, login, password).
					Return(user.AuthUserRes{}, errors.New("db error"))
			},
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			svc := user_mocks.NewServiceProvider(t)
			if tc.prepare != nil {
				tc.prepare(svc)
			}

			h := newHandler(svc)
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(
				http.MethodPost,
				"/api/user/register",
				strings.NewReader(tc.body),
			)
			req.Header.Set("Content-Type", "application/json")

			h.CreateUser()(rec, req)

			require.Equal(t, tc.wantStatus, rec.Code)
			if tc.wantSession == nil {
				return
			}

			require.Equal(t, tc.wantAuthHeader, rec.Header().Get("Authorization"))

			var got user.AuthUserRes
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
			require.Equal(t, *tc.wantSession, got)
		})
	}
}

func TestHandler_AuthUser(t *testing.T) {
	t.Parallel()

	userID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	login := "user1"
	password := "test"
	token := "Test123"
	session := user.AuthUserRes{
		User: user.UserResponse{
			ID:    userID,
			Login: login,
		},
		Token: token,
	}

	tests := []struct {
		name           string
		body           string
		prepare        func(svc *user_mocks.ServiceProvider)
		wantStatus     int
		wantSession    *user.AuthUserRes
		wantAuthHeader string
	}{
		{
			name: "Success auth",
			body: `{"login":"user1","password":"test"}`,
			prepare: func(svc *user_mocks.ServiceProvider) {
				svc.EXPECT().
					GetByLogin(mock.Anything, login, password).
					Return(session, nil)
			},
			wantStatus:     http.StatusOK,
			wantSession:    &session,
			wantAuthHeader: "Bearer " + token,
		},
		{
			name:       "Invalid JSON",
			body:       `{`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "User not found",
			body: `{"login":"user1","password":"test"}`,
			prepare: func(svc *user_mocks.ServiceProvider) {
				svc.EXPECT().
					GetByLogin(mock.Anything, login, password).
					Return(user.AuthUserRes{}, user.ErrUserNotFound)
			},
			wantStatus: http.StatusUnauthorized,
		},
		{
			name: "Invalid credentials",
			body: `{"login":"user1","password":"wrong"}`,
			prepare: func(svc *user_mocks.ServiceProvider) {
				svc.EXPECT().
					GetByLogin(mock.Anything, login, "wrong").
					Return(user.AuthUserRes{}, user.ErrInvalidCredentials)
			},
			wantStatus: http.StatusUnauthorized,
		},
		{
			name: "Internal server error",
			body: `{"login":"user1","password":"test"}`,
			prepare: func(svc *user_mocks.ServiceProvider) {
				svc.EXPECT().
					GetByLogin(mock.Anything, login, password).
					Return(user.AuthUserRes{}, errors.New("db error"))
			},
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			svc := user_mocks.NewServiceProvider(t)
			if tc.prepare != nil {
				tc.prepare(svc)
			}

			h := newHandler(svc)
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(
				http.MethodPost,
				"/api/user/login",
				strings.NewReader(tc.body),
			)
			req.Header.Set("Content-Type", "application/json")

			h.AuthUser()(rec, req)

			require.Equal(t, tc.wantStatus, rec.Code)
			if tc.wantSession == nil {
				return
			}

			require.Equal(t, tc.wantAuthHeader, rec.Header().Get("Authorization"))

			var got user.AuthUserRes
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
			require.Equal(t, *tc.wantSession, got)
		})
	}
}
