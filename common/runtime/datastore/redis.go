package datastore

import (
	"github.com/redis/go-redis/v9"

	"github.com/aegiscore/common/runtime/config"
)

// OpenRedisClient 根据配置构造 Redis 客户端，但不检查连接可用性。
func OpenRedisClient(redisCfg config.RedisConfig) *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr:         redisCfg.Addr,
		Username:     redisCfg.Username,
		Password:     redisCfg.Password,
		DB:           redisCfg.DB,
		DialTimeout:  redisCfg.DialTimeout,
		ReadTimeout:  redisCfg.ReadTimeout,
		WriteTimeout: redisCfg.WriteTimeout,
	})
}
