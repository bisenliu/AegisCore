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

// cachePolicyRevisionScript 是将单调递增的策略版本号原子缓存到 Redis 的 Lua 脚本。
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

// newStore 创建负责在缓存中广播和接收权限变更通知的组件。
func newStore(client policyRedisClient, keys KeyCatalog, instanceID string, log *zap.Logger) *Store {
	if instanceID == "" {
		instanceID = defaultInstanceID()
	}
	return &Store{client: client, instanceID: instanceID, keys: keys, log: log}
}

// PublishPolicyChanged 原子缓存不小于现值的数据库 policy revision，并发布可丢失的刷新 hint。
func (s *Store) PublishPolicyChanged(ctx context.Context, revision int64, change permissionapplication.PolicyChange) error {
	return s.publish(ctx, revision, change, true)
}

// PublishUserRoleChanged 发布可丢失的用户角色缓存失效 hint，不写入 policy revision 缓存。
func (s *Store) PublishUserRoleChanged(ctx context.Context, revision int64, change permissionapplication.PolicyChange) error {
	return s.publish(ctx, revision, change, false)
}

// publish 将权限变更通知广播到缓存频道中，提醒其他服务节点拉取最新规则。
func (s *Store) publish(ctx context.Context, revision int64, change permissionapplication.PolicyChange, cachePolicyRevision bool) error {
	reason := change.ReasonText()
	payload, err := encodePolicyRefreshMessage(newPolicyRefreshMessage(revision, change, s.instanceID))
	if err != nil {
		return err
	}
	if cachePolicyRevision {
		if err := s.client.Eval(ctx, cachePolicyRevisionScript, []string{s.keys.PolicyVersionKey()}, revision).Err(); err != nil {
			logger.Warn(ctx, "rbac policy revision cache failed", zap.Int64("policy_revision", revision), zap.String("instance_id", s.instanceID), zap.String("reason", reason), zap.Error(err))
			return fmt.Errorf("cache rbac policy revision: %w", err)
		}
		logger.Info(ctx, "rbac policy revision cached", zap.Int64("policy_revision", revision), zap.String("instance_id", s.instanceID), zap.String("reason", reason))
	}
	if err := s.client.Publish(ctx, s.keys.PolicyChannel(), payload).Err(); err != nil {
		logger.Warn(ctx, "rbac policy refresh publish failed", append(publicationLogFields(revision, change), zap.String("instance_id", s.instanceID), zap.Error(err))...)
		return fmt.Errorf("publish rbac policy refresh: %w", err)
	}
	logger.Info(ctx, "rbac policy refresh published", append(publicationLogFields(revision, change), zap.String("instance_id", s.instanceID))...)
	return nil
}

// publicationLogFields 整理广播日志的输出项，杜绝在日志中打印用户的敏感凭据。
func publicationLogFields(revision int64, change permissionapplication.PolicyChange) []zap.Field {
	if change.Kind == permissionapplication.PolicyChangeKindUserRole {
		return []zap.Field{zap.Int64("user_role_revision", revision), zap.String("reason", change.ReasonText())}
	}
	return []zap.Field{zap.Int64("policy_revision", revision), zap.String("reason", change.ReasonText())}
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

// defaultInstanceID 若未配置机器编号则自动生成一个，便于在广播时区分消息是由哪台服务器发出的。
func defaultInstanceID() string {
	hostname, err := os.Hostname()
	if err != nil || hostname == "" {
		return "user-service"
	}
	return hostname
}
