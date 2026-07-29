package postgres

import (
	permissionapplication "github.com/aegiscore/user-service/internal/features/permission/application"
	entpermission "github.com/aegiscore/user-service/internal/persistence/ent/permission"
	"github.com/aegiscore/user-service/internal/persistence/ent/predicate"
)

func buildListPredicates(input permissionapplication.ListPermissionsInput) []predicate.Permission {
	predicates := make([]predicate.Permission, 0, 2)
	if input.Module != "" {
		predicates = append(predicates, entpermission.ModuleEQ(input.Module))
	}
	if input.HTTPMethod != "" {
		predicates = append(predicates, entpermission.HTTPMethodEQ(input.HTTPMethod))
	}
	return predicates
}
