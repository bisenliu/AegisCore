package userschema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// defaultUserStatus 是 users.status 的持久化默认值，调用方未显式传入状态时新用户默认为正常状态。
// 该值必须与 domain.UserStatusNormal 保持一致，避免 schema 默认值和领域默认值漂移。
const defaultUserStatus = 100

// Fields 返回 users 表列定义，包括认证字段和时间戳默认值。
func Fields() []ent.Field {
	return []ent.Field{
		field.Int64("id").Unique().Immutable().Comment("用户ID"),
		field.UUID("user_id", uuid.UUID{}).Unique().Immutable().Comment("外部用户ID"),
		field.String("nickname").NotEmpty().MaxLen(128).Comment("用户昵称"),
		field.String("username").NotEmpty().Unique().MaxLen(255).Comment("用户名"),
		field.String("password_hash").NotEmpty().Comment("密码哈希"),
		field.Int64("token_version").Default(1).Comment("认证令牌版本"),
		field.Int64("status").Default(defaultUserStatus).Comment("用户状态：100 正常，200 冻结/停用，300 必须修改密码"),
		field.Int64("deleted_at").Optional().Nillable().Comment("软删除时间戳毫秒，NULL 表示未删除"),
		field.Int64("created_at").DefaultFunc(func() int64 { return time.Now().UnixMilli() }).Immutable().Comment("创建时间戳毫秒"),
		field.Int64("updated_at").DefaultFunc(func() int64 { return time.Now().UnixMilli() }).UpdateDefault(func() int64 { return time.Now().UnixMilli() }).Comment("更新时间戳毫秒"),
	}
}

// Indexes 返回支持 nickname、status 和软删除过滤查询的索引。
func Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("nickname"),
		index.Fields("status"),
		index.Fields("deleted_at"),
	}
}
