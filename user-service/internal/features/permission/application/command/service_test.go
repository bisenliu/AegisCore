package command

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	permissionapplication "github.com/aegiscore/user-service/internal/features/permission/application"
	permissiondomain "github.com/aegiscore/user-service/internal/features/permission/domain"
)

func TestPermissionCommandServiceCreateAndProtectSystemPermission(t *testing.T) {
	store := NewMockPermissionStore(gomock.NewController(t))
	notifier := NewMockPolicyChangeNotifier(gomock.NewController(t))
	store.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, input permissionapplication.CreatePermissionInput) (*permissiondomain.Permission, error) {
		return &permissiondomain.Permission{PermissionID: input.PermissionID, Name: input.Name, Description: input.Description, Module: input.Module, HTTPMethod: input.HTTPMethod, PathTemplate: input.PathTemplate, Active: input.Active}, nil
	})
	notifier.EXPECT().NotifyPolicyChanged(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, change permissionapplication.PolicyChange) error {
		require.Equal(t, "permission_created", change.Reason)
		return nil
	})
	service := NewPermissionCommandService(PermissionCommandParams{Store: store, PolicyChanges: notifier})

	created, err := service.CreatePermission(context.Background(), CreatePermissionCommand{Name: "List Users", Module: "user", HTTPMethod: "get", PathTemplate: "/api/v1/users"})
	require.NoError(t, err)
	require.Equal(t, "GET", created.Permission.HTTPMethod)
	require.True(t, created.Permission.Active)
	require.False(t, created.Permission.IsSystem)

	store = NewMockPermissionStore(gomock.NewController(t))
	service = NewPermissionCommandService(PermissionCommandParams{Store: store, PolicyChanges: NewMockPolicyChangeNotifier(gomock.NewController(t))})
	systemPermission := created.Permission
	systemPermission.IsSystem = true
	store.EXPECT().GetByPermissionID(gomock.Any(), created.Permission.PermissionID).Return(&systemPermission, nil)
	err = service.UpdatePermission(context.Background(), UpdatePermissionCommand{PermissionID: created.Permission.PermissionID, Name: "List Users", Module: "user", HTTPMethod: "POST", PathTemplate: "/api/v1/users", Active: true})
	require.ErrorIs(t, err, permissiondomain.ErrSystemPermissionProtected)
}

func TestPermissionCommandServiceSetActive(t *testing.T) {
	permissionID := uuid.MustParse("018f0000-0000-7000-8000-000000000001")
	store := NewMockPermissionStore(gomock.NewController(t))
	notifier := NewMockPolicyChangeNotifier(gomock.NewController(t))
	store.EXPECT().SetActive(gomock.Any(), permissionID, false).Return(nil)
	notifier.EXPECT().NotifyPolicyChanged(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, change permissionapplication.PolicyChange) error {
		require.Equal(t, "permission_active_changed", change.Reason)
		return nil
	})
	service := NewPermissionCommandService(PermissionCommandParams{Store: store, PolicyChanges: notifier})

	err := service.DisablePermission(context.Background(), SetPermissionActiveCommand{PermissionID: permissionID})
	require.NoError(t, err)
}

func TestPermissionCommandServiceCreateMapsDuplicateAndShortCircuitsValidation(t *testing.T) {
	store := NewMockPermissionStore(gomock.NewController(t))
	store.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil, permissiondomain.ErrPermissionAlreadyExists)
	service := NewPermissionCommandService(PermissionCommandParams{Store: store, PolicyChanges: NewMockPolicyChangeNotifier(gomock.NewController(t))})

	_, err := service.CreatePermission(context.Background(), CreatePermissionCommand{Name: "List Users", Module: "user", HTTPMethod: "GET", PathTemplate: "/api/v1/users"})
	require.ErrorIs(t, err, permissiondomain.ErrPermissionAlreadyExists)

	store = NewMockPermissionStore(gomock.NewController(t))
	service = NewPermissionCommandService(PermissionCommandParams{Store: store, PolicyChanges: NewMockPolicyChangeNotifier(gomock.NewController(t))})
	_, err = service.CreatePermission(context.Background(), CreatePermissionCommand{Name: "", Module: "user", HTTPMethod: "GET", PathTemplate: "/api/v1/users"})
	require.Error(t, err)
}

func TestPermissionCommandServiceUpdateNonSystemNormalizesAndMapsDuplicate(t *testing.T) {
	permissionID := uuid.MustParse("018f0000-0000-7000-8000-000000000002")
	store := NewMockPermissionStore(gomock.NewController(t))
	notifier := NewMockPolicyChangeNotifier(gomock.NewController(t))
	store.EXPECT().GetByPermissionID(gomock.Any(), permissionID).Return(&permissiondomain.Permission{PermissionID: permissionID, HTTPMethod: "GET", PathTemplate: "/api/v1/users", Active: true}, nil)
	store.EXPECT().Update(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, input permissionapplication.UpdatePermissionInput) error {
		require.Equal(t, "Create User", input.Name)
		require.Equal(t, "Create users", input.Description)
		require.Equal(t, "user", input.Module)
		require.Equal(t, "POST", input.HTTPMethod)
		require.Equal(t, "/api/v1/users", input.PathTemplate)
		return nil
	})
	notifier.EXPECT().NotifyPolicyChanged(gomock.Any(), gomock.Any()).Return(nil)
	service := NewPermissionCommandService(PermissionCommandParams{Store: store, PolicyChanges: notifier})

	err := service.UpdatePermission(context.Background(), UpdatePermissionCommand{PermissionID: permissionID, Name: "  Create User  ", Description: "  Create users  ", Module: "  user  ", HTTPMethod: "post", PathTemplate: "/api/v1/users", Active: true})
	require.NoError(t, err)

	store = NewMockPermissionStore(gomock.NewController(t))
	service = NewPermissionCommandService(PermissionCommandParams{Store: store, PolicyChanges: NewMockPolicyChangeNotifier(gomock.NewController(t))})
	store.EXPECT().GetByPermissionID(gomock.Any(), permissionID).Return(&permissiondomain.Permission{PermissionID: permissionID, HTTPMethod: "GET", PathTemplate: "/api/v1/users", Active: true}, nil)
	store.EXPECT().Update(gomock.Any(), gomock.Any()).Return(permissiondomain.ErrPermissionAlreadyExists)
	err = service.UpdatePermission(context.Background(), UpdatePermissionCommand{PermissionID: permissionID, Name: "Create User", Module: "user", HTTPMethod: "POST", PathTemplate: "/api/v1/users", Active: true})
	require.ErrorIs(t, err, permissiondomain.ErrPermissionAlreadyExists)
}

func TestPermissionCommandServiceEnablePermission(t *testing.T) {
	permissionID := uuid.MustParse("018f0000-0000-7000-8000-000000000003")
	store := NewMockPermissionStore(gomock.NewController(t))
	notifier := NewMockPolicyChangeNotifier(gomock.NewController(t))
	store.EXPECT().SetActive(gomock.Any(), permissionID, true).Return(nil)
	notifier.EXPECT().NotifyPolicyChanged(gomock.Any(), gomock.Any()).Return(nil)
	service := NewPermissionCommandService(PermissionCommandParams{Store: store, PolicyChanges: notifier})

	err := service.EnablePermission(context.Background(), SetPermissionActiveCommand{PermissionID: permissionID})
	require.NoError(t, err)
}

func TestPermissionCommandServiceRequiresPolicyChangeNotifier(t *testing.T) {
	require.PanicsWithValue(t, "permission policy change notifier is required", func() {
		NewPermissionCommandService(PermissionCommandParams{})
	})
}

func TestPermissionCommandServicePropagatesRefreshFailureAfterSuccessfulWrite(t *testing.T) {
	permissionID := uuid.MustParse("018f0000-0000-7000-8000-000000000004")
	refreshErr := errors.New("refresh failed")

	tests := []struct {
		name       string
		run        func(PermissionCommandService) error
		setupStore func(*MockPermissionStore)
		wantReason string
	}{
		{name: "create", run: func(service PermissionCommandService) error {
			result, err := service.CreatePermission(context.Background(), CreatePermissionCommand{Name: "List Users", Module: "user", HTTPMethod: "GET", PathTemplate: "/api/v1/users"})
			require.Nil(t, result)
			return err
		}, setupStore: func(store *MockPermissionStore) {
			store.EXPECT().Create(gomock.Any(), gomock.Any()).Return(&permissiondomain.Permission{PermissionID: permissionID, HTTPMethod: "GET", PathTemplate: "/api/v1/users", Active: true}, nil)
		}, wantReason: "permission_created"},
		{name: "update", run: func(service PermissionCommandService) error {
			return service.UpdatePermission(context.Background(), UpdatePermissionCommand{PermissionID: permissionID, Name: "List Users", Module: "user", HTTPMethod: "GET", PathTemplate: "/api/v1/users", Active: true})
		}, setupStore: func(store *MockPermissionStore) {
			store.EXPECT().GetByPermissionID(gomock.Any(), permissionID).Return(&permissiondomain.Permission{PermissionID: permissionID, HTTPMethod: "GET", PathTemplate: "/api/v1/users", Active: true}, nil)
			store.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)
		}, wantReason: "permission_updated"},
		{name: "enable", run: func(service PermissionCommandService) error {
			return service.EnablePermission(context.Background(), SetPermissionActiveCommand{PermissionID: permissionID})
		}, setupStore: func(store *MockPermissionStore) {
			store.EXPECT().SetActive(gomock.Any(), permissionID, true).Return(nil)
		}, wantReason: "permission_active_changed"},
		{name: "disable", run: func(service PermissionCommandService) error {
			return service.DisablePermission(context.Background(), SetPermissionActiveCommand{PermissionID: permissionID})
		}, setupStore: func(store *MockPermissionStore) {
			store.EXPECT().SetActive(gomock.Any(), permissionID, false).Return(nil)
		}, wantReason: "permission_active_changed"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := NewMockPermissionStore(gomock.NewController(t))
			tt.setupStore(store)
			notifier := NewMockPolicyChangeNotifier(gomock.NewController(t))
			notifier.EXPECT().NotifyPolicyChanged(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, change permissionapplication.PolicyChange) error {
				require.Equal(t, tt.wantReason, change.Reason)
				return refreshErr
			})
			service := NewPermissionCommandService(PermissionCommandParams{Store: store, PolicyChanges: notifier})
			require.ErrorIs(t, tt.run(service), refreshErr)
		})
	}
}
