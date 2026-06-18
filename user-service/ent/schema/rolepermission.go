package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// RolePermission 是 role_permissions 表的 Ent schema。
type RolePermission struct {
	ent.Schema
}

// Mixin 返回 role_permissions 表复用的公共 schema mixin。
func (RolePermission) Mixin() []ent.Mixin {
	return []ent.Mixin{databaseCommentMixin{}}
}

// Fields 返回角色权限绑定表字段定义。
func (RolePermission) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("id").Unique().Immutable().Comment("角色权限绑定ID"),
		field.Int64("role_id").Comment("角色内部ID"),
		field.Int64("permission_id").Comment("权限内部ID"),
		field.Int64("created_at").DefaultFunc(func() int64 { return time.Now().UnixMilli() }).Immutable().Comment("创建时间戳毫秒"),
	}
}

// Edges 返回角色权限绑定和角色、权限的关联定义。
func (RolePermission) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("role", Role.Type).Ref("role_permissions").Field("role_id").Required().Unique(),
		edge.From("permission", Permission.Type).Ref("role_permissions").Field("permission_id").Required().Unique(),
	}
}

// Indexes 返回角色权限绑定表唯一约束定义。
func (RolePermission) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("role_id", "permission_id").Unique(),
	}
}
