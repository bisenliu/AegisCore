package redis

import (
	rediscache "github.com/redis/go-redis/v9"
	"go.uber.org/fx"
	"go.uber.org/zap"

	"github.com/aegiscore/common/runtime/workerpool"
)

// SessionPurgePoolParams 包含认证会话批量清理任务池所需的 Fx 输入。
type SessionPurgePoolParams struct {
	fx.In

	Lifecycle fx.Lifecycle
	Redis     *rediscache.Client `name:"cache_redis"`
	Log       *zap.Logger
}

// NewSessionPurgePool 构造认证会话批量清理专用后台任务池。
func NewSessionPurgePool(params SessionPurgePoolParams) (*workerpool.Pool, error) {
	return workerpool.New(params.Lifecycle, params.Log, workerpool.Options{
		Name:        "auth.redis.session_purge",
		Workers:     deleteAllUserSessionsPurgeWorkers,
		StopTimeout: deleteAllUserSessionsPurgeStopTimeout,
	})
}
