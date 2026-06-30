package casbin

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	entrole "github.com/aegiscore/user-service/ent/role"
	entuser "github.com/aegiscore/user-service/ent/user"
	entuserrole "github.com/aegiscore/user-service/ent/userrole"
)

func (r *entUserRoleResolver) loadRolesForUser(ctx context.Context, userID uuid.UUID) ([]uuid.UUID, error) {
	var rows []struct {
		RoleID uuid.UUID `json:"role_id,omitempty"`
	}
	err := r.client.Role.Query().
		Where(
			entrole.ActiveEQ(true),
			entrole.HasUserRolesWith(
				entuserrole.HasUserWith(entuser.UserIDEQ(userID), entuser.DeletedAtIsNil()),
			),
		).
		Order(entrole.ByRoleID()).
		Select(entrole.FieldRoleID).
		Scan(ctx, &rows)
	if err != nil {
		return nil, fmt.Errorf("load user role policy subjects for user %s: %w", userID.String(), err)
	}
	roleIDs := make([]uuid.UUID, 0, len(rows))
	for _, row := range rows {
		roleIDs = append(roleIDs, row.RoleID)
	}
	return roleIDs, nil
}
