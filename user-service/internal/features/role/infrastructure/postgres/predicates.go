package postgres

import (
	roleapplication "github.com/aegiscore/user-service/internal/features/role/application"
	"github.com/aegiscore/user-service/internal/persistence/ent/predicate"
	entrole "github.com/aegiscore/user-service/internal/persistence/ent/role"
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
