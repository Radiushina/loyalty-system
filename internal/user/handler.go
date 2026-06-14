package user

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/Radiushina/loyalty-system/internal/httputil"
	"go.uber.org/zap"
)

type (
	Handler struct {
		service ServiceProvider
		log     *zap.Logger
	}

	ServiceProvider interface {
		CreateUser(ctx context.Context, login, password string) (AuthUserRes, error)
		GetByLogin(ctx context.Context, login, password string) (AuthUserRes, error)
	}
)

func NewHandler(service ServiceProvider, log *zap.Logger) *Handler {
	return &Handler{
		service: service,
		log:     log,
	}
}

func (h *Handler) CreateUser(ctx context.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer func() { _ = r.Body.Close() }()

		b, err := io.ReadAll(r.Body)
		if err != nil {
			httputil.WriteError(w, http.StatusBadRequest, "failed to read body")
			return
		}

		var u AuthUserReq
		if err := json.Unmarshal(b, &u); err != nil {
			httputil.WriteError(w, http.StatusBadRequest, "invalid JSON")
			return
		}

		user, err := h.service.CreateUser(ctx, u.Login, u.Password)
		if err != nil {
			if errors.Is(err, ErrUserAlreadyExists) {
				httputil.WriteError(w, http.StatusConflict, "user already exists")
				return
			}
			if errors.Is(err, ErrInvalidCredentials) {
				httputil.WriteError(w, http.StatusBadRequest, "invalid credentials")
				return
			}
			h.log.Error("create user", zap.Error(err))
			httputil.WriteError(w, http.StatusInternalServerError, "internal server error")
			return
		}

		if err := writeAuthSession(w, user); err != nil {
			h.log.Error("encode response", zap.Error(err))
		}
	}
}

func (h *Handler) AuthUser(ctx context.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer func() { _ = r.Body.Close() }()

		b, err := io.ReadAll(r.Body)
		if err != nil {
			httputil.WriteError(w, http.StatusBadRequest, "failed to read body")
			return
		}

		var u AuthUserReq
		if err := json.Unmarshal(b, &u); err != nil {
			httputil.WriteError(w, http.StatusBadRequest, "invalid JSON")
			return
		}

		user, err := h.service.GetByLogin(ctx, u.Login, u.Password)
		if err != nil {
			if errors.Is(err, ErrUserNotFound) {
				httputil.WriteError(w, http.StatusUnauthorized, "user not found")
				return
			}
			if errors.Is(err, ErrInvalidCredentials) {
				httputil.WriteError(w, http.StatusUnauthorized, "invalid credentials")
				return
			}
			h.log.Error("authenticate user", zap.Error(err))
			httputil.WriteError(w, http.StatusInternalServerError, "internal server error")
			return
		}

		if err := writeAuthSession(w, user); err != nil {
			h.log.Error("encode response", zap.Error(err))
		}
	}
}

func writeAuthSession(w http.ResponseWriter, session AuthUserRes) error {
	w.Header().Set("Authorization", "Bearer "+session.Token)
	return httputil.WriteJSON(w, http.StatusOK, session)
}
