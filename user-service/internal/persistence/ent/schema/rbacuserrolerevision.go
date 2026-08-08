package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
)

// RbacUserRoleRevision 是用户角色绑定缓存失效的数据库权威 revision schema。
type RbacUserRoleRevision struct {
	ent.Schema
}

// Mixin 返回 user-role revision 表复用的公共 schema mixin。
func (RbacUserRoleRevision) Mixin() []ent.Mixin {
	return []ent.Mixin{databaseCommentMixin{}, createdAtMillisMixin{}}
}

// Fields 返回 user-role revision 表字段定义。
func (RbacUserRoleRevision) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("id").StorageKey("revision").Unique().Immutable().Comment("单调递增的 RBAC user-role revision"),
		field.String("reason").NotEmpty().MaxLen(64).Immutable().Comment("触发用户角色缓存失效的稳定原因"),
		field.UUID("user_id", uuid.UUID{}).Immutable().Comment("相关外部用户ID"),
		field.UUID("role_id", uuid.UUID{}).Optional().Nillable().Immutable().Comment("相关外部角色ID"),
	}
}

// Edges 返回 user-role revision 与 outbox event 的一对一关联。
func (RbacUserRoleRevision) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("outbox_event", RbacPolicyOutboxEvent.Type).Unique(),
	}
}
