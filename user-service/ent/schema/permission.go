package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// Permission 是 permissions 表的 Ent schema。
type Permission struct {
	ent.Schema
}

// Mixin 返回 permissions 表复用的公共 schema mixin。
func (Permission) Mixin() []ent.Mixin {
	return []ent.Mixin{databaseCommentMixin{}}
}

// Fields 返回权限表字段定义。
func (Permission) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("id").Unique().Immutable().Comment("权限ID"),
		field.UUID("permission_id", uuid.UUID{}).Unique().Immutable().Comment("外部权限ID"),
		field.String("name").NotEmpty().MaxLen(128).Comment("权限名称"),
		field.String("description").Default("").MaxLen(512).Comment("权限说明"),
		field.String("module").NotEmpty().MaxLen(64).Comment("权限所属模块"),
		field.String("http_method").NotEmpty().MaxLen(16).Comment("HTTP 方法"),
		field.String("path_template").NotEmpty().MaxLen(512).Comment("路径模板"),
		field.Bool("active").Default(true).Comment("权限是否启用"),
		field.Bool("is_system").Default(false).Comment("是否系统权限"),
		field.Int64("created_at").DefaultFunc(func() int64 { return time.Now().UnixMilli() }).Immutable().Comment("创建时间戳毫秒"),
		field.Int64("updated_at").DefaultFunc(func() int64 { return time.Now().UnixMilli() }).UpdateDefault(func() int64 { return time.Now().UnixMilli() }).Comment("更新时间戳毫秒"),
	}
}

// Edges 返回权限和角色权限绑定的关联定义。
func (Permission) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("role_permissions", RolePermission.Type),
	}
}

// Indexes 返回权限表唯一约束和列表过滤索引定义。
func (Permission) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("http_method", "path_template").Unique(),
		index.Fields("active", "permission_id"),
		index.Fields("module", "permission_id"),
		index.Fields("http_method", "permission_id"),
		index.Fields("is_system", "permission_id"),
	}
}
