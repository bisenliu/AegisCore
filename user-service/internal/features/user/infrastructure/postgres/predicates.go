package postgres

import (
	userapplication "github.com/aegiscore/user-service/internal/features/user/application"
	"github.com/aegiscore/user-service/internal/persistence/ent/predicate"
	entuser "github.com/aegiscore/user-service/internal/persistence/ent/user"
)

// buildListPredicates 根据用户查询请求中的筛选条件（如用户名或账号状态），拼接数据库查询过滤语句。
func buildListPredicates(input userapplication.ListUsersInput) []predicate.User {
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
