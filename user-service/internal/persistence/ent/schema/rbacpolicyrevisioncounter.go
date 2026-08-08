package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
)

// RbacPolicyRevisionCounter 是在线 RBAC policy mutation 的提交顺序 revision counter。
type RbacPolicyRevisionCounter struct {
	ent.Schema
}

// Mixin 返回 revision counter 表复用的公共 schema mixin。
func (RbacPolicyRevisionCounter) Mixin() []ent.Mixin {
	return []ent.Mixin{databaseCommentMixin{}}
}

// Fields 返回 revision counter 表字段定义。
func (RbacPolicyRevisionCounter) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("id").Immutable().Comment("固定 RBAC policy revision counter ID"),
		field.Int64("last_revision").Default(0).NonNegative().Comment("最近已分配的 RBAC policy revision"),
	}
}
