package user

import (
	"context"
	"fmt"
)

type (
	Service struct {
		repo RepoProvider
	}

	RepoProvider interface {
		CreateUser(ctx context.Context, ogin, password string) (User, error)
		GetByLogin(ctx context.Context, ogin, password string) (User, error)
	}
)

func NewService(repo RepoProvider) *Service {
	return &Service{
		repo: repo,
	}
}

func (s *Service) CreateUser(ctx context.Context, login, password string) (User, error) {
	if login == "" || password == "" {
		return User{}, fmt.Errorf("%w: login and password are required", ErrInvalidCredentials)
	}

	user, err := s.repo.CreateUser(ctx, login, password)
	if err != nil {
		return User{}, fmt.Errorf("create user: %w", err)
	}

	return user, nil
}

func (s *Service) GetByLogin(ctx context.Context, login, password string) (User, error) {
	if login == "" || password == "" {
		return User{}, fmt.Errorf("%w: login and password are required", ErrInvalidCredentials)
	}

	user, err := s.repo.GetByLogin(ctx, login, password)
	if err != nil {
		return User{}, fmt.Errorf("authenticate: %w", err)
	}

	return user, nil
}
