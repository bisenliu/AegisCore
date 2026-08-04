package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	roledomain "github.com/aegiscore/user-service/internal/features/role/domain"
	"github.com/aegiscore/user-service/internal/persistence/ent"
	entrole "github.com/aegiscore/user-service/internal/persistence/ent/role"
)

// lockOrdinaryRole 在普通写事务中锁定角色并拒绝系统角色。
func lockOrdinaryRole(ctx context.Context, tx *ent.Tx, roleID uuid.UUID) (*ent.Role, error) {
	role, err := tx.Role.Query().
		Where(entrole.RoleIDEQ(roleID)).
		ForUpdate().
		Only(ctx)
	if err == nil {
		if role.IsSystem {
			return nil, roledomain.ErrSystemRoleProtected
		}
		return role, nil
	}
	if ent.IsNotFound(err) {
		return nil, roledomain.ErrRoleNotFound
	}
	return nil, fmt.Errorf("lock role by role_id %s: %w", roleID.String(), err)
}
