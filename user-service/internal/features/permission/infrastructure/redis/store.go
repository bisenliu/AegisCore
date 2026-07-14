package redis

import (
	"context"
	"fmt"
	"os"

	rediscmd "github.com/redis/go-redis/v9"
	"go.uber.org/fx"
	"go.uber.org/zap"

	"github.com/aegiscore/common/runtime/config"
	"github.com/aegiscore/common/runtime/logger"
	permissionapplication "github.com/aegiscore/user-service/internal/features/permission/application"
)

// StoreParams 包含 RBAC policy Redis store 所需依赖。
type StoreParams struct {
	fx.In

	Client *rediscmd.Client `name:"cache_redis"`
	Cfg    *config.Config
	Log    *zap.Logger
}

// Store 负责 RBAC policy Redis 版本和 Pub/Sub 通知。
type Store struct {
	client     *rediscmd.Client
	instanceID string
	keys       KeyCatalog
	log        *zap.Logger
}

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
func NewStore(params StoreParams) (*Store, error) {
	keys, err := NewKeyCatalog(params.Cfg.App.Name)
	if err != nil {
		return nil, fmt.Errorf("new rbac policy redis keys: %w", err)
	}
	return newStore(params.Client, keys, defaultInstanceID(), params.Log), nil
}

func newStore(client *rediscmd.Client, keys KeyCatalog, instanceID string, log *zap.Logger) *Store {
	if instanceID == "" {
		instanceID = defaultInstanceID()
	}
	return &Store{client: client, instanceID: instanceID, keys: keys, log: log}
}

// PublishPolicyChanged 递增 RBAC policy 版本并发布刷新消息。
func (s *Store) PublishPolicyChanged(ctx context.Context, change permissionapplication.PolicyChange) (int64, error) {
	reason := change.ReasonText()
	version, err := s.client.Incr(ctx, s.keys.PolicyVersionKey()).Result()
	if err != nil {
		logger.Error(ctx, "rbac policy version increment failed", logger.StackTrace(zap.String("instance_id", s.instanceID), zap.String("reason", reason), zap.Error(err))...)
		return 0, fmt.Errorf("increment rbac policy version: %w", err)
	}
	logger.Info(ctx, "rbac policy version incremented", zap.Int64("policy_version", version), zap.String("instance_id", s.instanceID), zap.String("reason", reason))
	payload, err := encodePolicyRefreshMessage(newPolicyRefreshMessage(version, s.instanceID, change))
	if err != nil {
		return 0, err
	}
	if err := s.client.Publish(ctx, s.keys.PolicyChannel(), payload).Err(); err != nil {
		logger.Error(ctx, "rbac policy refresh publish failed", logger.StackTrace(zap.Int64("policy_version", version), zap.String("instance_id", s.instanceID), zap.String("reason", reason), zap.Error(err))...)
		return 0, fmt.Errorf("publish rbac policy refresh: %w", err)
	}
	logger.Info(ctx, "rbac policy refresh published", zap.Int64("policy_version", version), zap.String("instance_id", s.instanceID), zap.String("reason", reason))
	return version, nil
}

// CurrentVersion 读取 Redis 中最新 RBAC policy 版本。
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
