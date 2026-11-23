package redis

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	authapplication "github.com/aegiscore/user-service/internal/features/auth/application"
)

const accessTokenDeniedValue = "1"

// boundedAccessTokenDenylist 把拒绝名单保留时间限制在 access token 配置有效期内。
// 时钟偏差或异常过期时间不能让一条吊销记录比 token 本身活得更久。
type boundedAccessTokenDenylist struct {
	inner  authapplication.AccessTokenDenylist
	maxTTL time.Duration
}

// NewBoundedAccessTokenDenylist 包装拒绝名单，并强制 TTL 不超过 access token 配置有效期。
func NewBoundedAccessTokenDenylist(inner authapplication.AccessTokenDenylist, maxTTL time.Duration) authapplication.AccessTokenDenylist {
	return &boundedAccessTokenDenylist{inner: inner, maxTTL: maxTTL}
}

// DenyAccessToken 在写入前截断超过配置有效期的剩余时间。
func (s *boundedAccessTokenDenylist) DenyAccessToken(ctx context.Context, tokenID string, ttl time.Duration) error {
	if s == nil || s.inner == nil {
		return fmt.Errorf("access token denylist is not configured")
	}
	if s.maxTTL > 0 && ttl > s.maxTTL {
		ttl = s.maxTTL
	}
	return s.inner.DenyAccessToken(ctx, tokenID, ttl)
}

// AccessTokenDenied 原样转发查询，不改变 fail-closed 语义。
func (s *boundedAccessTokenDenylist) AccessTokenDenied(ctx context.Context, tokenID string) (bool, error) {
	if s == nil || s.inner == nil {
		return false, fmt.Errorf("access token denylist is not configured")
	}
	return s.inner.AccessTokenDenied(ctx, tokenID)
}

// DenyAccessToken 将 token 摘要写入拒绝名单，并只保留到该 token 自然过期。
func (s *SessionStore) DenyAccessToken(ctx context.Context, tokenID string, ttl time.Duration) error {
	if s == nil || s.redis == nil {
		return fmt.Errorf("access token denylist is not configured")
	}
	if tokenID == "" || ttl <= 0 {
		return fmt.Errorf("access token denylist requires token id and positive ttl")
	}
	if err := s.redis.Set(ctx, s.accessDenylistKey(tokenID), accessTokenDeniedValue, ttl).Err(); err != nil {
		return fmt.Errorf("deny access token: %w", err)
	}
	return nil
}

// AccessTokenDenied 判断 token 摘要是否仍在拒绝名单中。
// Redis 不可用时返回错误，调用方必须拒绝访问，不能把故障当成未吊销。
func (s *SessionStore) AccessTokenDenied(ctx context.Context, tokenID string) (bool, error) {
	if s == nil || s.redis == nil {
		return false, fmt.Errorf("access token denylist is not configured")
	}
	if tokenID == "" {
		return false, fmt.Errorf("access token denylist requires token id")
	}
	exists, err := s.redis.Exists(ctx, s.accessDenylistKey(tokenID)).Result()
	if err != nil {
		return false, fmt.Errorf("check access token denylist: %w", err)
	}
	return exists > 0, nil
}

// accessDenylistKey 使用 jti 摘要构造 key，避免原始 jti 出现在 Redis key 或错误文本中。
func (s *SessionStore) accessDenylistKey(tokenID string) string {
	sum := sha256.Sum256([]byte(tokenID))
	return s.keys.AuthAccessDenylist(hex.EncodeToString(sum[:]))
}
