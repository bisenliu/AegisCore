package schema

import (
	"entgo.io/ent"
	"github.com/aegiscore/user-service/ent/schema/userschema"
)

// User 是 users 表的 Ent schema。
type User struct {
	ent.Schema
}

// Fields 返回委托给共享 userschema 包定义的用户表字段。
func (User) Fields() []ent.Field {
	return userschema.Fields()
}

// Indexes 返回委托给共享 userschema 包定义的用户表索引。
func (User) Indexes() []ent.Index {
	return userschema.Indexes()
}
