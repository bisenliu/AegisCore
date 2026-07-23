package permission

import (
	rediscmd "github.com/redis/go-redis/v9"
	"go.uber.org/fx"

	"github.com/aegiscore/user-service/ent"
	permissionapplication "github.com/aegiscore/user-service/internal/features/permission/application"
	permissionpostgres "github.com/aegiscore/user-service/internal/features/permission/infrastructure/postgres"
)

// Fx 选项

// permissionStorageOptions 组装 permission feature 直接依赖的持久化端口。
var permissionStorageOptions = fx.Options(
	fx.Provide(
		providePermissionStore,
	),
)

// Fx 参数：命名资源

// PrimaryDBParams 只消费服务级命名主库连接，避免 feature provider 直接依赖 providers 包。
type PrimaryDBParams struct {
	fx.In

	Client *ent.Client `name:"primary_db"`
}

// CacheRedisParams 只消费服务级命名缓存 Redis 连接，供 policy sync 装配复用。
type CacheRedisParams struct {
	fx.In

	Client *rediscmd.Client `name:"cache_redis"`
}

// Provider：存储适配器

func providePermissionStore(params PrimaryDBParams) permissionapplication.PermissionStore {
	return permissionpostgres.NewPermissionStore(params.Client)
}
