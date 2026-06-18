package postgres

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/aegiscore/common/testing/containers"
	"github.com/aegiscore/user-service/ent"
	permissionapplication "github.com/aegiscore/user-service/internal/features/permission/application"
	permissiondomain "github.com/aegiscore/user-service/internal/features/permission/domain"
	permissionpostgres "github.com/aegiscore/user-service/internal/features/permission/infrastructure/postgres"
	roleapplication "github.com/aegiscore/user-service/internal/features/role/application"
)

func TestRolePermissionStorePostgresAddRechecksPermissionActiveState(t *testing.T) {
	client := newRolePermissionPostgresTestClient(t)
	ctx := context.Background()
	roleID := uuid.MustParse("018f0000-0000-7000-8000-000000000401")
	permissionID := uuid.MustParse("018f0000-0000-7000-8000-000000000402")
	roleStore := NewRoleStore(RoleStoreParams{Client: client})
	permissionStore := permissionpostgres.NewPermissionStore(permissionpostgres.PermissionStoreParams{Client: client})
	bindingStore := NewRolePermissionStore(RolePermissionStoreParams{Client: client})

	if _, err := roleStore.Create(ctx, roleapplication.CreateRoleInput{RoleID: roleID, Name: "Operator", Active: true}); err != nil {
		t.Fatalf("Create role: %v", err)
	}
	createdPermission, err := permissionStore.Create(ctx, permissionapplication.CreatePermissionInput{PermissionID: permissionID, Name: "Create User", Module: "user", HTTPMethod: "POST", PathTemplate: "/api/v1/users", Active: true})
	if err != nil {
		t.Fatalf("Create permission: %v", err)
	}
	if _, err := permissionStore.SetActive(ctx, permissionID, false); err != nil {
		t.Fatalf("SetActive: %v", err)
	}

	err = bindingStore.Add(ctx, roleID, roleapplication.PermissionReference{ID: createdPermission.ID, PermissionID: createdPermission.PermissionID})
	if !errors.Is(err, permissiondomain.ErrPermissionNotFound) {
		t.Fatalf("Add err = %v, want %v", err, permissiondomain.ErrPermissionNotFound)
	}
	items, err := bindingStore.ListByRoleID(ctx, roleID)
	if err != nil {
		t.Fatalf("ListByRoleID: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("bindings = %d, want 0", len(items))
	}
}

func TestRolePermissionStorePostgresReplaceRechecksPermissionActiveState(t *testing.T) {
	client := newRolePermissionPostgresTestClient(t)
	ctx := context.Background()
	roleID := uuid.MustParse("018f0000-0000-7000-8000-000000000501")
	activePermissionID := uuid.MustParse("018f0000-0000-7000-8000-000000000502")
	inactivePermissionID := uuid.MustParse("018f0000-0000-7000-8000-000000000503")
	roleStore := NewRoleStore(RoleStoreParams{Client: client})
	permissionStore := permissionpostgres.NewPermissionStore(permissionpostgres.PermissionStoreParams{Client: client})
	bindingStore := NewRolePermissionStore(RolePermissionStoreParams{Client: client})

	if _, err := roleStore.Create(ctx, roleapplication.CreateRoleInput{RoleID: roleID, Name: "Auditor", Active: true}); err != nil {
		t.Fatalf("Create role: %v", err)
	}
	activePermission, err := permissionStore.Create(ctx, permissionapplication.CreatePermissionInput{PermissionID: activePermissionID, Name: "List Users", Module: "user", HTTPMethod: "GET", PathTemplate: "/api/v1/users", Active: true})
	if err != nil {
		t.Fatalf("Create active permission: %v", err)
	}
	inactivePermission, err := permissionStore.Create(ctx, permissionapplication.CreatePermissionInput{PermissionID: inactivePermissionID, Name: "Delete User", Module: "user", HTTPMethod: "DELETE", PathTemplate: "/api/v1/users/:id", Active: true})
	if err != nil {
		t.Fatalf("Create inactive permission: %v", err)
	}
	if _, err := bindingStore.Replace(ctx, roleID, []roleapplication.PermissionReference{{PermissionID: activePermissionID}}); err != nil {
		t.Fatalf("Replace initial: %v", err)
	}
	if _, err := permissionStore.SetActive(ctx, inactivePermissionID, false); err != nil {
		t.Fatalf("SetActive: %v", err)
	}

	_, err = bindingStore.Replace(ctx, roleID, []roleapplication.PermissionReference{
		{ID: activePermission.ID, PermissionID: activePermission.PermissionID},
		{ID: inactivePermission.ID, PermissionID: inactivePermission.PermissionID},
	})
	if !errors.Is(err, permissiondomain.ErrPermissionNotFound) {
		t.Fatalf("Replace err = %v, want %v", err, permissiondomain.ErrPermissionNotFound)
	}
	items, err := bindingStore.ListByRoleID(ctx, roleID)
	if err != nil {
		t.Fatalf("ListByRoleID: %v", err)
	}
	if len(items) != 1 || items[0].PermissionID != activePermissionID {
		t.Fatalf("bindings = %#v, want only %s", items, activePermissionID.String())
	}
}

func newRolePermissionPostgresTestClient(t *testing.T) *ent.Client {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	t.Cleanup(cancel)

	postgres := containers.StartPostgres(ctx, t, containers.PostgresOptions{})
	db, err := sql.Open("pgx", postgres.DSN)
	if err != nil {
		t.Fatalf("open PostgreSQL database/sql client: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	client := ent.NewClient(ent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	if err := client.Schema.Create(ctx); err != nil {
		t.Fatalf("create PostgreSQL test schema: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })
	return client
}
