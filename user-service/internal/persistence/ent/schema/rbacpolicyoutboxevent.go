package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

const (
	rbacPolicyOutboxKindPolicyChanged   = "policy_changed"
	rbacPolicyOutboxKindUserRoleChanged = "user_role_changed"
	rbacPolicyOutboxStatusPending       = "pending"
	rbacPolicyOutboxStatusProcessing    = "processing"
	rbacPolicyOutboxStatusFailed        = "failed"
	rbacPolicyOutboxStatusDelivered     = "delivered"
	defaultRbacPolicyOutboxStatus       = rbacPolicyOutboxStatusPending
)

// RbacPolicyOutboxEvent 是待投递 RBAC policy 变更事件的 Ent schema。
type RbacPolicyOutboxEvent struct {
	ent.Schema
}

// Mixin 返回 policy outbox 表复用的公共 schema mixin。
func (RbacPolicyOutboxEvent) Mixin() []ent.Mixin {
	return []ent.Mixin{databaseCommentMixin{}, createdAtMillisMixin{}, updatedAtMillisMixin{}}
}

// Fields 返回 policy outbox 表字段定义。
func (RbacPolicyOutboxEvent) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("id").Unique().Immutable().Comment("RBAC policy outbox 内部ID"),
		field.UUID("event_id", uuid.UUID{}).Unique().Immutable().Comment("稳定 outbox event ID"),
		field.Int64("policy_revision").Optional().Nillable().Immutable().Comment("关联的 RBAC policy revision"),
		field.Int64("user_role_revision").Optional().Nillable().Immutable().Comment("关联的 RBAC user-role revision"),
		field.String("kind").NotEmpty().Immutable().MaxLen(32).Validate(oneOfStrings(rbacPolicyOutboxKindPolicyChanged, rbacPolicyOutboxKindUserRoleChanged)).Comment("事件类型"),
		field.String("reason").NotEmpty().Immutable().MaxLen(64).Comment("触发 policy 变更的稳定原因"),
		field.UUID("role_id", uuid.UUID{}).Optional().Nillable().Immutable().Comment("相关外部角色ID"),
		field.UUID("user_id", uuid.UUID{}).Optional().Nillable().Immutable().Comment("相关外部用户ID"),
		field.UUID("permission_id", uuid.UUID{}).Optional().Nillable().Immutable().Comment("相关外部权限ID"),
		field.String("status").Default(defaultRbacPolicyOutboxStatus).MaxLen(16).Validate(oneOfStrings(rbacPolicyOutboxStatusPending, rbacPolicyOutboxStatusProcessing, rbacPolicyOutboxStatusFailed, rbacPolicyOutboxStatusDelivered)).Comment("投递状态"),
		field.Int("attempt_count").Default(0).NonNegative().Comment("投递尝试次数"),
		field.Int64("next_attempt_at").DefaultFunc(func() int64 { return time.Now().UnixMilli() }).Comment("下次允许尝试的时间戳毫秒"),
		field.String("last_error").Optional().Nillable().MaxLen(2048).Comment("最近一次投递错误"),
		field.UUID("claim_token", uuid.UUID{}).Optional().Nillable().Comment("当前 dispatcher claim token"),
		field.Int64("claimed_until").Optional().Nillable().Comment("当前 claim lease 截止时间戳毫秒"),
		field.String("idempotency_key").NotEmpty().Unique().Immutable().MaxLen(128).Comment("稳定投递幂等键"),
		field.Int64("delivered_at").Optional().Nillable().Comment("投递完成时间戳毫秒"),
	}
}

// Edges 返回 outbox event 与 policy revision 的一对一关联。
func (RbacPolicyOutboxEvent) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("policy_revision_edge", RbacPolicyRevision.Type).Ref("outbox_event").Field("policy_revision").Unique().Immutable(),
		edge.From("user_role_revision_edge", RbacUserRoleRevision.Type).Ref("outbox_event").Field("user_role_revision").Unique().Immutable(),
	}
}

// Indexes 返回 dispatcher 后续按状态和重试时间扫描所需索引。
func (RbacPolicyOutboxEvent) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("status", "next_attempt_at", "policy_revision", "user_role_revision"),
		index.Fields("status", "claimed_until", "policy_revision", "user_role_revision"),
	}
}
