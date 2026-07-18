package permission

import (
	"github.com/gin-gonic/gin"
	rediscmd "github.com/redis/go-redis/v9"
	"go.uber.org/fx"

	"github.com/aegiscore/user-service/ent"
	permissionapplication "github.com/aegiscore/user-service/internal/features/permission/application"
	permissionpostgres "github.com/aegiscore/user-service/internal/features/permission/infrastructure/postgres"
	permissionhttp "github.com/aegiscore/user-service/internal/features/permission/transport/http"
)

// permissionStorageOptions 组装 permission feature 直接依赖的持久化与路由目录扫描端口。
var permissionStorageOptions = fx.Options(
	fx.Provide(
		providePermissionStore,
		provideRouteCatalogScanner,
	),
)

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

func providePermissionStore(params PrimaryDBParams) permissionapplication.PermissionStore {
	return permissionpostgres.NewPermissionStore(params.Client)
}

func provideRouteCatalogScanner(engine *gin.Engine) permissionapplication.RouteCatalogScanner {
	return permissionhttp.NewRouteCatalogScanner(engine)
}
