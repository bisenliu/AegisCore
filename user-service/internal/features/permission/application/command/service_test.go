package command

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	permissionapplication "github.com/aegiscore/user-service/internal/features/permission/application"
	permissiondomain "github.com/aegiscore/user-service/internal/features/permission/domain"
)

func TestPermissionCommandServiceCreateAndProtectSystemPermission(t *testing.T) {
	store := NewMockPermissionStore(gomock.NewController(t))
	notifier := NewMockPolicyChangeNotifier(gomock.NewController(t))
	store.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, input permissionapplication.CreatePermissionInput) (*permissiondomain.Permission, error) {
		return &permissiondomain.Permission{PermissionID: input.PermissionID, Name: input.Name, Description: input.Description, Module: input.Module, HTTPMethod: input.HTTPMethod, PathTemplate: input.PathTemplate, Active: input.Active, IsSystem: input.IsSystem}, nil
	})
	notifier.EXPECT().NotifyPolicyChanged(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, change permissionapplication.PolicyChange) error {
		if change.Reason != "permission_created" {
			t.Fatalf("reason = %q", change.Reason)
		}
		return nil
	})
	service := NewPermissionCommandService(PermissionCommandParams{Store: store, PolicyChanges: notifier})

	created, err := service.CreatePermission(context.Background(), CreatePermissionCommand{Name: "List Users", Module: "user", HTTPMethod: "get", PathTemplate: "/api/v1/users", IsSystem: true})
	if err != nil {
		t.Fatalf("CreatePermission: %v", err)
	}
	if created.Permission.HTTPMethod != "GET" || !created.Permission.Active || !created.Permission.IsSystem {
		t.Fatalf("created = %#v", created.Permission)
	}

	store = NewMockPermissionStore(gomock.NewController(t))
	service = NewPermissionCommandService(PermissionCommandParams{Store: store, PolicyChanges: NewMockPolicyChangeNotifier(gomock.NewController(t))})
	store.EXPECT().GetByPermissionID(gomock.Any(), created.Permission.PermissionID).Return(&created.Permission, nil)
	_, err = service.UpdatePermission(context.Background(), UpdatePermissionCommand{PermissionID: created.Permission.PermissionID, Name: "List Users", Module: "user", HTTPMethod: "POST", PathTemplate: "/api/v1/users", Active: true})
	if !errors.Is(err, permissiondomain.ErrSystemPermissionProtected) {
		t.Fatalf("err = %v, want ErrSystemPermissionProtected", err)
	}
}

func TestPermissionCommandServiceSetActive(t *testing.T) {
	permissionID := uuid.MustParse("018f0000-0000-7000-8000-000000000001")
	store := NewMockPermissionStore(gomock.NewController(t))
	notifier := NewMockPolicyChangeNotifier(gomock.NewController(t))
	store.EXPECT().SetActive(gomock.Any(), permissionID, false).Return(&permissiondomain.Permission{PermissionID: permissionID, HTTPMethod: "GET", PathTemplate: "/api/v1/users", Active: false}, nil)
	notifier.EXPECT().NotifyPolicyChanged(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, change permissionapplication.PolicyChange) error {
		if change.Reason != "permission_active_changed" {
			t.Fatalf("reason = %q", change.Reason)
		}
		return nil
	})
	service := NewPermissionCommandService(PermissionCommandParams{Store: store, PolicyChanges: notifier})

	result, err := service.DisablePermission(context.Background(), SetPermissionActiveCommand{PermissionID: permissionID})
	if err != nil {
		t.Fatalf("DisablePermission: %v", err)
	}
	if result.Permission.Active {
		t.Fatalf("permission remains active")
	}
}

func TestPermissionCommandServiceCreateMapsDuplicateAndShortCircuitsValidation(t *testing.T) {
	store := NewMockPermissionStore(gomock.NewController(t))
	store.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil, permissiondomain.ErrPermissionAlreadyExists)
	service := NewPermissionCommandService(PermissionCommandParams{Store: store, PolicyChanges: NewMockPolicyChangeNotifier(gomock.NewController(t))})

	_, err := service.CreatePermission(context.Background(), CreatePermissionCommand{Name: "List Users", Module: "user", HTTPMethod: "GET", PathTemplate: "/api/v1/users"})
	if !errors.Is(err, permissiondomain.ErrPermissionAlreadyExists) {
		t.Fatalf("err = %v, want ErrPermissionAlreadyExists", err)
	}

	store = NewMockPermissionStore(gomock.NewController(t))
	service = NewPermissionCommandService(PermissionCommandParams{Store: store, PolicyChanges: NewMockPolicyChangeNotifier(gomock.NewController(t))})
	_, err = service.CreatePermission(context.Background(), CreatePermissionCommand{Name: "", Module: "user", HTTPMethod: "GET", PathTemplate: "/api/v1/users"})
	if err == nil {
		t.Fatalf("err is nil for invalid input")
	}
}

func TestPermissionCommandServiceUpdateNonSystemNormalizesAndMapsDuplicate(t *testing.T) {
	permissionID := uuid.MustParse("018f0000-0000-7000-8000-000000000002")
	store := NewMockPermissionStore(gomock.NewController(t))
	notifier := NewMockPolicyChangeNotifier(gomock.NewController(t))
	store.EXPECT().GetByPermissionID(gomock.Any(), permissionID).Return(&permissiondomain.Permission{PermissionID: permissionID, HTTPMethod: "GET", PathTemplate: "/api/v1/users", Active: true}, nil)
	store.EXPECT().Update(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, input permissionapplication.UpdatePermissionInput) (*permissiondomain.Permission, error) {
		if input.HTTPMethod != "POST" || input.PathTemplate != "/api/v1/users" {
			t.Fatalf("update input = %#v", input)
		}
		return &permissiondomain.Permission{PermissionID: permissionID, Name: input.Name, Description: input.Description, Module: input.Module, HTTPMethod: input.HTTPMethod, PathTemplate: input.PathTemplate, Active: input.Active}, nil
	})
	notifier.EXPECT().NotifyPolicyChanged(gomock.Any(), gomock.Any()).Return(nil)
	service := NewPermissionCommandService(PermissionCommandParams{Store: store, PolicyChanges: notifier})

	result, err := service.UpdatePermission(context.Background(), UpdatePermissionCommand{PermissionID: permissionID, Name: "  Create User  ", Description: "  Create users  ", Module: "  user  ", HTTPMethod: "post", PathTemplate: "/api/v1/users", Active: true})
	if err != nil {
		t.Fatalf("UpdatePermission: %v", err)
	}
	if result.Permission.Name != "Create User" || result.Permission.Description != "Create users" || result.Permission.Module != "user" || result.Permission.HTTPMethod != "POST" {
		t.Fatalf("updated permission = %#v", result.Permission)
	}

	store = NewMockPermissionStore(gomock.NewController(t))
	service = NewPermissionCommandService(PermissionCommandParams{Store: store, PolicyChanges: NewMockPolicyChangeNotifier(gomock.NewController(t))})
	store.EXPECT().GetByPermissionID(gomock.Any(), permissionID).Return(&permissiondomain.Permission{PermissionID: permissionID, HTTPMethod: "GET", PathTemplate: "/api/v1/users", Active: true}, nil)
	store.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil, permissiondomain.ErrPermissionAlreadyExists)
	_, err = service.UpdatePermission(context.Background(), UpdatePermissionCommand{PermissionID: permissionID, Name: "Create User", Module: "user", HTTPMethod: "POST", PathTemplate: "/api/v1/users", Active: true})
	if !errors.Is(err, permissiondomain.ErrPermissionAlreadyExists) {
		t.Fatalf("err = %v, want ErrPermissionAlreadyExists", err)
	}
}

func TestPermissionCommandServiceEnablePermission(t *testing.T) {
	permissionID := uuid.MustParse("018f0000-0000-7000-8000-000000000003")
	store := NewMockPermissionStore(gomock.NewController(t))
	notifier := NewMockPolicyChangeNotifier(gomock.NewController(t))
	store.EXPECT().SetActive(gomock.Any(), permissionID, true).Return(&permissiondomain.Permission{PermissionID: permissionID, HTTPMethod: "GET", PathTemplate: "/api/v1/users", Active: true}, nil)
	notifier.EXPECT().NotifyPolicyChanged(gomock.Any(), gomock.Any()).Return(nil)
	service := NewPermissionCommandService(PermissionCommandParams{Store: store, PolicyChanges: notifier})

	result, err := service.EnablePermission(context.Background(), SetPermissionActiveCommand{PermissionID: permissionID})
	if err != nil {
		t.Fatalf("EnablePermission: %v", err)
	}
	if !result.Permission.Active {
		t.Fatalf("permission remains inactive")
	}
}

func TestPermissionCommandServiceSwallowsRefreshFailureAfterSuccessfulWrite(t *testing.T) {
	permissionID := uuid.MustParse("018f0000-0000-7000-8000-000000000004")
	refreshErr := errors.New("refresh failed")

	tests := []struct {
		name       string
		run        func(*testing.T, PermissionCommandService) *PermissionResult
		setupStore func(*MockPermissionStore)
		wantReason string
	}{
		{name: "create", run: func(t *testing.T, service PermissionCommandService) *PermissionResult {
			result, err := service.CreatePermission(context.Background(), CreatePermissionCommand{Name: "List Users", Module: "user", HTTPMethod: "GET", PathTemplate: "/api/v1/users"})
			if err != nil {
				t.Fatalf("CreatePermission: %v", err)
			}
			return result
		}, setupStore: func(store *MockPermissionStore) {
			store.EXPECT().Create(gomock.Any(), gomock.Any()).Return(&permissiondomain.Permission{PermissionID: permissionID, HTTPMethod: "GET", PathTemplate: "/api/v1/users", Active: true}, nil)
		}, wantReason: "permission_created"},
		{name: "update", run: func(t *testing.T, service PermissionCommandService) *PermissionResult {
			result, err := service.UpdatePermission(context.Background(), UpdatePermissionCommand{PermissionID: permissionID, Name: "List Users", Module: "user", HTTPMethod: "GET", PathTemplate: "/api/v1/users", Active: true})
			if err != nil {
				t.Fatalf("UpdatePermission: %v", err)
			}
			return result
		}, setupStore: func(store *MockPermissionStore) {
			store.EXPECT().GetByPermissionID(gomock.Any(), permissionID).Return(&permissiondomain.Permission{PermissionID: permissionID, HTTPMethod: "GET", PathTemplate: "/api/v1/users", Active: true}, nil)
			store.EXPECT().Update(gomock.Any(), gomock.Any()).Return(&permissiondomain.Permission{PermissionID: permissionID, HTTPMethod: "GET", PathTemplate: "/api/v1/users", Active: true}, nil)
		}, wantReason: "permission_updated"},
		{name: "enable", run: func(t *testing.T, service PermissionCommandService) *PermissionResult {
			result, err := service.EnablePermission(context.Background(), SetPermissionActiveCommand{PermissionID: permissionID})
			if err != nil {
				t.Fatalf("EnablePermission: %v", err)
			}
			return result
		}, setupStore: func(store *MockPermissionStore) {
			store.EXPECT().SetActive(gomock.Any(), permissionID, true).Return(&permissiondomain.Permission{PermissionID: permissionID, HTTPMethod: "GET", PathTemplate: "/api/v1/users", Active: true}, nil)
		}, wantReason: "permission_active_changed"},
		{name: "disable", run: func(t *testing.T, service PermissionCommandService) *PermissionResult {
			result, err := service.DisablePermission(context.Background(), SetPermissionActiveCommand{PermissionID: permissionID})
			if err != nil {
				t.Fatalf("DisablePermission: %v", err)
			}
			return result
		}, setupStore: func(store *MockPermissionStore) {
			store.EXPECT().SetActive(gomock.Any(), permissionID, false).Return(&permissiondomain.Permission{PermissionID: permissionID, HTTPMethod: "GET", PathTemplate: "/api/v1/users", Active: false}, nil)
		}, wantReason: "permission_active_changed"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := NewMockPermissionStore(gomock.NewController(t))
			tt.setupStore(store)
			notifier := NewMockPolicyChangeNotifier(gomock.NewController(t))
			notifier.EXPECT().NotifyPolicyChanged(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, change permissionapplication.PolicyChange) error {
				if change.Reason != tt.wantReason {
					t.Fatalf("reason = %q", change.Reason)
				}
				return refreshErr
			})
			service := NewPermissionCommandService(PermissionCommandParams{Store: store, PolicyChanges: notifier})
			result := tt.run(t, service)
			if result == nil || result.Permission.PermissionID == uuid.Nil {
				t.Fatalf("result = %#v", result)
			}
		})
	}
}
