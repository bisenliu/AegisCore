package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// UserRole 是 user_roles 表的 Ent schema。
type UserRole struct {
	ent.Schema
}

// Fields 返回用户角色绑定表字段定义。
func (UserRole) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("id").Unique().Immutable().Comment("用户角色绑定ID"),
		field.Int64("user_id").Comment("用户内部ID"),
		field.Int64("role_id").Comment("角色内部ID"),
		field.Int64("created_at").DefaultFunc(func() int64 { return time.Now().UnixMilli() }).Immutable().Comment("创建时间戳毫秒"),
	}
}

// Edges 返回用户角色绑定和用户、角色的关联定义。
func (UserRole) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).Ref("user_roles").Field("user_id").Required().Unique(),
		edge.From("role", Role.Type).Ref("user_roles").Field("role_id").Required().Unique(),
	}
}

// Indexes 返回用户角色绑定表唯一约束定义。
func (UserRole) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id", "role_id").Unique(),
	}
}
