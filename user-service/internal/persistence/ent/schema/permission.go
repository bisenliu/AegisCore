package schema

import (
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
	return []ent.Mixin{databaseCommentMixin{}, createdAtMillisMixin{}, updatedAtMillisMixin{}}
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
		index.Fields("module", "permission_id"),
		index.Fields("http_method", "permission_id"),
	}
}
