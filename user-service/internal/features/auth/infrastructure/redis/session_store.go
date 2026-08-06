package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	rediscache "github.com/redis/go-redis/v9"

	runtimeconfig "github.com/aegiscore/common/runtime/config"
	runtimeid "github.com/aegiscore/common/runtime/id"
	"github.com/aegiscore/common/runtime/workerpool"
	authapplication "github.com/aegiscore/user-service/internal/features/auth/application"
)

const (
	// defaultTokenVersionCacheTTL 限制 token_version 校验依赖 Redis 缓存的最长时间。
	defaultTokenVersionCacheTTL = 5 * time.Minute
	// defaultAuthSessionTTL 是调用方未提供有效时长时的会话兜底过期时间。
	defaultAuthSessionTTL = time.Hour
	// defaultPasswordChangeSessionTTL 是调用方未提供有效时长时改密一次性会话的兜底过期时间。
	defaultPasswordChangeSessionTTL = 5 * time.Minute
	// authSessionIndexTTLBuffer 让用户会话索引在最后一个会话过期后仍保留短暂窗口用于懒清理。
	authSessionIndexTTLBuffer = 5 * time.Minute
	// deleteAllUserSessionsPurgeTTL 限制临时清理索引在后台清理失败时的最长保留时间。
	deleteAllUserSessionsPurgeTTL = time.Hour
	// deleteAllUserSessionsBatchSize 限制退出全部设备后台清理每批 Redis key 数量。
	deleteAllUserSessionsBatchSize int64 = 500
	// deleteAllUserSessionsPurgeWorkers 限制退出全部设备后台清理并发。
	deleteAllUserSessionsPurgeWorkers = 4
	// deleteAllUserSessionsPurgeStopTimeout 限制服务关闭时等待后台清理的时间，必须与 runtime lifecycle worker drain 预算一致。
	deleteAllUserSessionsPurgeStopTimeout = runtimeconfig.DefaultLifecycleWorkerDrainAllowance

	// expiredSessionMinScore 让 ZRemRangeByScore 清理所有 score 小于等于当前时间的会话。
	expiredSessionMinScore = "-inf"
)

// SessionStoreOptions 包含 Redis 认证会话 store 的普通构造依赖。
type SessionStoreOptions struct {
	Redis                rediscache.UniversalClient
	Keys                 KeyCatalog
	TokenVersionCacheTTL time.Duration
	PurgePool            PurgeTaskPool
	Metrics              authapplication.Metrics
}

// SessionStore 使用 Redis 实现 token version 投影、refresh 会话和一次性改密会话端口。
type SessionStore struct {
	redis                rediscache.UniversalClient
	keys                 KeyCatalog
	tokenVersionCacheTTL time.Duration
	purgePool            PurgeTaskPool
	metrics              authapplication.Metrics
}

var (
	_ authapplication.TokenVersionCache          = (*SessionStore)(nil)
	_ authapplication.RefreshSessionStore        = (*SessionStore)(nil)
	_ authapplication.PasswordChangeSessionStore = (*SessionStore)(nil)
)

// PurgeTaskPool 是认证 Redis 适配器消费的后台清理任务池窄接口。
type PurgeTaskPool interface {
	Submit(ctx context.Context, task workerpool.Task) error
	Stats() workerpool.Stats
}

// NewSessionStore 构造认证会话持久化的 Redis 实现。
func NewSessionStore(options SessionStoreOptions) *SessionStore {
	metrics := options.Metrics
	if metrics == nil {
		metrics = authapplication.NopMetrics()
	}
	return &SessionStore{
		redis:                options.Redis,
		keys:                 options.Keys,
		tokenVersionCacheTTL: options.TokenVersionCacheTTL,
		purgePool:            options.PurgePool,
		metrics:              metrics,
	}
}

func (r *SessionStore) metricsRecorder() authapplication.Metrics {
	if r == nil || r.metrics == nil {
		return authapplication.NopMetrics()
	}
	return r.metrics
}

func (r *SessionStore) tokenVersionKey(userID uuid.UUID) string {
	return r.keys.AuthUserTokenVersion(userID.String())
}

func (r *SessionStore) sessionKey(userID uuid.UUID, sessionID string) string {
	return r.keys.AuthSession(userID.String(), sessionID)
}

func (r *SessionStore) passwordChangeSessionKey(userID uuid.UUID, sessionID string) string {
	return r.keys.PasswordChangeSession(userID.String(), sessionID)
}

func (r *SessionStore) userSessionsKey(userID uuid.UUID) string {
	return r.keys.AuthUserSessions(userID.String())
}

func (r *SessionStore) purgeUserSessionsKey(userID uuid.UUID) (string, error) {
	purgeID, err := runtimeid.NewUUIDString()
	if err != nil {
		return "", fmt.Errorf("generate auth sessions purge id: %w", err)
	}
	return r.keys.AuthUserSessionsPurge(userID.String(), purgeID), nil
}
