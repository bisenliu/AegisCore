package postgres

import (
	"github.com/aegiscore/user-service/ent/predicate"
	entuser "github.com/aegiscore/user-service/ent/user"
	userapp "github.com/aegiscore/user-service/internal/features/user/app"
)

func buildListPredicates(input userapp.ListUsersInput) []predicate.User {
	// 所有列表查询先隐藏软删除用户，再应用可选业务过滤条件。
	predicates := []predicate.User{entuser.DeletedAtIsNil()}
	if input.Nickname != "" {
		predicates = append(predicates, entuser.NicknameContains(input.Nickname))
	}
	if input.Username != "" {
		predicates = append(predicates, entuser.UsernameEQ(input.Username))
	}
	if input.Status != nil {
		predicates = append(predicates, entuser.StatusEQ(int64(*input.Status)))
	}
	return predicates
}
