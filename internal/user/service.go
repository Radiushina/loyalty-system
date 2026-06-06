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
	}

	RepoProvider interface {
		CreateUser(ctx context.Context, ogin, password string) (User, error)
		GetByLogin(ctx context.Context, ogin, password string) (User, error)
	}

	TokenProvider interface {
		Generate(userID uuid.UUID) (string, error)
	}
)

func NewService(repo RepoProvider, tokens TokenProvider) *Service {
	return &Service{
		repo:   repo,
		tokens: tokens,
	}
}

func (s *Service) CreateUser(ctx context.Context, login, password string) (AuthSession, error) {
	if login == "" || password == "" {
		return AuthSession{}, fmt.Errorf("%w: login and password are required", ErrInvalidCredentials)
	}

	user, err := s.repo.CreateUser(ctx, login, password)
	if err != nil {
		return AuthSession{}, fmt.Errorf("create user: %w", err)
	}

	token, err := s.tokens.Generate(user.ID)
	if err != nil {
		return AuthSession{}, fmt.Errorf("generate token: %w", err)
	}

	return NewAuthSession(user, token), nil
}

func (s *Service) GetByLogin(ctx context.Context, login, password string) (AuthSession, error) {
	if login == "" || password == "" {
		return AuthSession{}, fmt.Errorf("%w: login and password are required", ErrInvalidCredentials)
	}

	user, err := s.repo.GetByLogin(ctx, login, password)
	token, err := s.tokens.Generate(user.ID)
	if err != nil {
		return AuthSession{}, fmt.Errorf("generate token: %w", err)
	}

	return NewAuthSession(user, token), nil
}
