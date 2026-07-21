package postgres

import (
	entpermission "github.com/aegiscore/user-service/ent/permission"
	"github.com/aegiscore/user-service/ent/predicate"
	permissionapplication "github.com/aegiscore/user-service/internal/features/permission/application"
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
