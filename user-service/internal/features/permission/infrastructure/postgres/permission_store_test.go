package postgres

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"

	"github.com/aegiscore/user-service/ent/enttest"
	permissionapplication "github.com/aegiscore/user-service/internal/features/permission/application"
	permissiondomain "github.com/aegiscore/user-service/internal/features/permission/domain"
)

func TestPermissionStoreDomainErrors(t *testing.T) {
	store := newTestPermissionStore(t)
	ctx := context.Background()

	_, err := store.GetByPermissionID(ctx, uuid.MustParse("018f0000-0000-7000-8000-000000000099"))
	if !errors.Is(err, permissiondomain.ErrPermissionNotFound) {
		t.Fatalf("err = %v, want ErrPermissionNotFound", err)
	}

	input := permissionapplication.CreatePermissionInput{PermissionID: uuid.MustParse("018f0000-0000-7000-8000-000000000001"), Name: "List Users", Module: "user", HTTPMethod: "GET", PathTemplate: "/api/v1/users", Active: true, IsSystem: true}
	if _, err := store.Create(ctx, input); err != nil {
		t.Fatalf("Create: %v", err)
	}
	input.PermissionID = uuid.MustParse("018f0000-0000-7000-8000-000000000002")
	_, err = store.Create(ctx, input)
	if !errors.Is(err, permissiondomain.ErrPermissionAlreadyExists) {
		t.Fatalf("err = %v, want ErrPermissionAlreadyExists", err)
	}
}

func TestPermissionStoreListAndSetActive(t *testing.T) {
	store := newTestPermissionStore(t)
	ctx := context.Background()
	permissionID := uuid.MustParse("018f0000-0000-7000-8000-000000000011")
	created, err := store.Create(ctx, permissionapplication.CreatePermissionInput{PermissionID: permissionID, Name: "List Users", Module: "user", HTTPMethod: "GET", PathTemplate: "/api/v1/users", Active: true, IsSystem: true})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if !created.Active || !created.IsSystem {
		t.Fatalf("created = %#v", created)
	}

	disabled, err := store.SetActive(ctx, permissionID, false)
	if err != nil {
		t.Fatalf("SetActive: %v", err)
	}
	if disabled.Active {
		t.Fatalf("disabled.Active = true")
	}

	system := true
	items, hasNext, err := store.List(ctx, permissionapplication.ListPermissionsInput{Limit: 10, IsSystem: &system})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if hasNext || len(items) != 1 || items[0].PermissionID != permissionID {
		t.Fatalf("items=%#v hasNext=%v", items, hasNext)
	}
}

func newTestPermissionStore(t *testing.T) *PermissionStore {
	t.Helper()
	client := enttest.Open(t, "sqlite3", fmt.Sprintf("file:permission_store_test_%s?mode=memory&cache=shared&_fk=1", uuid.NewString()))
	t.Cleanup(func() { _ = client.Close() })
	return NewPermissionStore(PermissionStoreParams{Client: client})
}
