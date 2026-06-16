package postgres

import (
	"github.com/aegiscore/user-service/ent/predicate"
	entrole "github.com/aegiscore/user-service/ent/role"
	roleapplication "github.com/aegiscore/user-service/internal/features/role/application"
)

func buildListPredicates(input roleapplication.ListRolesInput) []predicate.Role {
	predicates := make([]predicate.Role, 0, 2)
	if input.Active != nil {
		predicates = append(predicates, entrole.ActiveEQ(*input.Active))
	}
	if input.IsSystem != nil {
		predicates = append(predicates, entrole.IsSystemEQ(*input.IsSystem))
	}
	return predicates
}
