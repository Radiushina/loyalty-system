package user

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

type (
	Service struct {
		repo   RepoProvider
		tokens TokenProvider
		hasher HasherProvider
	}

	RepoProvider interface {
		CreateUser(ctx context.Context, login, hashedPassword string) (User, error)
		GetByLogin(ctx context.Context, login string) (User, error)
	}

	TokenProvider interface {
		Generate(userID uuid.UUID) (string, error)
	}

	HasherProvider interface {
		Hash(plain string) (string, error)
		Compare(hash, plain string) error
	}
)

func NewService(repo RepoProvider, tokens TokenProvider, hasher HasherProvider) *Service {
	return &Service{
		repo:   repo,
		tokens: tokens,
		hasher: hasher,
	}
}

func (s *Service) CreateUser(ctx context.Context, login, password string) (AuthUserRes, error) {
	if login == "" || password == "" {
		return AuthUserRes{}, fmt.Errorf("%w: login and password are required", ErrInvalidCredentials)
	}

	hashed, err := s.hasher.Hash(password)

	if err != nil {
		return AuthUserRes{}, fmt.Errorf("hash password: %w", err)
	}

	user, err := s.repo.CreateUser(ctx, login, hashed)
	if err != nil {
		return AuthUserRes{}, fmt.Errorf("create user: %w", err)
	}

	token, err := s.tokens.Generate(user.ID)
	if err != nil {
		return AuthUserRes{}, fmt.Errorf("generate token: %w", err)
	}

	return NewAuthSession(user, token), nil
}

func (s *Service) GetByLogin(ctx context.Context, login, password string) (AuthUserRes, error) {
	if login == "" || password == "" {
		return AuthUserRes{}, fmt.Errorf("%w: login and password are required", ErrInvalidCredentials)
	}

	user, err := s.repo.GetByLogin(ctx, login)
	if err != nil {
		return AuthUserRes{}, fmt.Errorf("authenticate: %w", err)
	}

	if err := s.hasher.Compare(user.Password, password); err != nil {
		return AuthUserRes{}, ErrInvalidCredentials
	}

	token, err := s.tokens.Generate(user.ID)
	if err != nil {
		return AuthUserRes{}, fmt.Errorf("generate token: %w", err)
	}

	return NewAuthSession(user, token), nil
}
