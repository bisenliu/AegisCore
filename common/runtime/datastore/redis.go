package datastore

import (
	redisotel "github.com/redis/go-redis/extra/redisotel/v9"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"

	"github.com/aegiscore/common/runtime/resources"
)

// RedisClientOption 配置 Redis 客户端的共享运行时能力。
type RedisClientOption func(*redisClientOptions)

type redisClientOptions struct {
	tracerProvider trace.TracerProvider
}

// WithRedisTracerProvider 显式指定 Redis instrumentation 使用的 tracer provider。
func WithRedisTracerProvider(provider trace.TracerProvider) RedisClientOption {
	return func(opts *redisClientOptions) {
		if provider != nil {
			opts.tracerProvider = provider
		}
	}
}

// OpenRedisClient 根据配置构造 Redis 客户端，但不检查连接可用性。
func OpenRedisClient(redisCfg resources.RedisConfig, options ...RedisClientOption) *redis.Client {
	redisCfg.ApplyDefaults()
	return openRedisClient(redisCfg, options...)
}

func openRedisClient(redisCfg resources.RedisConfig, options ...RedisClientOption) *redis.Client {
	opts := redisClientOptions{tracerProvider: otel.GetTracerProvider()}
	for _, option := range options {
		if option != nil {
			option(&opts)
		}
	}
	client := redis.NewClient(&redis.Options{
		Addr:         redisCfg.Addr,
		Username:     redisCfg.Username,
		Password:     redisCfg.Password,
		DB:           redisCfg.DB,
		DialTimeout:  redisCfg.Timeout,
		ReadTimeout:  redisCfg.Timeout,
		WriteTimeout: redisCfg.Timeout,
	})
	if err := redisotel.InstrumentTracing(client, redisotel.WithTracerProvider(opts.tracerProvider)); err != nil {
		panic(err)
	}
	return client
}
