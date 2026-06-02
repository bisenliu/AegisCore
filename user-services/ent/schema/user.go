package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
)

type User struct {
	ent.Schema
}

func (User) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("id").Unique().Immutable().Comment("用户ID"),
		field.String("name").NotEmpty().MaxLen(128).Comment("用户名"),
		field.String("email").NotEmpty().Unique().MaxLen(255).Comment("邮箱"),
		field.String("password").NotEmpty().Comment("密码"),
		field.Int64("token_version").Default(1).Comment("认证令牌版本"),
		field.Bool("active").Default(true).Comment("是否启用"),
		field.Int64("created_at").DefaultFunc(func() int64 { return time.Now().UnixMilli() }).Immutable().Comment("创建时间戳毫秒"),
		field.Int64("updated_at").DefaultFunc(func() int64 { return time.Now().UnixMilli() }).UpdateDefault(func() int64 { return time.Now().UnixMilli() }).Comment("更新时间戳毫秒"),
	}
}
