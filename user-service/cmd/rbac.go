package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/google/uuid"

	"github.com/aegiscore/common/runtime/config"
	"github.com/aegiscore/common/runtime/datastore"
	"github.com/aegiscore/common/runtime/logger"
	"github.com/aegiscore/common/runtime/resources"
	"github.com/aegiscore/common/security/password"
	"github.com/aegiscore/user-service/ent"
	authdomain "github.com/aegiscore/user-service/internal/features/auth/domain"
	authpostgres "github.com/aegiscore/user-service/internal/features/auth/infrastructure/postgres"
	permissionpostgres "github.com/aegiscore/user-service/internal/features/permission/infrastructure/postgres"
	roleseed "github.com/aegiscore/user-service/internal/features/role/application/seed"
	rolepostgres "github.com/aegiscore/user-service/internal/features/role/infrastructure/postgres"
	usercommand "github.com/aegiscore/user-service/internal/features/user/application/command"
	userpostgres "github.com/aegiscore/user-service/internal/features/user/infrastructure/postgres"
	"github.com/aegiscore/user-service/internal/shared/identity"
)

const rbacCommandTimeout = 30 * time.Second

const (
	defaultCreateSuperAdminUsername    = "admin"
	defaultCreateSuperAdminNickname    = "Admin"
	defaultCreateSuperAdminPasswordEnv = "ADMIN_PASSWORD"
)

type rbacSeedOptions struct {
	reactivateSystem   bool
	syncSystemBindings bool
}

type rbacCreateSuperAdminOptions struct {
	username      string
	nickname      string
	password      string
	passwordEnv   string
	resetPassword bool
}

type rbacCreateSuperAdminResult struct {
	userID          uuid.UUID
	created         bool
	passwordUpdated bool
	roleAdded       bool
}

type rbacSeedService interface {
	Seed(ctx context.Context, opts roleseed.SeedOptions) (roleseed.SeedResult, error)
	AssignSuperAdmin(ctx context.Context, userID uuid.UUID) (roleseed.AssignSuperAdminResult, error)
}

type rbacCredentialStore interface {
	GetByUsername(ctx context.Context, username string) (*authdomain.UserCredential, error)
	UpdateCredentials(ctx context.Context, input authdomain.UpdateCredentialsInput) (int64, error)
}

type rbacPasswordHasher interface {
	HashContext(ctx context.Context, plain string) (string, error)
}

type rbacSeedDependencies struct {
	service         rbacSeedService
	users           usercommand.CreateUserService
	credentials     rbacCredentialStore
	passwordService rbacPasswordHasher
}

type rbacSeedDependencyFactory func(context.Context, string) (rbacSeedDependencies, func() error, error)

func runRBACSeedCommand(ctx context.Context, configPath string, opts rbacSeedOptions, newDependencies rbacSeedDependencyFactory) (err error) {
	deps, cleanup, err := newDependencies(ctx, configPath)
	if err != nil {
		return err
	}
	defer func() {
		err = errors.Join(err, cleanup())
	}()

	result, err := deps.service.Seed(ctx, roleseed.SeedOptions{ReactivateSystem: opts.reactivateSystem, SyncSystemBindings: opts.syncSystemBindings})
	if err != nil {
		return err
	}
	fmt.Printf("RBAC seed complete: roles inserted=%d updated=%d permissions inserted=%d updated=%d bindings added=%d removed=%d\n", result.RolesInserted, result.RolesUpdated, result.PermissionsInserted, result.PermissionsUpdated, result.RolePermissionBindingsAdd, result.RolePermissionBindingsDel)
	return nil
}

func runAssignSuperAdminCommand(ctx context.Context, configPath string, userID uuid.UUID, newDependencies rbacSeedDependencyFactory) (err error) {
	deps, cleanup, err := newDependencies(ctx, configPath)
	if err != nil {
		return err
	}
	defer func() {
		err = errors.Join(err, cleanup())
	}()

	result, err := deps.service.AssignSuperAdmin(ctx, userID)
	if err != nil {
		return err
	}
	if result.Added {
		fmt.Printf("Super admin role assigned to user %s\n", userID.String())
	} else {
		fmt.Printf("Super admin role already assigned to user %s\n", userID.String())
	}
	return nil
}

func runCreateSuperAdminCommand(ctx context.Context, configPath string, opts rbacCreateSuperAdminOptions, newDependencies rbacSeedDependencyFactory) (err error) {
	deps, cleanup, err := newDependencies(ctx, configPath)
	if err != nil {
		return err
	}
	defer func() {
		err = errors.Join(err, cleanup())
	}()

	result, err := createSuperAdmin(ctx, deps, opts)
	if err != nil {
		return err
	}
	fmt.Printf("Super admin create complete: username=%s user_id=%s created=%t password_updated=%t super_admin_role_added=%t\n", normalizeUsername(opts.username), result.userID.String(), result.created, result.passwordUpdated, result.roleAdded)
	return nil
}

func createSuperAdmin(ctx context.Context, deps rbacSeedDependencies, opts rbacCreateSuperAdminOptions) (rbacCreateSuperAdminResult, error) {
	normalized, err := normalizeCreateSuperAdminOptions(opts)
	if err != nil {
		return rbacCreateSuperAdminResult{}, err
	}

	credential, err := deps.credentials.GetByUsername(ctx, normalized.username)
	if err != nil && !errors.Is(err, identity.ErrUserNotFound) {
		return rbacCreateSuperAdminResult{}, err
	}

	result := rbacCreateSuperAdminResult{}
	if errors.Is(err, identity.ErrUserNotFound) {
		status := identity.UserStatusNormal
		created, err := deps.users.CreateUser(ctx, usercommand.CreateUserCommand{Nickname: normalized.nickname, Username: normalized.username, Password: normalized.password, Status: &status})
		if err != nil {
			return rbacCreateSuperAdminResult{}, err
		}
		result.userID = created.User.UserID
		result.created = true
	} else {
		result.userID = credential.UserID
		if !normalized.resetPassword {
			assigned, err := deps.service.AssignSuperAdmin(ctx, result.userID)
			if err != nil {
				return rbacCreateSuperAdminResult{}, err
			}
			result.roleAdded = assigned.Added
			return result, nil
		}
		passwordHash, err := deps.passwordService.HashContext(ctx, normalized.password)
		if err != nil {
			return rbacCreateSuperAdminResult{}, fmt.Errorf("hash create super admin password: %w", err)
		}
		if _, err := deps.credentials.UpdateCredentials(ctx, authdomain.UpdateCredentialsInput{UserID: credential.UserID, PasswordHash: passwordHash, Status: identity.UserStatusNormal}); err != nil {
			return rbacCreateSuperAdminResult{}, err
		}
		result.passwordUpdated = true
	}

	assigned, err := deps.service.AssignSuperAdmin(ctx, result.userID)
	if err != nil {
		return rbacCreateSuperAdminResult{}, err
	}
	result.roleAdded = assigned.Added
	return result, nil
}

func defaultRBACSeedDependencies(parent context.Context, configPath string) (rbacSeedDependencies, func() error, error) {
	ctx, cancel := context.WithTimeout(parent, rbacCommandTimeout)
	cleanup := func() error {
		cancel()
		return nil
	}
	fail := func(err error) (rbacSeedDependencies, func() error, error) {
		cleanupErr := cleanup()
		return rbacSeedDependencies{}, func() error { return nil }, errors.Join(err, cleanupErr)
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		return fail(err)
	}
	passwordService, err := password.NewService(password.Options{
		Concurrency: cfg.Auth.PasswordKDF.Argon2Concurrency,
		QueueSize:   cfg.Auth.PasswordKDF.Argon2QueueSize,
	})
	if err != nil {
		return fail(err)
	}
	log, err := logger.New(cfg)
	if err != nil {
		return fail(err)
	}
	cleanup = chainCleanup(cleanup, func() error {
		_ = log.Sync()
		return nil
	})

	dbCfg, ok := cfg.PostgresDatabaseConfig(resources.NameUserDB)
	if !ok {
		return fail(fmt.Errorf("postgres.%s config not found", resources.NameUserDB))
	}
	db, err := datastore.OpenPostgres(resources.NameUserDB, dbCfg)
	if err != nil {
		return fail(err)
	}
	cleanup = chainCleanup(cleanup, func() error {
		_ = db.Close()
		return nil
	})
	if err := db.PingContext(ctx); err != nil {
		return fail(fmt.Errorf("ping postgres %s: %w", resources.NameUserDB, err))
	}

	client := newRBACEntClient(db)
	cleanup = chainCleanup(cleanup, func() error {
		_ = client.Close()
		return nil
	})
	permissionStore := permissionpostgres.NewPermissionStore(permissionpostgres.PermissionStoreParams{Client: client})
	roleStore := rolepostgres.NewRoleStore(rolepostgres.RoleStoreParams{Client: client})
	rolePermissionStore := rolepostgres.NewRolePermissionStore(rolepostgres.RolePermissionStoreParams{Client: client})
	userRoleStore := rolepostgres.NewUserRoleStore(rolepostgres.UserRoleStoreParams{Client: client})
	userStore := userpostgres.NewUserStore(userpostgres.UserStoreParams{Client: client})
	credentialStore := authpostgres.NewCredentialStore(authpostgres.CredentialStoreParams{Client: client})
	service := roleseed.NewService(roleStore, permissionStore, rolePermissionStore, userRoleStore)
	userCreator := usercommand.NewCreateUserService(userStore, passwordService)

	return rbacSeedDependencies{service: service, users: userCreator, credentials: credentialStore, passwordService: passwordService}, cleanup, nil
}

func newRBACEntClient(db *sql.DB) *ent.Client {
	driver := entsql.OpenDB(dialect.Postgres, db)
	return ent.NewClient(ent.Driver(driver))
}

func chainCleanup(first func() error, second func() error) func() error {
	return func() error {
		return errors.Join(second(), first())
	}
}

func normalizeCreateSuperAdminOptions(opts rbacCreateSuperAdminOptions) (rbacCreateSuperAdminOptions, error) {
	passwordEnv := strings.TrimSpace(opts.passwordEnv)
	if passwordEnv == "" {
		passwordEnv = defaultCreateSuperAdminPasswordEnv
	}
	adminPassword, ok := os.LookupEnv(passwordEnv)
	if !ok {
		return rbacCreateSuperAdminOptions{}, fmt.Errorf("%s environment variable is required", passwordEnv)
	}
	normalized := rbacCreateSuperAdminOptions{
		username:      normalizeUsername(opts.username),
		nickname:      strings.TrimSpace(opts.nickname),
		password:      strings.TrimSpace(adminPassword),
		passwordEnv:   passwordEnv,
		resetPassword: opts.resetPassword,
	}
	if normalized.username == "" {
		return rbacCreateSuperAdminOptions{}, fmt.Errorf("admin username is required")
	}
	if normalized.nickname == "" {
		normalized.nickname = normalized.username
	}
	if strings.TrimSpace(adminPassword) == "" {
		return rbacCreateSuperAdminOptions{}, fmt.Errorf("admin password is required")
	}
	return normalized, nil
}

func normalizeUsername(username string) string {
	return strings.ToLower(strings.TrimSpace(username))
}
