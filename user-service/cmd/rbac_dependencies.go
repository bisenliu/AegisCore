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
	serviceconfig "github.com/aegiscore/user-service/internal/config"
	permissionpostgres "github.com/aegiscore/user-service/internal/features/permission/infrastructure/postgres"
	rolebootstrap "github.com/aegiscore/user-service/internal/features/role/application/bootstrap"
	roleseed "github.com/aegiscore/user-service/internal/features/role/application/seed"
	rolepostgres "github.com/aegiscore/user-service/internal/features/role/infrastructure/postgres"
	"github.com/aegiscore/user-service/internal/persistence/ent"
	"github.com/aegiscore/user-service/internal/resources"
)

// rbacCommandTimeout 是执行 RBAC 离线初始化命令的全局超时时间。
const rbacCommandTimeout = 30 * time.Second

// defaultRBACSeedDependencies 初始化权限写入命令所需的数据库连接和日志记录工具。
func defaultRBACSeedDependencies(parent context.Context) (rbacSeedDependencies, func() error, error) {
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

	loaded, err := serviceconfig.Load(ctx)
	if err != nil {
		return fail(err)
	}
	cfg := loaded.Config
	passwordService, err := password.NewService()
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
	permissionStore := permissionpostgres.NewPermissionStore(client)
	roleStore := rolepostgres.NewRoleStore(client)
	rolePermissionStore := rolepostgres.NewRolePermissionStore(client)
	service := roleseed.NewService(roleStore, permissionStore, rolePermissionStore)
	bootstrapStore := rolepostgres.NewBootstrapStore(db)
	bootstrapService := rolebootstrap.NewService(bootstrapStore, passwordService)

	return rbacSeedDependencies{service: service, bootstrap: bootstrapService, cfg: cfg, log: log}, cleanup, nil
}

// rbacPostgresConfig 从系统全局配置中提取数据库连接参数，确保数据库配置已填写。
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

// newRBACEntClient 使用已有的数据库连接创建用于操作权限和用户数据的数据访问客户端。
func newRBACEntClient(db *sql.DB) *ent.Client {
	driver := entsql.OpenDB(dialect.Postgres, db)
	return ent.NewClient(ent.Driver(driver))
}

// chainCleanup 将两个资源释放操作按先后顺序串联起来，确保在命令执行完毕后所有打开的连接都能完整关闭。
func chainCleanup(first func() error, second func() error) func() error {
	return func() error {
		// 遵循后打开的资源先关闭的原则（例如先关闭依赖数据库的客户端，再关闭数据库底层的物理连接），确保依赖关系正常解除，并收集所有关闭时发生的错误。
		return errors.Join(second(), first())
	}
}
