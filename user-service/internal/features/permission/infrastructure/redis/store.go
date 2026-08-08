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

var _ permissionapplication.PolicyRevisionPublisher = (*Store)(nil)

type policyRedisClient interface {
	Eval(ctx context.Context, script string, keys []string, args ...any) *rediscmd.Cmd
	Publish(ctx context.Context, channel string, message any) *rediscmd.IntCmd
	Get(ctx context.Context, key string) *rediscmd.StringCmd
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

// PublishPolicyRevision 原子缓存不小于现值的数据库 revision，并发布对应 outbox event envelope。
func (s *Store) PublishPolicyRevision(ctx context.Context, event permissionapplication.OutboxEvent) error {
	reason := event.Reason
	payload, err := encodePolicyRefreshMessage(newPolicyRefreshMessage(event, s.instanceID))
	if err != nil {
		return err
	}
	if event.PolicyRevision != nil {
		if err := s.client.Eval(ctx, cachePolicyRevisionScript, []string{s.keys.PolicyVersionKey()}, *event.PolicyRevision).Err(); err != nil {
			logger.Error(ctx, "rbac policy revision cache failed", logger.StackTrace(zap.Int64("policy_revision", *event.PolicyRevision), zap.String("instance_id", s.instanceID), zap.String("reason", reason), zap.Error(err))...)
			return fmt.Errorf("cache rbac policy revision: %w", err)
		}
		logger.Info(ctx, "rbac policy revision cached", zap.Int64("policy_revision", *event.PolicyRevision), zap.String("instance_id", s.instanceID), zap.String("reason", reason))
	}
	if err := s.client.Publish(ctx, s.keys.PolicyChannel(), payload).Err(); err != nil {
		logger.Error(ctx, "rbac policy refresh publish failed", logger.StackTrace(append(eventRevisionLogFields(event), zap.String("instance_id", s.instanceID), zap.String("reason", reason), zap.Error(err))...)...)
		return fmt.Errorf("publish rbac policy refresh: %w", err)
	}
	logger.Info(ctx, "rbac policy refresh published", append(eventRevisionLogFields(event), zap.String("instance_id", s.instanceID), zap.String("reason", reason))...)
	return nil
}

func eventRevisionLogFields(event permissionapplication.OutboxEvent) []zap.Field {
	fields := make([]zap.Field, 0, 2)
	if event.PolicyRevision != nil {
		fields = append(fields, zap.Int64("policy_revision", *event.PolicyRevision))
	}
	if event.UserRoleRevision != nil {
		fields = append(fields, zap.Int64("user_role_revision", *event.UserRoleRevision))
	}
	return fields
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

// PolicyChannel 返回 RBAC policy 刷新 Pub/Sub channel，供 composition 构造通用 subscriber。
func (s *Store) PolicyChannel() string {
	return s.keys.PolicyChannel()
}

func defaultInstanceID() string {
	hostname, err := os.Hostname()
	if err != nil || hostname == "" {
		return "user-service"
	}
	return hostname
}
