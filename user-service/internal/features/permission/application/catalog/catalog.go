package catalog

// PermissionSpec 描述人工维护的系统权限基线。
type PermissionSpec struct {
	PermissionID string
	Name         string
	Description  string
	Module       string
	Method       string
	PathTemplate string
	System       bool
}

// DefaultPermissions 返回用户服务当前人工维护的系统权限基线。
func DefaultPermissions() []PermissionSpec {
	return []PermissionSpec{
		{
			PermissionID: "00000000-0000-0000-0000-000000010001",
			Name:         "查询用户列表",
			Description:  "允许分页查询用户资料",
			Module:       "user",
			Method:       "GET",
			PathTemplate: "/api/v1/users",
			System:       true,
		},
		{
			PermissionID: "00000000-0000-0000-0000-000000010002",
			Name:         "创建用户",
			Description:  "允许创建用户资料",
			Module:       "user",
			Method:       "POST",
			PathTemplate: "/api/v1/users",
			System:       true,
		},
		{
			PermissionID: "00000000-0000-0000-0000-000000010003",
			Name:         "查看用户详情",
			Description:  "允许按用户 ID 查询用户资料",
			Module:       "user",
			Method:       "GET",
			PathTemplate: "/api/v1/users/:user_id",
			System:       true,
		},
	}
}
