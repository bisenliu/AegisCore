package main

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/google/uuid"

	"github.com/aegiscore/common/runtime/config"
	"github.com/aegiscore/common/runtime/datastore"
	"github.com/aegiscore/common/runtime/logger"
	"github.com/aegiscore/common/runtime/resources"
	"github.com/aegiscore/user-service/ent"
	permissionpostgres "github.com/aegiscore/user-service/internal/features/permission/infrastructure/postgres"
	roleseed "github.com/aegiscore/user-service/internal/features/role/application/seed"
	rolepostgres "github.com/aegiscore/user-service/internal/features/role/infrastructure/postgres"
)

const rbacCommandTimeout = 30 * time.Second

type rbacSeedOptions struct {
	reactivateSystem   bool
	syncSystemBindings bool
}

func runRBACSeedCommand(ctx context.Context, configPath string, opts rbacSeedOptions) error {
	deps, cleanup, err := newRBACSeedDependencies(ctx, configPath)
	if err != nil {
		return err
	}
	defer cleanup()

	result, err := deps.service.Seed(ctx, roleseed.SeedOptions{ReactivateSystem: opts.reactivateSystem, SyncSystemBindings: opts.syncSystemBindings})
	if err != nil {
		return err
	}
	fmt.Printf("RBAC seed complete: roles inserted=%d updated=%d permissions inserted=%d updated=%d bindings added=%d removed=%d\n", result.RolesInserted, result.RolesUpdated, result.PermissionsInserted, result.PermissionsUpdated, result.RolePermissionBindingsAdd, result.RolePermissionBindingsDel)
	return nil
}

func runAssignSuperAdminCommand(ctx context.Context, configPath string, userID uuid.UUID) error {
	deps, cleanup, err := newRBACSeedDependencies(ctx, configPath)
	if err != nil {
		return err
	}
	defer cleanup()

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

type rbacSeedDependencies struct {
	service *roleseed.Service
}

func newRBACSeedDependencies(parent context.Context, configPath string) (rbacSeedDependencies, func(), error) {
	ctx, cancel := context.WithTimeout(parent, rbacCommandTimeout)
	cleanup := func() { cancel() }

	cfg, err := config.Load(configPath)
	if err != nil {
		cleanup()
		return rbacSeedDependencies{}, func() {}, err
	}
	log, err := logger.New(cfg)
	if err != nil {
		cleanup()
		return rbacSeedDependencies{}, func() {}, err
	}
	cleanup = chainCleanup(cleanup, func() { _ = log.Sync() })

	dbCfg, ok := cfg.PostgresDatabaseConfig(resources.NameUserDB)
	if !ok {
		cleanup()
		return rbacSeedDependencies{}, func() {}, fmt.Errorf("postgres.%s config not found", resources.NameUserDB)
	}
	db, err := datastore.OpenPostgres(resources.NameUserDB, dbCfg)
	if err != nil {
		cleanup()
		return rbacSeedDependencies{}, func() {}, err
	}
	cleanup = chainCleanup(cleanup, func() { _ = db.Close() })
	if err := db.PingContext(ctx); err != nil {
		cleanup()
		return rbacSeedDependencies{}, func() {}, fmt.Errorf("ping postgres %s: %w", resources.NameUserDB, err)
	}

	client := newRBACEntClient(db)
	cleanup = chainCleanup(cleanup, func() { _ = client.Close() })
	permissionStore := permissionpostgres.NewPermissionStore(permissionpostgres.PermissionStoreParams{Client: client})
	roleStore := rolepostgres.NewRoleStore(rolepostgres.RoleStoreParams{Client: client})
	rolePermissionStore := rolepostgres.NewRolePermissionStore(rolepostgres.RolePermissionStoreParams{Client: client})
	userRoleStore := rolepostgres.NewUserRoleStore(rolepostgres.UserRoleStoreParams{Client: client})
	service := roleseed.NewService(roleStore, permissionStore, rolePermissionStore, userRoleStore)

	return rbacSeedDependencies{service: service}, cleanup, nil
}

func newRBACEntClient(db *sql.DB) *ent.Client {
	driver := entsql.OpenDB(dialect.Postgres, db)
	return ent.NewClient(ent.Driver(driver))
}

func chainCleanup(first func(), second func()) func() {
	return func() {
		second()
		first()
	}
}
