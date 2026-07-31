package redis

import (
	"context"
	"fmt"
	"os"

	rediscmd "github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/aegiscore/common/runtime/logger"
	permissionapplication "github.com/aegiscore/user-service/internal/features/permission/application"
)

// Store 负责 RBAC policy Redis revision 缓存和 Pub/Sub 通知。
type Store struct {
	client     policyRedisClient
	instanceID string
	keys       KeyCatalog
	log        *zap.Logger
}

type policyRedisClient interface {
	Eval(ctx context.Context, script string, keys []string, args ...any) *rediscmd.Cmd
	Publish(ctx context.Context, channel string, message any) *rediscmd.IntCmd
	Get(ctx context.Context, key string) *rediscmd.StringCmd
	Subscribe(ctx context.Context, channels ...string) *rediscmd.PubSub
}

const cachePolicyRevisionScript = `
local current = redis.call("GET", KEYS[1])
local supplied = ARGV[1]
if not current or string.len(current) < string.len(supplied) or
    (string.len(current) == string.len(supplied) and current < supplied) then
  redis.call("SET", KEYS[1], supplied)
end
return 1
`

type policySubscriber interface {
	Receive(ctx context.Context) (interface{}, error)
	Channel(opts ...rediscmd.ChannelOption) <-chan *rediscmd.Message
	Close() error
}

type policySubscriptionStore interface {
	CurrentVersion(ctx context.Context) (int64, error)
	Subscribe(ctx context.Context) policySubscriber
}

// NewStore 构造 RBAC policy Redis store。
func NewStore(client rediscmd.UniversalClient, appName string, log *zap.Logger) (*Store, error) {
	keys, err := NewKeyCatalog(appName)
	if err != nil {
		return nil, fmt.Errorf("new rbac policy redis keys: %w", err)
	}
	return newStore(client, keys, defaultInstanceID(), log), nil
}

func newStore(client policyRedisClient, keys KeyCatalog, instanceID string, log *zap.Logger) *Store {
	if instanceID == "" {
		instanceID = defaultInstanceID()
	}
	return &Store{client: client, instanceID: instanceID, keys: keys, log: log}
}

// PublishPolicyChanged 原子缓存不小于现值的数据库 revision，并使用同一 revision 发布刷新消息。
func (s *Store) PublishPolicyChanged(ctx context.Context, revision int64, change permissionapplication.PolicyChange) error {
	reason := change.ReasonText()
	if err := s.client.Eval(ctx, cachePolicyRevisionScript, []string{s.keys.PolicyVersionKey()}, revision).Err(); err != nil {
		logger.Error(ctx, "rbac policy revision cache failed", logger.StackTrace(zap.Int64("policy_revision", revision), zap.String("instance_id", s.instanceID), zap.String("reason", reason), zap.Error(err))...)
		return fmt.Errorf("cache rbac policy revision: %w", err)
	}
	logger.Info(ctx, "rbac policy revision cached", zap.Int64("policy_revision", revision), zap.String("instance_id", s.instanceID), zap.String("reason", reason))
	payload, err := encodePolicyRefreshMessage(newPolicyRefreshMessage(revision, s.instanceID, change))
	if err != nil {
		return err
	}
	if err := s.client.Publish(ctx, s.keys.PolicyChannel(), payload).Err(); err != nil {
		logger.Error(ctx, "rbac policy refresh publish failed", logger.StackTrace(zap.Int64("policy_revision", revision), zap.String("instance_id", s.instanceID), zap.String("reason", reason), zap.Error(err))...)
		return fmt.Errorf("publish rbac policy refresh: %w", err)
	}
	logger.Info(ctx, "rbac policy refresh published", zap.Int64("policy_revision", revision), zap.String("instance_id", s.instanceID), zap.String("reason", reason))
	return nil
}

// CurrentVersion 读取 Redis 中缓存的最新 RBAC policy revision。
func (s *Store) CurrentVersion(ctx context.Context) (int64, error) {
	version, err := s.client.Get(ctx, s.keys.PolicyVersionKey()).Int64()
	if err == nil {
		return version, nil
	}
	if err == rediscmd.Nil {
		return 0, nil
	}
	return 0, fmt.Errorf("read rbac policy version: %w", err)
}

// Subscribe 订阅 RBAC policy 刷新 channel。
func (s *Store) Subscribe(ctx context.Context) policySubscriber {
	return s.client.Subscribe(ctx, s.keys.PolicyChannel())
}

func defaultInstanceID() string {
	hostname, err := os.Hostname()
	if err != nil || hostname == "" {
		return "user-service"
	}
	return hostname
}
