package datastore

import (
	"errors"
	"fmt"
	"strings"

	redisotel "github.com/redis/go-redis/extra/redisotel/v9"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"

	"github.com/aegiscore/common/runtime/resources"
)

// RedisClientOption 配置 Redis 客户端的共享运行时能力。
type RedisClientOption func(*redisClientOptions)

type redisInstrumenter func(redis.UniversalClient, trace.TracerProvider) error

type redisClientOptions struct {
	tracerProvider trace.TracerProvider
	instrument     redisInstrumenter
}

func defaultRedisInstrumenter(client redis.UniversalClient, provider trace.TracerProvider) error {
	return redisotel.InstrumentTracing(client,
		redisotel.WithTracerProvider(provider),
		redisotel.WithCommandFilter(omitRedisCommandTrace),
	)

}

func omitRedisCommandTrace(cmd redis.Cmder) bool {
	if cmd == nil {
		return true
	}
	return redisotel.DefaultCommandFilter(cmd) || strings.EqualFold(cmd.Name(), "ping")
}

func newRedisClientOptions() redisClientOptions {
	return redisClientOptions{tracerProvider: otel.GetTracerProvider(), instrument: defaultRedisInstrumenter}
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
func OpenRedisClient(redisCfg resources.RedisConfig, options ...RedisClientOption) (redis.UniversalClient, error) {
	redisCfg.ApplyDefaults()
	return openRedisClient(redisCfg, options...)
}

func openRedisClient(redisCfg resources.RedisConfig, options ...RedisClientOption) (redis.UniversalClient, error) {
	opts := newRedisClientOptions()
	for _, option := range options {
		if option != nil {
			option(&opts)
		}
	}
	client := newRedisClient(redisCfg)
	if err := opts.instrument(client, opts.tracerProvider); err != nil {
		return nil, errors.Join(
			fmt.Errorf("instrument redis tracing: %w", err),
			client.Close(),
		)
	}
	return client, nil
}

func newRedisClient(redisCfg resources.RedisConfig) redis.UniversalClient {
	if redisCfg.Mode == resources.RedisModeCluster {
		return redis.NewClusterClient(&redis.ClusterOptions{
			Addrs:        append([]string(nil), redisCfg.Addrs...),
			Username:     redisCfg.Username,
			Password:     redisCfg.Password,
			DialTimeout:  redisCfg.Timeout,
			ReadTimeout:  redisCfg.Timeout,
			WriteTimeout: redisCfg.Timeout,
			MaxRedirects: redisCfg.Cluster.MaxRedirects,
		})
	}
	return redis.NewClient(&redis.Options{
		Addr:     redisCfg.Addr,
		Username: redisCfg.Username,
		Password: redisCfg.Password,
		// Redis Cluster 只支持 0 号库；standalone 也固定使用 0 号库，避免两种 mode 出现数据隔离语义分叉。
		DB:           0,
		DialTimeout:  redisCfg.Timeout,
		ReadTimeout:  redisCfg.Timeout,
		WriteTimeout: redisCfg.Timeout,
	})
}
