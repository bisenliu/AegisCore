package casbin

import (
	"context"
	"fmt"
	"uuid"

	entrole "github.com/aegiscore/user-service/internal/persistence/ent/role"
	entuser "github.com/aegiscore/user-service/internal/persistence/ent/user"
	entuserrole "github.com/aegiscore/user-service/internal/persistence/ent/userrole"
)

func (r *entUserRoleResolver) loadRolesForUser(ctx context.Context, userID uuid.UUID) ([]uuid.UUID, error) {
	var rows []struct {
		RoleID string `json:"role_id,omitempty"`
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
		roleID, err := uuid.Parse(row.RoleID)
		if err != nil {
			return nil, fmt.Errorf("parse role policy subject %q for user %s: %w", row.RoleID, userID.String(), err)
		}
		roleIDs = append(roleIDs, roleID)
	}
	return roleIDs, nil
}
