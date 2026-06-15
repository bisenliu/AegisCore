package command

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	permissionapplication "github.com/aegiscore/user-service/internal/features/permission/application"
	permissiondomain "github.com/aegiscore/user-service/internal/features/permission/domain"
)

func TestPermissionCommandServiceCreateAndProtectSystemPermission(t *testing.T) {
	store := &stubPermissionStore{}
	service := NewPermissionCommandService(store)

	created, err := service.CreatePermission(context.Background(), CreatePermissionCommand{Name: "List Users", Module: "user", HTTPMethod: "get", PathTemplate: "/api/v1/users", IsSystem: true})
	if err != nil {
		t.Fatalf("CreatePermission: %v", err)
	}
	if created.Permission.HTTPMethod != "GET" || !created.Permission.Active || !created.Permission.IsSystem {
		t.Fatalf("created = %#v", created.Permission)
	}

	_, err = service.UpdatePermission(context.Background(), UpdatePermissionCommand{PermissionID: created.Permission.PermissionID, Name: "List Users", Module: "user", HTTPMethod: "POST", PathTemplate: "/api/v1/users", Active: true})
	if !errors.Is(err, permissiondomain.ErrSystemPermissionProtected) {
		t.Fatalf("err = %v, want ErrSystemPermissionProtected", err)
	}
}

func TestPermissionCommandServiceSetActive(t *testing.T) {
	store := &stubPermissionStore{permission: permissiondomain.Permission{PermissionID: uuid.MustParse("018f0000-0000-7000-8000-000000000001"), HTTPMethod: "GET", PathTemplate: "/api/v1/users", Active: true}}
	service := NewPermissionCommandService(store)

	result, err := service.DisablePermission(context.Background(), SetPermissionActiveCommand{PermissionID: store.permission.PermissionID})
	if err != nil {
		t.Fatalf("DisablePermission: %v", err)
	}
	if result.Permission.Active {
		t.Fatalf("permission remains active")
	}
}

func TestPermissionCommandServiceCreateMapsDuplicateAndShortCircuitsValidation(t *testing.T) {
	store := &stubPermissionStore{createErr: permissiondomain.ErrPermissionAlreadyExists}
	service := NewPermissionCommandService(store)

	_, err := service.CreatePermission(context.Background(), CreatePermissionCommand{Name: "List Users", Module: "user", HTTPMethod: "GET", PathTemplate: "/api/v1/users"})
	if !errors.Is(err, permissiondomain.ErrPermissionAlreadyExists) {
		t.Fatalf("err = %v, want ErrPermissionAlreadyExists", err)
	}
	if !store.createCalled {
		t.Fatalf("Create was not called for valid duplicate input")
	}

	store = &stubPermissionStore{}
	service = NewPermissionCommandService(store)
	_, err = service.CreatePermission(context.Background(), CreatePermissionCommand{Name: "", Module: "user", HTTPMethod: "GET", PathTemplate: "/api/v1/users"})
	if err == nil {
		t.Fatalf("err is nil for invalid input")
	}
	if store.createCalled {
		t.Fatalf("Create called after validation failure")
	}
}

func TestPermissionCommandServiceUpdateNonSystemNormalizesAndMapsDuplicate(t *testing.T) {
	permissionID := uuid.MustParse("018f0000-0000-7000-8000-000000000002")
	store := &stubPermissionStore{permission: permissiondomain.Permission{PermissionID: permissionID, HTTPMethod: "GET", PathTemplate: "/api/v1/users", Active: true}}
	service := NewPermissionCommandService(store)

	result, err := service.UpdatePermission(context.Background(), UpdatePermissionCommand{PermissionID: permissionID, Name: "  Create User  ", Description: "  Create users  ", Module: "  user  ", HTTPMethod: "post", PathTemplate: "/api/v1/users", Active: true})
	if err != nil {
		t.Fatalf("UpdatePermission: %v", err)
	}
	if result.Permission.Name != "Create User" || result.Permission.Description != "Create users" || result.Permission.Module != "user" || result.Permission.HTTPMethod != "POST" {
		t.Fatalf("updated permission = %#v", result.Permission)
	}
	if store.updateInput.HTTPMethod != "POST" || store.updateInput.PathTemplate != "/api/v1/users" {
		t.Fatalf("update input = %#v", store.updateInput)
	}

	store.updateErr = permissiondomain.ErrPermissionAlreadyExists
	_, err = service.UpdatePermission(context.Background(), UpdatePermissionCommand{PermissionID: permissionID, Name: "Create User", Module: "user", HTTPMethod: "POST", PathTemplate: "/api/v1/users", Active: true})
	if !errors.Is(err, permissiondomain.ErrPermissionAlreadyExists) {
		t.Fatalf("err = %v, want ErrPermissionAlreadyExists", err)
	}
}

func TestPermissionCommandServiceEnablePermission(t *testing.T) {
	permissionID := uuid.MustParse("018f0000-0000-7000-8000-000000000003")
	store := &stubPermissionStore{permission: permissiondomain.Permission{PermissionID: permissionID, HTTPMethod: "GET", PathTemplate: "/api/v1/users", Active: false}}
	service := NewPermissionCommandService(store)

	result, err := service.EnablePermission(context.Background(), SetPermissionActiveCommand{PermissionID: permissionID})
	if err != nil {
		t.Fatalf("EnablePermission: %v", err)
	}
	if !result.Permission.Active || !store.setActiveCalled || !store.setActiveValue {
		t.Fatalf("active = %v called = %v value = %v", result.Permission.Active, store.setActiveCalled, store.setActiveValue)
	}
}

type stubPermissionStore struct {
	permission      permissiondomain.Permission
	createCalled    bool
	createErr       error
	updateInput     permissionapplication.UpdatePermissionInput
	updateErr       error
	setActiveCalled bool
	setActiveValue  bool
}

func (s *stubPermissionStore) Create(_ context.Context, input permissionapplication.CreatePermissionInput) (*permissiondomain.Permission, error) {
	s.createCalled = true
	if s.createErr != nil {
		return nil, s.createErr
	}
	s.permission = permissiondomain.Permission{PermissionID: input.PermissionID, Name: input.Name, Description: input.Description, Module: input.Module, HTTPMethod: input.HTTPMethod, PathTemplate: input.PathTemplate, Active: input.Active, IsSystem: input.IsSystem}
	return &s.permission, nil
}

func (s *stubPermissionStore) GetByPermissionID(_ context.Context, permissionID uuid.UUID) (*permissiondomain.Permission, error) {
	if s.permission.PermissionID != permissionID {
		return nil, permissiondomain.ErrPermissionNotFound
	}
	return &s.permission, nil
}

func (s *stubPermissionStore) List(context.Context, permissionapplication.ListPermissionsInput) ([]permissiondomain.Permission, bool, error) {
	return []permissiondomain.Permission{s.permission}, false, nil
}

func (s *stubPermissionStore) ListAll(context.Context) ([]permissiondomain.Permission, error) {
	return []permissiondomain.Permission{s.permission}, nil
}

func (s *stubPermissionStore) ListEffectiveByUserID(context.Context, uuid.UUID) ([]permissiondomain.Permission, error) {
	return []permissiondomain.Permission{s.permission}, nil
}

func (s *stubPermissionStore) Update(_ context.Context, input permissionapplication.UpdatePermissionInput) (*permissiondomain.Permission, error) {
	s.updateInput = input
	if s.updateErr != nil {
		return nil, s.updateErr
	}
	s.permission.Name = input.Name
	s.permission.Description = input.Description
	s.permission.Module = input.Module
	s.permission.HTTPMethod = input.HTTPMethod
	s.permission.PathTemplate = input.PathTemplate
	s.permission.Active = input.Active
	return &s.permission, nil
}

func (s *stubPermissionStore) SetActive(_ context.Context, _ uuid.UUID, active bool) (*permissiondomain.Permission, error) {
	s.setActiveCalled = true
	s.setActiveValue = active
	s.permission.Active = active
	return &s.permission, nil
}
