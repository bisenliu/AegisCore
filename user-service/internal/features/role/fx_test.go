package role

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"

	commonvalidation "github.com/aegiscore/common/validation"
	permissionapplication "github.com/aegiscore/user-service/internal/features/permission/application"
	permissiondomain "github.com/aegiscore/user-service/internal/features/permission/domain"
	roleapplication "github.com/aegiscore/user-service/internal/features/role/application"
	rolecommand "github.com/aegiscore/user-service/internal/features/role/application/command"
	rolequery "github.com/aegiscore/user-service/internal/features/role/application/query"
	roledomain "github.com/aegiscore/user-service/internal/features/role/domain"
	rolehttp "github.com/aegiscore/user-service/internal/features/role/transport/http"
)

func TestRoleModuleBuildsWithCompositionAdapters(t *testing.T) {
	validator, err := commonvalidation.NewDefault()
	require.NoError(t, err)
	store := roleModuleStore{}
	permissionStore := roleModulePermissionStore{}
	permissionCatalogStore := roleModulePermissionCatalogStore{}
	notifier := roleModulePolicyNotifier{}

	var commands rolecommand.RoleCommandService
	var queries rolequery.RoleQueryService
	var controller *rolehttp.RoleController
	var lookup roleapplication.PermissionLookup
	app := fxtest.New(t,
		Module,
		fx.Supply(
			validator,
			fx.Annotate(permissionCatalogStore, fx.As(new(permissionapplication.PermissionStore))),
		),
		fx.Provide(func() permissionapplication.PolicyChangeNotifier { return notifier }),
		fx.Replace(
			fx.Annotate(store, fx.As(new(roleapplication.RoleStore))),
			fx.Annotate(store, fx.As(new(roleapplication.UserRoleStore))),
			fx.Annotate(permissionStore, fx.As(new(roleapplication.RolePermissionStore))),
		),
		fx.Populate(&commands, &queries, &controller, &lookup),
	)
	app.RequireStart().RequireStop()

	require.NotNil(t, commands)
	require.NotNil(t, queries)
	require.NotNil(t, controller)
	require.NotNil(t, lookup)
}

type roleModulePolicyNotifier struct{}

func (roleModulePolicyNotifier) NotifyPolicyChanged(context.Context, int64, permissionapplication.PolicyChange) error {
	return nil
}

type roleModuleStore struct{}

func (roleModuleStore) Create(context.Context, roleapplication.CreateRoleInput, roleapplication.PolicyChange) (*roleapplication.RoleWriteResult, error) {
	return nil, nil
}

func (roleModuleStore) GetByRoleID(context.Context, uuid.UUID) (*roledomain.Role, error) {
	return nil, nil
}

func (roleModuleStore) GetByRoleIDs(context.Context, []uuid.UUID) ([]roledomain.Role, error) {
	return nil, nil
}

func (roleModuleStore) List(context.Context, roleapplication.ListRolesInput) ([]roledomain.Role, bool, error) {
	return nil, false, nil
}

func (roleModuleStore) Update(context.Context, roleapplication.UpdateRoleInput, roleapplication.PolicyChange) (roleapplication.PolicyWriteResult, error) {
	return roleapplication.PolicyWriteResult{}, nil
}

func (roleModuleStore) SetActive(context.Context, uuid.UUID, bool, roleapplication.PolicyChange) (roleapplication.PolicyWriteResult, error) {
	return roleapplication.PolicyWriteResult{}, nil
}

func (roleModuleStore) ListByUserID(context.Context, uuid.UUID) ([]roledomain.Role, error) {
	return nil, nil
}

func (roleModuleStore) Add(context.Context, uuid.UUID, uuid.UUID, roleapplication.PolicyChange) (roleapplication.PolicyWriteResult, error) {
	return roleapplication.PolicyWriteResult{}, nil
}

func (roleModuleStore) Replace(context.Context, uuid.UUID, []uuid.UUID, roleapplication.PolicyChange) (roleapplication.RolesWriteResult, error) {
	return roleapplication.RolesWriteResult{}, nil
}

func (roleModuleStore) Remove(context.Context, uuid.UUID, uuid.UUID, roleapplication.PolicyChange) (roleapplication.PolicyWriteResult, error) {
	return roleapplication.PolicyWriteResult{}, nil
}

func (roleModuleStore) ListByRoleID(context.Context, uuid.UUID) ([]roleapplication.PermissionReference, error) {
	return nil, nil
}

func (roleModuleStore) AddRolePermission(context.Context, uuid.UUID, roleapplication.PermissionReference) error {
	return nil
}

func (roleModuleStore) ReplaceRolePermissions(context.Context, uuid.UUID, []roleapplication.PermissionReference) ([]roleapplication.PermissionReference, error) {
	return nil, nil
}

func (roleModuleStore) RemoveRolePermission(context.Context, uuid.UUID, uuid.UUID) error {
	return nil
}

func (roleModuleStore) GetByPermissionID(context.Context, uuid.UUID) (*roleapplication.PermissionReference, error) {
	return nil, nil
}

var _ roleapplication.RoleStore = roleModuleStore{}
var _ roleapplication.UserRoleStore = roleModuleStore{}
var _ roleapplication.PermissionLookup = roleModuleStore{}

type roleModulePermissionStore struct {
	roleModuleStore
}

func (roleModulePermissionStore) Add(context.Context, uuid.UUID, roleapplication.PermissionReference, roleapplication.PolicyChange) (roleapplication.PolicyWriteResult, error) {
	return roleapplication.PolicyWriteResult{}, nil
}

func (roleModulePermissionStore) Replace(context.Context, uuid.UUID, []roleapplication.PermissionReference, roleapplication.PolicyChange) (roleapplication.PermissionsWriteResult, error) {
	return roleapplication.PermissionsWriteResult{}, nil
}

func (roleModulePermissionStore) Remove(context.Context, uuid.UUID, uuid.UUID, roleapplication.PolicyChange) (roleapplication.PolicyWriteResult, error) {
	return roleapplication.PolicyWriteResult{}, nil
}

var _ roleapplication.RolePermissionStore = roleModulePermissionStore{}

type roleModulePermissionCatalogStore struct{}

func (roleModulePermissionCatalogStore) GetByPermissionID(context.Context, uuid.UUID) (*permissiondomain.Permission, error) {
	return nil, nil
}

func (roleModulePermissionCatalogStore) List(context.Context, permissionapplication.ListPermissionsInput) ([]permissiondomain.Permission, error) {
	return nil, nil
}

func (roleModulePermissionCatalogStore) ListEffectiveByUserID(context.Context, uuid.UUID) ([]permissiondomain.Permission, error) {
	return nil, nil
}

var _ permissionapplication.PermissionStore = roleModulePermissionCatalogStore{}
