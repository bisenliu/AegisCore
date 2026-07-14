package main

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	authdomain "github.com/aegiscore/user-service/internal/features/auth/domain"
	roleseed "github.com/aegiscore/user-service/internal/features/role/application/seed"
	usercommand "github.com/aegiscore/user-service/internal/features/user/application/command"
	userdomain "github.com/aegiscore/user-service/internal/features/user/domain"
	"github.com/aegiscore/user-service/internal/shared/identity"
)

func TestCreateSuperAdmin(t *testing.T) {
	t.Run("creates missing user and assigns role", func(t *testing.T) {
		t.Setenv("ADMIN_SECRET", " secret ")
		userID := uuid.MustParse("018f0000-0000-7000-8000-000000000301")
		var createdCmd usercommand.CreateUserCommand
		assignCalled := false
		deps := rbacSeedDependencies{
			service: newRBACSeedServiceMock(t, nil, func(_ context.Context, got uuid.UUID) (roleseed.AssignSuperAdminResult, error) {
				assignCalled = true
				require.Equal(t, userID, got)
				return roleseed.AssignSuperAdminResult{Added: true}, nil
			}),
			users: newCreateUserServiceMock(t, func(_ context.Context, cmd usercommand.CreateUserCommand) (*usercommand.CreateUserResult, error) {
				createdCmd = cmd
				return &usercommand.CreateUserResult{User: userdomain.User{UserID: userID}}, nil
			}),
			credentials: newRBACCredentialStoreMock(t, func(_ context.Context, username string) (*authdomain.UserCredential, error) {
				require.Equal(t, "admin", username)
				return nil, identity.ErrUserNotFound
			}, nil),
		}

		result, err := createSuperAdmin(context.Background(), deps, rbacCreateSuperAdminOptions{username: " ADMIN ", nickname: " Root ", passwordEnv: "ADMIN_SECRET"})

		require.NoError(t, err)
		require.Equal(t, userID, result.userID)
		require.True(t, result.created)
		require.True(t, result.roleAdded)
		require.False(t, result.passwordUpdated)
		require.True(t, assignCalled)
		require.Equal(t, "admin", createdCmd.Username)
		require.Equal(t, "Root", createdCmd.Nickname)
		require.Equal(t, "secret", createdCmd.Password)
		require.NotNil(t, createdCmd.Status)
		require.Equal(t, identity.UserStatusNormal, *createdCmd.Status)
	})

	t.Run("binds existing user without password reset", func(t *testing.T) {
		t.Setenv("ADMIN_SECRET", "secret")
		userID := uuid.MustParse("018f0000-0000-7000-8000-000000000302")
		deps := rbacSeedDependencies{
			service: newRBACSeedServiceMock(t, nil, func(_ context.Context, got uuid.UUID) (roleseed.AssignSuperAdminResult, error) {
				require.Equal(t, userID, got)
				return roleseed.AssignSuperAdminResult{Added: false}, nil
			}),
			credentials: newRBACCredentialStoreMock(t, func(_ context.Context, username string) (*authdomain.UserCredential, error) {
				require.Equal(t, "admin", username)
				return &authdomain.UserCredential{UserID: userID, Username: username, Status: identity.UserStatusNormal}, nil
			}, nil),
		}

		result, err := createSuperAdmin(context.Background(), deps, rbacCreateSuperAdminOptions{username: "admin", passwordEnv: "ADMIN_SECRET"})

		require.NoError(t, err)
		require.Equal(t, userID, result.userID)
		require.False(t, result.created)
		require.False(t, result.passwordUpdated)
		require.False(t, result.roleAdded)
	})

	t.Run("resets existing user password before assigning role", func(t *testing.T) {
		t.Setenv("ADMIN_SECRET", " secret ")
		userID := uuid.MustParse("018f0000-0000-7000-8000-000000000303")
		var updateInput authdomain.UpdateCredentialsInput
		deps := rbacSeedDependencies{
			service: newRBACSeedServiceMock(t, nil, func(_ context.Context, got uuid.UUID) (roleseed.AssignSuperAdminResult, error) {
				require.Equal(t, userID, got)
				return roleseed.AssignSuperAdminResult{Added: true}, nil
			}),
			credentials: newRBACCredentialStoreMock(t, func(context.Context, string) (*authdomain.UserCredential, error) {
				return &authdomain.UserCredential{UserID: userID, Username: "admin", Status: identity.UserStatusMustChangePassword}, nil
			}, func(_ context.Context, input authdomain.UpdateCredentialsInput) (int64, error) {
				updateInput = input
				return 3, nil
			}),
			passwordService: newRBACPasswordHasherMock(t, func(_ context.Context, plain string) (string, error) {
				require.Equal(t, "secret", plain)
				return "hashed-secret", nil
			}),
		}

		result, err := createSuperAdmin(context.Background(), deps, rbacCreateSuperAdminOptions{username: "admin", passwordEnv: "ADMIN_SECRET", resetPassword: true})

		require.NoError(t, err)
		require.Equal(t, userID, result.userID)
		require.True(t, result.passwordUpdated)
		require.True(t, result.roleAdded)
		require.Equal(t, userID, updateInput.UserID)
		require.Equal(t, "hashed-secret", updateInput.PasswordHash)
		require.Equal(t, identity.UserStatusNormal, updateInput.Status)
	})
}

func TestCreateSuperAdminPropagatesErrors(t *testing.T) {
	t.Run("credential read error", func(t *testing.T) {
		t.Setenv("ADMIN_SECRET", "secret")
		getErr := errors.New("credential store failed")
		deps := rbacSeedDependencies{
			credentials: newRBACCredentialStoreMock(t, func(context.Context, string) (*authdomain.UserCredential, error) {
				return nil, getErr
			}, nil),
		}
		_, err := createSuperAdmin(context.Background(), deps, rbacCreateSuperAdminOptions{username: "admin", passwordEnv: "ADMIN_SECRET"})
		require.ErrorIs(t, err, getErr)
	})

	t.Run("create user error", func(t *testing.T) {
		t.Setenv("ADMIN_SECRET", "secret")
		createErr := errors.New("create failed")
		deps := rbacSeedDependencies{
			users: newCreateUserServiceMock(t, func(context.Context, usercommand.CreateUserCommand) (*usercommand.CreateUserResult, error) {
				return nil, createErr
			}),
			credentials: newRBACCredentialStoreMock(t, func(context.Context, string) (*authdomain.UserCredential, error) {
				return nil, identity.ErrUserNotFound
			}, nil),
		}
		_, err := createSuperAdmin(context.Background(), deps, rbacCreateSuperAdminOptions{username: "admin", passwordEnv: "ADMIN_SECRET"})
		require.ErrorIs(t, err, createErr)
	})

	t.Run("hash error", func(t *testing.T) {
		t.Setenv("ADMIN_SECRET", "secret")
		hashErr := errors.New("hash failed")
		userID := uuid.MustParse("018f0000-0000-7000-8000-000000000401")
		deps := rbacSeedDependencies{
			credentials: newRBACCredentialStoreMock(t, func(context.Context, string) (*authdomain.UserCredential, error) {
				return &authdomain.UserCredential{UserID: userID}, nil
			}, nil),
			passwordService: newRBACPasswordHasherMock(t, func(context.Context, string) (string, error) {
				return "", hashErr
			}),
		}
		_, err := createSuperAdmin(context.Background(), deps, rbacCreateSuperAdminOptions{username: "admin", passwordEnv: "ADMIN_SECRET", resetPassword: true})
		require.ErrorIs(t, err, hashErr)
		require.ErrorContains(t, err, "hash create super admin password")
	})

	t.Run("update credentials error", func(t *testing.T) {
		t.Setenv("ADMIN_SECRET", "secret")
		updateErr := errors.New("update failed")
		userID := uuid.MustParse("018f0000-0000-7000-8000-000000000402")
		deps := rbacSeedDependencies{
			credentials: newRBACCredentialStoreMock(t, func(context.Context, string) (*authdomain.UserCredential, error) {
				return &authdomain.UserCredential{UserID: userID}, nil
			}, func(context.Context, authdomain.UpdateCredentialsInput) (int64, error) {
				return 0, updateErr
			}),
			passwordService: newRBACPasswordHasherMock(t, func(context.Context, string) (string, error) {
				return "hashed-secret", nil
			}),
		}
		_, err := createSuperAdmin(context.Background(), deps, rbacCreateSuperAdminOptions{username: "admin", passwordEnv: "ADMIN_SECRET", resetPassword: true})
		require.ErrorIs(t, err, updateErr)
	})

	t.Run("assign role error", func(t *testing.T) {
		t.Setenv("ADMIN_SECRET", "secret")
		assignErr := errors.New("assign failed")
		userID := uuid.MustParse("018f0000-0000-7000-8000-000000000403")
		deps := rbacSeedDependencies{
			service: newRBACSeedServiceMock(t, nil, func(context.Context, uuid.UUID) (roleseed.AssignSuperAdminResult, error) {
				return roleseed.AssignSuperAdminResult{}, assignErr
			}),
			credentials: newRBACCredentialStoreMock(t, func(context.Context, string) (*authdomain.UserCredential, error) {
				return &authdomain.UserCredential{UserID: userID}, nil
			}, nil),
		}
		_, err := createSuperAdmin(context.Background(), deps, rbacCreateSuperAdminOptions{username: "admin", passwordEnv: "ADMIN_SECRET"})
		require.ErrorIs(t, err, assignErr)
	})
}

func TestNormalizeCreateSuperAdminOptionsDefaultsAndValidation(t *testing.T) {
	t.Run("uses default password env", func(t *testing.T) {
		t.Setenv(defaultCreateSuperAdminPasswordEnv, " default-secret ")
		opts, err := normalizeCreateSuperAdminOptions(rbacCreateSuperAdminOptions{username: " ADMIN ", nickname: " ", resetPassword: true})
		require.NoError(t, err)
		require.Equal(t, "admin", opts.username)
		require.Equal(t, "admin", opts.nickname)
		require.Equal(t, "default-secret", opts.password)
		require.Equal(t, defaultCreateSuperAdminPasswordEnv, opts.passwordEnv)
		require.True(t, opts.resetPassword)
	})

	t.Run("requires username", func(t *testing.T) {
		t.Setenv("ADMIN_SECRET", "secret")
		_, err := normalizeCreateSuperAdminOptions(rbacCreateSuperAdminOptions{username: " ", passwordEnv: "ADMIN_SECRET"})
		require.ErrorContains(t, err, "admin username is required")
	})

	t.Run("requires password value", func(t *testing.T) {
		t.Setenv("EMPTY_ADMIN_PASSWORD", " ")
		_, err := normalizeCreateSuperAdminOptions(rbacCreateSuperAdminOptions{username: "admin", passwordEnv: "EMPTY_ADMIN_PASSWORD"})
		require.ErrorContains(t, err, "admin password is required")
	})
}

func TestNormalizeCreateSuperAdminOptionsRequiresPasswordEnv(t *testing.T) {
	t.Setenv(defaultCreateSuperAdminPasswordEnv, "")
	_, err := normalizeCreateSuperAdminOptions(rbacCreateSuperAdminOptions{username: "admin", nickname: "Admin", passwordEnv: "MISSING_ADMIN_PASSWORD"})
	require.ErrorContains(t, err, "MISSING_ADMIN_PASSWORD")
}

func TestNormalizeCreateSuperAdminOptionsReadsPasswordEnv(t *testing.T) {
	t.Setenv("ADMIN_SECRET", "  secret  ")
	opts, err := normalizeCreateSuperAdminOptions(rbacCreateSuperAdminOptions{username: " ADMIN ", nickname: " ", passwordEnv: "ADMIN_SECRET", resetPassword: true})
	require.NoError(t, err)
	assert.Equal(t, "admin", opts.username)
	assert.Equal(t, "admin", opts.nickname)
	assert.Equal(t, "secret", opts.password)
	assert.Equal(t, "ADMIN_SECRET", opts.passwordEnv)
	assert.True(t, opts.resetPassword)
}

func TestNormalizeUsername(t *testing.T) {
	require.Equal(t, "admin", normalizeUsername(" ADMIN "))
	require.Equal(t, "", normalizeUsername(" "))
}
