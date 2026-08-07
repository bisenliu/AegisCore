package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
)

// RbacPolicyRevision 是 RBAC policy 变更的数据库权威 revision schema。
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
		field.UUID("role_id", uuid.UUID{}).Optional().Nillable().Immutable().Comment("相关外部角色ID"),
		field.UUID("user_id", uuid.UUID{}).Optional().Nillable().Immutable().Comment("相关外部用户ID"),
		field.UUID("permission_id", uuid.UUID{}).Optional().Nillable().Immutable().Comment("相关外部权限ID"),
	}
}

// Edges 返回 policy revision 与 outbox event 的一对一关联。
func (RbacPolicyRevision) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("outbox_event", RbacPolicyOutboxEvent.Type).Unique(),
	}
}
