package user

import (
	"errors"

	"github.com/google/uuid"
)

var (
	ErrUserNotFound       = errors.New("user not found")
	ErrUserAlreadyExists  = errors.New("user already exists") // 409
	ErrUnauthorized       = errors.New("unauthorized")
	ErrInvalidCredentials = errors.New("invalid credentials") //401
)

type User struct {
	ID       uuid.UUID `db:"id" json:"id"`
	Login    string    `db:"login" json:"login"`
	Password string    `db:"-" json:"-"`
}

type UserAuth struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

type UserResponse struct {
	ID    uuid.UUID `json:"id"`
	Login string    `json:"login"`
}

type RegisterSuccess struct {
	User  UserResponse
	Token string
}
