package schema

import (
	"entgo.io/ent/dialect/entsql"
	entschema "entgo.io/ent/schema"
	"entgo.io/ent/schema/mixin"
)

// databaseCommentMixin 统一启用 Ent 字段注释到数据库列注释的生成。
type databaseCommentMixin struct {
	mixin.Schema
}

// Annotations 返回数据库列注释生成选项。
func (databaseCommentMixin) Annotations() []entschema.Annotation {
	return []entschema.Annotation{
		entsql.WithComments(true),
	}
}
