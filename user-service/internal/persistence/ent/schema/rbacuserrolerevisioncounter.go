package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
)

// RbacUserRoleRevisionCounter 是在线用户角色绑定 mutation 的提交顺序 revision counter。
type RbacUserRoleRevisionCounter struct {
	ent.Schema
}

// Mixin 返回 user-role revision counter 表复用的公共 schema mixin。
func (RbacUserRoleRevisionCounter) Mixin() []ent.Mixin {
	return []ent.Mixin{databaseCommentMixin{}}
}

// Fields 返回 user-role revision counter 表字段定义。
func (RbacUserRoleRevisionCounter) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("id").Immutable().Comment("固定 RBAC user-role revision counter ID"),
		field.Int64("last_revision").Default(0).NonNegative().Comment("最近已分配的 RBAC user-role revision"),
	}
}
