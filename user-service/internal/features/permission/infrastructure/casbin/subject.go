package casbin

import "github.com/google/uuid"

func roleSubject(roleID uuid.UUID) string {
	return "role:" + roleID.String()
}
