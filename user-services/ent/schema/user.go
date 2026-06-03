package schema

import (
	"entgo.io/ent"
	"github.com/aegiscore/user-services/ent/schema/user"
)

type User struct {
	ent.Schema
}

func (User) Fields() []ent.Field {
	return user.Fields()
}

func (User) Indexes() []ent.Index {
	return user.Indexes()
}
