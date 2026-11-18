package casbin

import "uuid"

// roleSubject 将角色标识格式化为权限规则中可识别的主体格式（如 role:admin）。
func roleSubject(roleID uuid.UUID) string {
	return "role:" + roleID.String()
}
