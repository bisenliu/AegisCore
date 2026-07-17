package redis

import (
	"go.uber.org/zap"

	"github.com/aegiscore/common/runtime/workerpool"
)

// NewSessionPurgePool 构造认证会话批量清理专用后台任务池。
func NewSessionPurgePool(log *zap.Logger) (*workerpool.Pool, error) {
	return workerpool.New(log, workerpool.Options{
		Name:        "auth.redis.session_purge",
		Workers:     deleteAllUserSessionsPurgeWorkers,
		StopTimeout: deleteAllUserSessionsPurgeStopTimeout,
	})
}
