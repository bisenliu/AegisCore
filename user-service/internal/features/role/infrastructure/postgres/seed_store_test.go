package postgres

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"

	"github.com/aegiscore/user-service/ent"
	"github.com/aegiscore/user-service/ent/enttest"
	permissionapplication "github.com/aegiscore/user-service/internal/features/permission/application"
	permissionpostgres "github.com/aegiscore/user-service/internal/features/permission/infrastructure/postgres"
	roleapplication "github.com/aegiscore/user-service/internal/features/role/application"
	roledomain "github.com/aegiscore/user-service/internal/features/role/domain"
)

func TestRoleStoreUpsertSystemRole(t *testing.T) {
	client := newRoleSeedTestClient(t)
	store := NewRoleStore(RoleStoreParams{Client: client})
	ctx := context.Background()
	roleID := uuid.MustParse("018f0000-0000-7000-8000-000000000101")
	input := roleapplication.SeedRoleInput{RoleID: roleID, Name: "Super Admin", Description: "all", Active: true, IsSystem: true}

	created, inserted, err := store.UpsertSystemRole(ctx, input)
	if err != nil {
		t.Fatalf("UpsertSystemRole create: %v", err)
	}
	if !inserted || created.RoleID != roleID || !created.Active || !created.IsSystem {
		t.Fatalf("created=%#v inserted=%v", created, inserted)
	}
	if _, err := store.SetActive(ctx, roleID, false); err != nil {
		t.Fatalf("SetActive: %v", err)
	}
	input.Name = "Super Admin Updated"
	updated, inserted, err := store.UpsertSystemRole(ctx, input)
	if err != nil {
		t.Fatalf("UpsertSystemRole update: %v", err)
	}
	if inserted || updated.Active || updated.Name != "Super Admin Updated" {
		t.Fatalf("updated=%#v inserted=%v", updated, inserted)
	}
	input.ReactivateSystem = true
	updated, inserted, err = store.UpsertSystemRole(ctx, input)
	if err != nil {
		t.Fatalf("UpsertSystemRole reactivate: %v", err)
	}
	if inserted || !updated.Active {
		t.Fatalf("reactivated=%#v inserted=%v", updated, inserted)
	}
}

func TestRolePermissionStoreSeedEnsureAndSync(t *testing.T) {
	client := newRoleSeedTestClient(t)
	ctx := context.Background()
	roleStore := NewRoleStore(RoleStoreParams{Client: client})
	permissionStore := permissionpostgres.NewPermissionStore(permissionpostgres.PermissionStoreParams{Client: client})
	bindingStore := NewRolePermissionStore(RolePermissionStoreParams{Client: client})
	roleID := uuid.MustParse("018f0000-0000-7000-8000-000000000201")
	permissionID := uuid.MustParse("018f0000-0000-7000-8000-000000000202")
	extraPermissionID := uuid.MustParse("018f0000-0000-7000-8000-000000000203")

	if _, _, err := roleStore.UpsertSystemRole(ctx, roleapplication.SeedRoleInput{RoleID: roleID, Name: "System", Active: true, IsSystem: true}); err != nil {
		t.Fatalf("UpsertSystemRole: %v", err)
	}
	if _, err := permissionStore.Create(ctx, permissionapplication.CreatePermissionInput{PermissionID: permissionID, Name: "List", Module: "user", HTTPMethod: "GET", PathTemplate: "/api/v1/users", Active: true, IsSystem: true}); err != nil {
		t.Fatalf("Create permission: %v", err)
	}
	if _, err := permissionStore.Create(ctx, permissionapplication.CreatePermissionInput{PermissionID: extraPermissionID, Name: "Create", Module: "user", HTTPMethod: "POST", PathTemplate: "/api/v1/users", Active: true, IsSystem: true}); err != nil {
		t.Fatalf("Create extra permission: %v", err)
	}

	added, err := bindingStore.EnsureSystemBindings(ctx, roleID, []uuid.UUID{permissionID, extraPermissionID})
	if err != nil {
		t.Fatalf("EnsureSystemBindings: %v", err)
	}
	if added != 2 {
		t.Fatalf("added = %d, want 2", added)
	}
	added, err = bindingStore.EnsureSystemBindings(ctx, roleID, []uuid.UUID{permissionID, extraPermissionID, permissionID})
	if err != nil {
		t.Fatalf("EnsureSystemBindings repeat: %v", err)
	}
	if added != 0 {
		t.Fatalf("repeat added = %d, want 0", added)
	}
	missingPermissionID := uuid.MustParse("018f0000-0000-7000-8000-000000000204")
	if _, err = bindingStore.EnsureSystemBindings(ctx, roleID, []uuid.UUID{missingPermissionID}); !errors.Is(err, roledomain.ErrRolePermissionNotFound) {
		t.Fatalf("EnsureSystemBindings missing err = %v, want %v", err, roledomain.ErrRolePermissionNotFound)
	}
	added, removed, err := bindingStore.SyncSystemBindings(ctx, roleID, []uuid.UUID{permissionID})
	if err != nil {
		t.Fatalf("SyncSystemBindings: %v", err)
	}
	if added != 0 || removed != 1 {
		t.Fatalf("added=%d removed=%d", added, removed)
	}
}

func TestUserRoleStoreAssignRoleIdempotent(t *testing.T) {
	client := newRoleSeedTestClient(t)
	ctx := context.Background()
	roleStore := NewRoleStore(RoleStoreParams{Client: client})
	userRoleStore := NewUserRoleStore(UserRoleStoreParams{Client: client})
	userID := uuid.MustParse("018f0000-0000-7000-8000-000000000301")
	roleID := uuid.MustParse("018f0000-0000-7000-8000-000000000302")
	if _, err := client.User.Create().SetUserID(userID).SetNickname("Admin").SetUsername("admin@example.com").SetPasswordHash("hash").Save(ctx); err != nil {
		t.Fatalf("Create user: %v", err)
	}
	if _, _, err := roleStore.UpsertSystemRole(ctx, roleapplication.SeedRoleInput{RoleID: roleID, Name: "System", Active: true, IsSystem: true}); err != nil {
		t.Fatalf("UpsertSystemRole: %v", err)
	}

	added, err := userRoleStore.AssignRole(ctx, userID, roleID)
	if err != nil {
		t.Fatalf("AssignRole: %v", err)
	}
	if !added {
		t.Fatal("first AssignRole added=false")
	}
	added, err = userRoleStore.AssignRole(ctx, userID, roleID)
	if err != nil {
		t.Fatalf("AssignRole repeat: %v", err)
	}
	if added {
		t.Fatal("repeat AssignRole added=true")
	}
}

func newRoleSeedTestClient(t *testing.T) *ent.Client {
	t.Helper()
	client := enttest.Open(t, "sqlite3", fmt.Sprintf("file:role_seed_test_%s?mode=memory&cache=shared&_fk=1", uuid.NewString()))
	t.Cleanup(func() { _ = client.Close() })
	return client
}
