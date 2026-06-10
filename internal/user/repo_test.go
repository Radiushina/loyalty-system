package user_test

import (
	"testing"

	"github.com/Radiushina/loyalty-system/internal/user"
	"github.com/Radiushina/loyalty-system/pkg/tests/containers/postgres"
	"github.com/stretchr/testify/require"
)

func TestRepo_Insert(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		login   string
		pass    string
		prepare func(t *testing.T, repo *user.UsersRepo)
		wantErr error
	}{
		{
			name:    "Success insert user",
			login:   "user1",
			pass:    "password",
			wantErr: nil,
		},
		{
			name:  "Not unique login",
			login: "user1",
			pass:  "password2",
			prepare: func(t *testing.T, repo *user.UsersRepo) {
				t.Helper()
				_, err := repo.CreateUser(t.Context(), "user1", "password")
				require.NoError(t, err)
			},
			wantErr: user.ErrUserAlreadyExists,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			pool, _, _ := postgres.New(t)
			repo := user.NewRepository(pool)

			if tc.prepare != nil {
				tc.prepare(t, repo)
			}

			created, err := repo.CreateUser(t.Context(), tc.login, tc.pass)
			if tc.wantErr != nil {
				require.Error(t, err)
				require.ErrorIs(t, err, tc.wantErr)
				return
			}

			require.NoError(t, err)
			require.NotEmpty(t, created.ID)
			require.Equal(t, tc.login, created.Login)
		})
	}
}

func TestRepo_SelectRow(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		login   string
		pass    string
		prepare func(t *testing.T, repo *user.UsersRepo) user.User
		wantErr error
	}{
		{
			name:  "Success get user",
			login: "user1",
			pass:  "password",
			prepare: func(t *testing.T, repo *user.UsersRepo) user.User {
				t.Helper()
				created, err := repo.CreateUser(t.Context(), "user1", "password")
				require.NoError(t, err)
				return created
			},
			wantErr: nil,
		},
		{
			name:  "Invalid credentials wrong password",
			login: "user1",
			pass:  "wrong",
			prepare: func(t *testing.T, repo *user.UsersRepo) user.User {
				t.Helper()
				_, err := repo.CreateUser(t.Context(), "user1", "password")
				require.NoError(t, err)
				return user.User{}
			},
			wantErr: user.ErrInvalidCredentials,
		},
		{
			name:    "User not found",
			login:   "unknown",
			pass:    "password",
			wantErr: user.ErrUserNotFound,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			pool, _, _ := postgres.New(t)
			repo := user.NewRepository(pool)

			var want user.User
			if tc.prepare != nil {
				want = tc.prepare(t, repo)
			}

			found, err := repo.GetByLogin(t.Context(), tc.login, tc.pass)
			if tc.wantErr != nil {
				require.Error(t, err)
				require.ErrorIs(t, err, tc.wantErr)
				return
			}

			require.NoError(t, err)
			require.Equal(t, want, found)
		})
	}
}
