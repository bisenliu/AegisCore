package main

import (
	"context"
	"errors"
	"io"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	authdomain "github.com/aegiscore/user-service/internal/features/auth/domain"
	roleseed "github.com/aegiscore/user-service/internal/features/role/application/seed"
	usercommand "github.com/aegiscore/user-service/internal/features/user/application/command"
	userdomain "github.com/aegiscore/user-service/internal/features/user/domain"
	"github.com/aegiscore/user-service/internal/shared/identity"
)

func TestRunRBACSeedCommand(t *testing.T) {
	t.Run("success passes options and cleans up", func(t *testing.T) {
		seedCalled := false
		cleanupCalled := false
		withRBACSeedDependencyFactory(t, func(_ context.Context, configPath string) (rbacSeedDependencies, func() error, error) {
			require.Equal(t, "test-config.yaml", configPath)
			return rbacSeedDependencies{
					service: fakeRBACSeedService{
						seed: func(_ context.Context, opts roleseed.SeedOptions) (roleseed.SeedResult, error) {
							seedCalled = true
							require.True(t, opts.ReactivateSystem)
							require.True(t, opts.SyncSystemBindings)
							return roleseed.SeedResult{
								RolesInserted:             1,
								RolesUpdated:              2,
								PermissionsInserted:       3,
								PermissionsUpdated:        4,
								RolePermissionBindingsAdd: 5,
								RolePermissionBindingsDel: 6,
							}, nil
						},
					},
				}, func() error {
					cleanupCalled = true
					return nil
				}, nil
		})

		out, err := captureStdout(t, func() error {
			return runRBACSeedCommand(context.Background(), "test-config.yaml", rbacSeedOptions{reactivateSystem: true, syncSystemBindings: true})
		})

		require.NoError(t, err)
		require.True(t, seedCalled)
		require.True(t, cleanupCalled)
		require.Contains(t, out, "RBAC seed complete: roles inserted=1 updated=2 permissions inserted=3 updated=4 bindings added=5 removed=6")
	})

	t.Run("dependency error returns before cleanup", func(t *testing.T) {
		initErr := errors.New("load config failed")
		withRBACSeedDependencyFactory(t, func(context.Context, string) (rbacSeedDependencies, func() error, error) {
			return rbacSeedDependencies{}, nil, initErr
		})

		err := runRBACSeedCommand(context.Background(), "bad.yaml", rbacSeedOptions{})

		require.ErrorIs(t, err, initErr)
	})

	t.Run("seed and cleanup errors are joined", func(t *testing.T) {
		seedErr := errors.New("seed failed")
		cleanupErr := errors.New("cleanup failed")
		withRBACSeedDependencyFactory(t, func(context.Context, string) (rbacSeedDependencies, func() error, error) {
			return rbacSeedDependencies{
					service: fakeRBACSeedService{
						seed: func(context.Context, roleseed.SeedOptions) (roleseed.SeedResult, error) {
							return roleseed.SeedResult{}, seedErr
						},
					},
				}, func() error {
					return cleanupErr
				}, nil
		})

		err := runRBACSeedCommand(context.Background(), "test-config.yaml", rbacSeedOptions{})

		require.ErrorIs(t, err, seedErr)
		require.ErrorIs(t, err, cleanupErr)
	})
}

func TestRunAssignSuperAdminCommand(t *testing.T) {
	userID := uuid.MustParse("018f0000-0000-7000-8000-000000000101")

	for _, tc := range []struct {
		name       string
		added      bool
		wantOutput string
	}{
		{
			name:       "new binding",
			added:      true,
			wantOutput: "Super admin role assigned to user " + userID.String(),
		},
		{
			name:       "existing binding",
			added:      false,
			wantOutput: "Super admin role already assigned to user " + userID.String(),
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			assignCalled := false
			cleanupCalled := false
			withRBACSeedDependencyFactory(t, func(_ context.Context, configPath string) (rbacSeedDependencies, func() error, error) {
				require.Equal(t, "test-config.yaml", configPath)
				return rbacSeedDependencies{
						service: fakeRBACSeedService{
							assign: func(_ context.Context, got uuid.UUID) (roleseed.AssignSuperAdminResult, error) {
								assignCalled = true
								require.Equal(t, userID, got)
								return roleseed.AssignSuperAdminResult{Added: tc.added}, nil
							},
						},
					}, func() error {
						cleanupCalled = true
						return nil
					}, nil
			})

			out, err := captureStdout(t, func() error {
				return runAssignSuperAdminCommand(context.Background(), "test-config.yaml", userID)
			})

			require.NoError(t, err)
			require.True(t, assignCalled)
			require.True(t, cleanupCalled)
			require.Contains(t, out, tc.wantOutput)
		})
	}

	t.Run("dependency error", func(t *testing.T) {
		initErr := errors.New("postgres config missing")
		withRBACSeedDependencyFactory(t, func(context.Context, string) (rbacSeedDependencies, func() error, error) {
			return rbacSeedDependencies{}, nil, initErr
		})

		err := runAssignSuperAdminCommand(context.Background(), "bad.yaml", userID)

		require.ErrorIs(t, err, initErr)
	})

	t.Run("service error still cleans up", func(t *testing.T) {
		assignErr := errors.New("assign failed")
		cleanupCalled := false
		withRBACSeedDependencyFactory(t, func(context.Context, string) (rbacSeedDependencies, func() error, error) {
			return rbacSeedDependencies{
					service: fakeRBACSeedService{
						assign: func(context.Context, uuid.UUID) (roleseed.AssignSuperAdminResult, error) {
							return roleseed.AssignSuperAdminResult{}, assignErr
						},
					},
				}, func() error {
					cleanupCalled = true
					return nil
				}, nil
		})

		err := runAssignSuperAdminCommand(context.Background(), "test-config.yaml", userID)

		require.ErrorIs(t, err, assignErr)
		require.True(t, cleanupCalled)
	})
}

func TestRunCreateSuperAdminCommand(t *testing.T) {
	userID := uuid.MustParse("018f0000-0000-7000-8000-000000000201")

	t.Run("success prints normalized username", func(t *testing.T) {
		t.Setenv("ADMIN_SECRET", " secret ")
		cleanupCalled := false
		withRBACSeedDependencyFactory(t, func(_ context.Context, configPath string) (rbacSeedDependencies, func() error, error) {
			require.Equal(t, "test-config.yaml", configPath)
			return rbacSeedDependencies{
					service: fakeRBACSeedService{
						assign: func(_ context.Context, got uuid.UUID) (roleseed.AssignSuperAdminResult, error) {
							require.Equal(t, userID, got)
							return roleseed.AssignSuperAdminResult{Added: true}, nil
						},
					},
					credentials: fakeRBACCredentialStore{
						getByUsername: func(_ context.Context, username string) (*authdomain.UserCredential, error) {
							require.Equal(t, "admin", username)
							return &authdomain.UserCredential{UserID: userID, Username: username, Status: identity.UserStatusNormal}, nil
						},
					},
				}, func() error {
					cleanupCalled = true
					return nil
				}, nil
		})

		out, err := captureStdout(t, func() error {
			return runCreateSuperAdminCommand(context.Background(), "test-config.yaml", rbacCreateSuperAdminOptions{username: " ADMIN ", passwordEnv: "ADMIN_SECRET"})
		})

		require.NoError(t, err)
		require.True(t, cleanupCalled)
		require.Contains(t, out, "Super admin create complete: username=admin user_id="+userID.String()+" created=false password_updated=false super_admin_role_added=true")
	})

	t.Run("dependency error", func(t *testing.T) {
		initErr := errors.New("init failed")
		withRBACSeedDependencyFactory(t, func(context.Context, string) (rbacSeedDependencies, func() error, error) {
			return rbacSeedDependencies{}, nil, initErr
		})

		err := runCreateSuperAdminCommand(context.Background(), "bad.yaml", rbacCreateSuperAdminOptions{})

		require.ErrorIs(t, err, initErr)
	})

	t.Run("create and cleanup errors are joined", func(t *testing.T) {
		t.Setenv("ADMIN_SECRET", "secret")
		getErr := errors.New("credential read failed")
		cleanupErr := errors.New("cleanup failed")
		withRBACSeedDependencyFactory(t, func(context.Context, string) (rbacSeedDependencies, func() error, error) {
			return rbacSeedDependencies{
					credentials: fakeRBACCredentialStore{
						getByUsername: func(context.Context, string) (*authdomain.UserCredential, error) {
							return nil, getErr
						},
					},
				}, func() error {
					return cleanupErr
				}, nil
		})

		err := runCreateSuperAdminCommand(context.Background(), "test-config.yaml", rbacCreateSuperAdminOptions{username: "admin", passwordEnv: "ADMIN_SECRET"})

		require.ErrorIs(t, err, getErr)
		require.ErrorIs(t, err, cleanupErr)
	})
}

func TestCreateSuperAdmin(t *testing.T) {
	t.Run("creates missing user and assigns role", func(t *testing.T) {
		t.Setenv("ADMIN_SECRET", " secret ")
		userID := uuid.MustParse("018f0000-0000-7000-8000-000000000301")
		var createdCmd usercommand.CreateUserCommand
		assignCalled := false
		deps := rbacSeedDependencies{
			service: fakeRBACSeedService{
				assign: func(_ context.Context, got uuid.UUID) (roleseed.AssignSuperAdminResult, error) {
					assignCalled = true
					require.Equal(t, userID, got)
					return roleseed.AssignSuperAdminResult{Added: true}, nil
				},
			},
			users: fakeCreateUserService{
				create: func(_ context.Context, cmd usercommand.CreateUserCommand) (*usercommand.CreateUserResult, error) {
					createdCmd = cmd
					return &usercommand.CreateUserResult{User: userdomain.User{UserID: userID}}, nil
				},
			},
			credentials: fakeRBACCredentialStore{
				getByUsername: func(_ context.Context, username string) (*authdomain.UserCredential, error) {
					require.Equal(t, "admin", username)
					return nil, identity.ErrUserNotFound
				},
			},
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
			service: fakeRBACSeedService{
				assign: func(_ context.Context, got uuid.UUID) (roleseed.AssignSuperAdminResult, error) {
					require.Equal(t, userID, got)
					return roleseed.AssignSuperAdminResult{Added: false}, nil
				},
			},
			credentials: fakeRBACCredentialStore{
				getByUsername: func(_ context.Context, username string) (*authdomain.UserCredential, error) {
					require.Equal(t, "admin", username)
					return &authdomain.UserCredential{UserID: userID, Username: username, Status: identity.UserStatusNormal}, nil
				},
			},
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
			service: fakeRBACSeedService{
				assign: func(_ context.Context, got uuid.UUID) (roleseed.AssignSuperAdminResult, error) {
					require.Equal(t, userID, got)
					return roleseed.AssignSuperAdminResult{Added: true}, nil
				},
			},
			credentials: fakeRBACCredentialStore{
				getByUsername: func(context.Context, string) (*authdomain.UserCredential, error) {
					return &authdomain.UserCredential{UserID: userID, Username: "admin", Status: identity.UserStatusMustChangePassword}, nil
				},
				updateCredentials: func(_ context.Context, input authdomain.UpdateCredentialsInput) (int64, error) {
					updateInput = input
					return 3, nil
				},
			},
			passwordService: fakePasswordHasher{
				hash: func(_ context.Context, plain string) (string, error) {
					require.Equal(t, "secret", plain)
					return "hashed-secret", nil
				},
			},
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
			credentials: fakeRBACCredentialStore{
				getByUsername: func(context.Context, string) (*authdomain.UserCredential, error) {
					return nil, getErr
				},
			},
		}

		_, err := createSuperAdmin(context.Background(), deps, rbacCreateSuperAdminOptions{username: "admin", passwordEnv: "ADMIN_SECRET"})

		require.ErrorIs(t, err, getErr)
	})

	t.Run("create user error", func(t *testing.T) {
		t.Setenv("ADMIN_SECRET", "secret")
		createErr := errors.New("create failed")
		deps := rbacSeedDependencies{
			users: fakeCreateUserService{
				create: func(context.Context, usercommand.CreateUserCommand) (*usercommand.CreateUserResult, error) {
					return nil, createErr
				},
			},
			credentials: fakeRBACCredentialStore{
				getByUsername: func(context.Context, string) (*authdomain.UserCredential, error) {
					return nil, identity.ErrUserNotFound
				},
			},
		}

		_, err := createSuperAdmin(context.Background(), deps, rbacCreateSuperAdminOptions{username: "admin", passwordEnv: "ADMIN_SECRET"})

		require.ErrorIs(t, err, createErr)
	})

	t.Run("hash error", func(t *testing.T) {
		t.Setenv("ADMIN_SECRET", "secret")
		hashErr := errors.New("hash failed")
		userID := uuid.MustParse("018f0000-0000-7000-8000-000000000401")
		deps := rbacSeedDependencies{
			credentials: fakeRBACCredentialStore{
				getByUsername: func(context.Context, string) (*authdomain.UserCredential, error) {
					return &authdomain.UserCredential{UserID: userID}, nil
				},
			},
			passwordService: fakePasswordHasher{
				hash: func(context.Context, string) (string, error) {
					return "", hashErr
				},
			},
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
			credentials: fakeRBACCredentialStore{
				getByUsername: func(context.Context, string) (*authdomain.UserCredential, error) {
					return &authdomain.UserCredential{UserID: userID}, nil
				},
				updateCredentials: func(context.Context, authdomain.UpdateCredentialsInput) (int64, error) {
					return 0, updateErr
				},
			},
			passwordService: fakePasswordHasher{
				hash: func(context.Context, string) (string, error) {
					return "hashed-secret", nil
				},
			},
		}

		_, err := createSuperAdmin(context.Background(), deps, rbacCreateSuperAdminOptions{username: "admin", passwordEnv: "ADMIN_SECRET", resetPassword: true})

		require.ErrorIs(t, err, updateErr)
	})

	t.Run("assign role error", func(t *testing.T) {
		t.Setenv("ADMIN_SECRET", "secret")
		assignErr := errors.New("assign failed")
		userID := uuid.MustParse("018f0000-0000-7000-8000-000000000403")
		deps := rbacSeedDependencies{
			service: fakeRBACSeedService{
				assign: func(context.Context, uuid.UUID) (roleseed.AssignSuperAdminResult, error) {
					return roleseed.AssignSuperAdminResult{}, assignErr
				},
			},
			credentials: fakeRBACCredentialStore{
				getByUsername: func(context.Context, string) (*authdomain.UserCredential, error) {
					return &authdomain.UserCredential{UserID: userID}, nil
				},
			},
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

func TestNormalizeUsername(t *testing.T) {
	require.Equal(t, "admin", normalizeUsername(" ADMIN "))
	require.Equal(t, "", normalizeUsername(" "))
}

func TestChainCleanupRunsSecondBeforeFirstAndJoinsErrors(t *testing.T) {
	firstErr := errors.New("first failed")
	secondErr := errors.New("second failed")
	order := make([]string, 0, 2)

	cleanup := chainCleanup(
		func() error {
			order = append(order, "first")
			return firstErr
		},
		func() error {
			order = append(order, "second")
			return secondErr
		},
	)

	err := cleanup()

	require.ErrorIs(t, err, firstErr)
	require.ErrorIs(t, err, secondErr)
	require.Equal(t, []string{"second", "first"}, order)
}

func withRBACSeedDependencyFactory(t *testing.T, factory func(context.Context, string) (rbacSeedDependencies, func() error, error)) {
	t.Helper()
	original := newRBACSeedDependencies
	newRBACSeedDependencies = factory
	t.Cleanup(func() { newRBACSeedDependencies = original })
}

func captureStdout(t *testing.T, fn func() error) (string, error) {
	t.Helper()

	original := os.Stdout
	reader, writer, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = writer

	runErr := fn()
	closeErr := writer.Close()
	os.Stdout = original
	require.NoError(t, closeErr)

	output, err := io.ReadAll(reader)
	require.NoError(t, err)
	require.NoError(t, reader.Close())
	return string(output), runErr
}

type fakeRBACSeedService struct {
	seed   func(context.Context, roleseed.SeedOptions) (roleseed.SeedResult, error)
	assign func(context.Context, uuid.UUID) (roleseed.AssignSuperAdminResult, error)
}

func (s fakeRBACSeedService) Seed(ctx context.Context, opts roleseed.SeedOptions) (roleseed.SeedResult, error) {
	if s.seed == nil {
		return roleseed.SeedResult{}, errors.New("unexpected seed call")
	}
	return s.seed(ctx, opts)
}

func (s fakeRBACSeedService) AssignSuperAdmin(ctx context.Context, userID uuid.UUID) (roleseed.AssignSuperAdminResult, error) {
	if s.assign == nil {
		return roleseed.AssignSuperAdminResult{}, errors.New("unexpected assign super admin call")
	}
	return s.assign(ctx, userID)
}

type fakeCreateUserService struct {
	create func(context.Context, usercommand.CreateUserCommand) (*usercommand.CreateUserResult, error)
}

func (s fakeCreateUserService) CreateUser(ctx context.Context, cmd usercommand.CreateUserCommand) (*usercommand.CreateUserResult, error) {
	if s.create == nil {
		return nil, errors.New("unexpected create user call")
	}
	return s.create(ctx, cmd)
}

type fakeRBACCredentialStore struct {
	getByUsername     func(context.Context, string) (*authdomain.UserCredential, error)
	updateCredentials func(context.Context, authdomain.UpdateCredentialsInput) (int64, error)
}

func (s fakeRBACCredentialStore) GetByUsername(ctx context.Context, username string) (*authdomain.UserCredential, error) {
	if s.getByUsername == nil {
		return nil, errors.New("unexpected get by username call")
	}
	return s.getByUsername(ctx, username)
}

func (s fakeRBACCredentialStore) UpdateCredentials(ctx context.Context, input authdomain.UpdateCredentialsInput) (int64, error) {
	if s.updateCredentials == nil {
		return 0, errors.New("unexpected update credentials call")
	}
	return s.updateCredentials(ctx, input)
}

type fakePasswordHasher struct {
	hash func(context.Context, string) (string, error)
}

func (s fakePasswordHasher) HashContext(ctx context.Context, plain string) (string, error) {
	if s.hash == nil {
		return "", errors.New("unexpected password hash call")
	}
	return s.hash(ctx, plain)
}
