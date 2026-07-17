package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"

	"github.com/aegiscore/common/runtime/datastore"
	"github.com/aegiscore/common/runtime/logger"
	commonresources "github.com/aegiscore/common/runtime/resources"
	"github.com/aegiscore/common/security/password"
	"github.com/aegiscore/user-service/ent"
	serviceconfig "github.com/aegiscore/user-service/internal/config"
	authpostgres "github.com/aegiscore/user-service/internal/features/auth/infrastructure/postgres"
	permissionpostgres "github.com/aegiscore/user-service/internal/features/permission/infrastructure/postgres"
	roleseed "github.com/aegiscore/user-service/internal/features/role/application/seed"
	rolepostgres "github.com/aegiscore/user-service/internal/features/role/infrastructure/postgres"
	usercommand "github.com/aegiscore/user-service/internal/features/user/application/command"
	userpostgres "github.com/aegiscore/user-service/internal/features/user/infrastructure/postgres"
	"github.com/aegiscore/user-service/internal/resources"
)

const rbacCommandTimeout = 30 * time.Second

func defaultRBACSeedDependencies(parent context.Context, configPath string) (rbacSeedDependencies, func() error, error) {
	// RBAC CLI 绕过 Fx composition root，只打开 PostgreSQL 并组装 seed 所需最小依赖；不能在这里启动 HTTP、Redis watcher 或服务生命周期。
	ctx, cancel := context.WithTimeout(parent, rbacCommandTimeout)
	cleanup := func() error {
		cancel()
		return nil
	}
	fail := func(err error) (rbacSeedDependencies, func() error, error) {
		cleanupErr := cleanup()
		return rbacSeedDependencies{}, func() error { return nil }, errors.Join(err, cleanupErr)
	}

	cfg, err := serviceconfig.NewConfig(serviceconfig.ConfigPath(configPath))
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
	runtimeCfg := cfg.RuntimeConfig()
	log, err := logger.New(&runtimeCfg)
	if err != nil {
		return fail(err)
	}
	cleanup = chainCleanup(cleanup, func() error {
		_ = log.Sync()
		return nil
	})

	dbCfg, err := rbacPostgresConfig(cfg)
	if err != nil {
		return fail(err)
	}
	db, err := datastore.OpenPostgres(resources.NamePrimaryDB, dbCfg)
	if err != nil {
		return fail(err)
	}
	cleanup = chainCleanup(cleanup, func() error {
		_ = datastore.ClosePostgres(resources.NamePrimaryDB, db)
		return nil
	})
	if err := datastore.PingPostgres(ctx, resources.NamePrimaryDB, db); err != nil {
		return fail(err)
	}

	client := newRBACEntClient(db)
	cleanup = chainCleanup(cleanup, func() error {
		_ = client.Close()
		return nil
	})
	permissionStore := permissionpostgres.NewPermissionStore(permissionpostgres.PermissionStoreParams{Client: client})
	roleStore := rolepostgres.NewRoleStore(client)
	rolePermissionStore := rolepostgres.NewRolePermissionStore(client)
	userRoleStore := rolepostgres.NewUserRoleStore(client)
	userStore := userpostgres.NewUserStore(client)
	credentialStore := authpostgres.NewCredentialStore(authpostgres.CredentialStoreParams{Client: client})
	service := roleseed.NewService(roleStore, permissionStore, rolePermissionStore, userRoleStore)
	userCreator := usercommand.NewCreateUserService(userStore, passwordService)

	return rbacSeedDependencies{service: service, users: userCreator, credentials: credentialStore, passwordService: passwordService, log: log}, cleanup, nil
}

func rbacPostgresConfig(cfg *serviceconfig.Config) (commonresources.PostgresConfig, error) {
	if cfg == nil {
		return commonresources.PostgresConfig{}, errors.New("user-service config is required")
	}
	dbCfg, ok := cfg.Resources.Postgres[resources.NamePrimaryDB]
	if !ok {
		return commonresources.PostgresConfig{}, fmt.Errorf("resources.postgres.%s config not found", resources.NamePrimaryDB)
	}
	return dbCfg, nil
}

func newRBACEntClient(db *sql.DB) *ent.Client {
	driver := entsql.OpenDB(dialect.Postgres, db)
	return ent.NewClient(ent.Driver(driver))
}

func chainCleanup(first func() error, second func() error) func() error {
	return func() error {
		// 后注册资源先关闭，保持与 defer 栈一致的 LIFO 语义，并聚合所有关闭错误。
		return errors.Join(second(), first())
	}
}
