package user

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"go.uber.org/zap"
)

type (
	Handler struct {
		service ServiceProvider
		log     *zap.Logger
	}

	ServiceProvider interface {
		CreateUser(ctx context.Context, login, password string) (AuthSession, error)
		GetByLogin(ctx context.Context, login, password string) (AuthSession, error)
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
			writeError(w, http.StatusBadRequest, "failed to read body")
			return
		}

		var u UserAuth
		if err := json.Unmarshal(b, &u); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON")
			return
		}

		user, err := h.service.CreateUser(ctx, u.Login, u.Password)
		if err != nil {
			if errors.Is(err, ErrUserAlreadyExists) {
				writeError(w, http.StatusConflict, "user already exists")
				return
			}
			if errors.Is(err, ErrInvalidCredentials) {
				writeError(w, http.StatusBadRequest, "invalid credentials")
				return
			}
			h.log.Error("create user", zap.Error(err))
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}

		if err := writeJSON(w, http.StatusOK, user); err != nil {
			h.log.Error("encode response", zap.Error(err))
		}
	}
}

func (h *Handler) GetByLogin(ctx context.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer func() { _ = r.Body.Close() }()

		b, err := io.ReadAll(r.Body)
		if err != nil {
			writeError(w, http.StatusBadRequest, "failed to read body")
			return
		}

		var u UserAuth
		if err := json.Unmarshal(b, &u); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON")
			return
		}

		user, err := h.service.GetByLogin(ctx, u.Login, u.Password)
		if err != nil {
			if errors.Is(err, ErrUserNotFound) {
				writeError(w, http.StatusUnauthorized, "user not found")
				return
			}
			if errors.Is(err, ErrInvalidCredentials) {
				writeError(w, http.StatusUnauthorized, "invalid credentials")
				return
			}
			h.log.Error("authenticate user", zap.Error(err))
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}

		if err := writeJSON(w, http.StatusOK, user); err != nil {
			h.log.Error("encode response", zap.Error(err))
		}
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) error {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	return json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, message string) {
	_ = writeJSON(w, status, map[string]string{"msg": message})
}
