package datastore

import (
	"github.com/aegiscore/common/runtime/config"
	"github.com/redis/go-redis/v9"
)

func NewRedisClient(redisCfg config.RedisConfig) *redis.Client {
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
