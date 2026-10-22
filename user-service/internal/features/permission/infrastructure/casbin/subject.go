package casbin

import "uuid"

func roleSubject(roleID uuid.UUID) string {
	return "role:" + roleID.String()
}
