package schema

import (
	"uuid"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
)

// RbacPolicyRevision 是会改变 Casbin 静态 policy 的 RBAC 变更数据库权威 revision schema。
type RbacPolicyRevision struct {
	ent.Schema
}

// Mixin 返回 policy revision 表复用的公共 schema mixin。
func (RbacPolicyRevision) Mixin() []ent.Mixin {
	return []ent.Mixin{databaseCommentMixin{}, createdAtMillisMixin{}}
}

// Fields 返回 policy revision 表字段定义。
func (RbacPolicyRevision) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("id").StorageKey("revision").Unique().Immutable().Comment("单调递增的 RBAC policy revision"),
		field.String("reason").NotEmpty().MaxLen(64).Immutable().Comment("触发 policy 变更的稳定原因"),
		field.String("role_id").GoType((*uuid.UUID)(nil)).ValueScanner(nullableUUIDScanner).SchemaType(uuidColumnTypes).Optional().Nillable().Immutable().Comment("相关外部角色ID"),
		field.String("permission_id").GoType((*uuid.UUID)(nil)).ValueScanner(nullableUUIDScanner).SchemaType(uuidColumnTypes).Optional().Nillable().Immutable().Comment("相关外部权限ID"),
	}
}

// Edges 返回 policy revision 与 outbox event 的一对一关联。
func (RbacPolicyRevision) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("outbox_event", RbacPolicyOutboxEvent.Type).Unique(),
	}
}
