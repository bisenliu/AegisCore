package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// Role 是 roles 表的 Ent schema。
type Role struct {
	ent.Schema
}

// Mixin 返回 roles 表复用的公共 schema mixin。
func (Role) Mixin() []ent.Mixin {
	return []ent.Mixin{databaseCommentMixin{}}
}

// Fields 返回角色表字段定义。
func (Role) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("id").Unique().Immutable().Comment("角色ID"),
		field.UUID("role_id", uuid.UUID{}).Unique().Immutable().Comment("外部角色ID"),
		field.String("name").NotEmpty().MaxLen(128).Comment("角色名称"),
		field.String("description").Default("").MaxLen(512).Comment("角色说明"),
		field.Bool("active").Default(true).Comment("角色是否启用"),
		field.Bool("is_system").Default(false).Comment("是否系统角色"),
		field.Int64("created_at").DefaultFunc(func() int64 { return time.Now().UnixMilli() }).Immutable().Comment("创建时间戳毫秒"),
		field.Int64("updated_at").DefaultFunc(func() int64 { return time.Now().UnixMilli() }).UpdateDefault(func() int64 { return time.Now().UnixMilli() }).Comment("更新时间戳毫秒"),
	}
}

// Edges 返回角色和用户角色、角色权限绑定的关联定义。
func (Role) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("user_roles", UserRole.Type),
		edge.To("role_permissions", RolePermission.Type),
	}
}

// Indexes 返回支持角色列表过滤和授权回源排序的索引。
func (Role) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("active", "role_id"),
		index.Fields("is_system", "role_id"),
	}
}
