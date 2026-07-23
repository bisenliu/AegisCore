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

type redisInstrumenter func(*redis.Client, trace.TracerProvider) error

type redisClientOptions struct {
	tracerProvider trace.TracerProvider
	instrument     redisInstrumenter
}

func defaultRedisInstrumenter(client *redis.Client, provider trace.TracerProvider) error {
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
func OpenRedisClient(redisCfg resources.RedisConfig, options ...RedisClientOption) (*redis.Client, error) {
	redisCfg.ApplyDefaults()
	return openRedisClient(redisCfg, options...)
}

func openRedisClient(redisCfg resources.RedisConfig, options ...RedisClientOption) (*redis.Client, error) {
	opts := newRedisClientOptions()
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
	if err := opts.instrument(client, opts.tracerProvider); err != nil {
		return nil, errors.Join(
			fmt.Errorf("instrument redis tracing: %w", err),
			client.Close(),
		)
	}
	return client, nil
}
