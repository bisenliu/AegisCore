package schema

import (
	"entgo.io/ent"
	"github.com/aegiscore/user-services/ent/schema/userschema"
)

type User struct {
	ent.Schema
}

func (User) Fields() []ent.Field {
	return userschema.Fields()
}

func (User) Indexes() []ent.Index {
	return userschema.Indexes()
}
