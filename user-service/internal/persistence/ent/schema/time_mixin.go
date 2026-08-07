package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/mixin"
)

// createdAtMillisMixin 统一声明毫秒级创建时间字段。
type createdAtMillisMixin struct {
	mixin.Schema
}

// Fields 返回创建时间字段定义。
func (createdAtMillisMixin) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("created_at").DefaultFunc(func() int64 { return time.Now().UnixMilli() }).Immutable().Comment("创建时间戳毫秒"),
	}
}

// updatedAtMillisMixin 统一声明毫秒级更新时间字段。
type updatedAtMillisMixin struct {
	mixin.Schema
}

// Fields 返回更新时间字段定义。
func (updatedAtMillisMixin) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("updated_at").DefaultFunc(func() int64 { return time.Now().UnixMilli() }).UpdateDefault(func() int64 { return time.Now().UnixMilli() }).Comment("更新时间戳毫秒"),
	}
}
