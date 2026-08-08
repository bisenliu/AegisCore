package redis

import (
	"go.uber.org/zap"

	"github.com/aegiscore/common/runtime/workerpool"
)

// NewSessionPurgePool 构造认证会话批量清理专用后台任务池。
// 池容量只控制单副本后台 Redis 清理并发，不是入口限流；调整时需参考 api_rate_limit.authenticated 的 rate_per_second 和 Redis 承载能力。
func NewSessionPurgePool(log *zap.Logger) (*workerpool.Pool, error) {
	return workerpool.New(log, workerpool.Options{
		Name:        "auth.redis.session_purge",
		Workers:     deleteAllUserSessionsPurgeWorkers,
		StopTimeout: deleteAllUserSessionsPurgeStopTimeout,
	})
}
