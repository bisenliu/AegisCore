package datastore

import (
	"errors"
	"fmt"
	"strings"

	redisotel "github.com/redis/go-redis/extra/redisotel/v9"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"

	"github.com/aegiscore/platform/runtime/resources"
)

// RedisClientOption 配置 Redis 客户端的共享运行时能力。
type RedisClientOption func(*redisClientOptions)

type redisInstrumenter func(redis.UniversalClient, trace.TracerProvider) error

type redisClientOptions struct {
	tracerProvider trace.TracerProvider
	instrument     redisInstrumenter
}

// defaultRedisInstrumenter 为缓存客户端开启操作追踪，自动记录每条缓存读写命令的耗时与执行状态。
func defaultRedisInstrumenter(client redis.UniversalClient, provider trace.TracerProvider) error {
	return redisotel.InstrumentTracing(client,
		redisotel.WithTracerProvider(provider),
		redisotel.WithCommandFilter(omitRedisCommandTrace),
	)

}

// omitRedisCommandTrace 判断该缓存操作是否需要跳过调用链记录；例如健康心跳探测命令（PING）过于频繁，跳过记录可防止监控数据被海量无用信息淹没。
func omitRedisCommandTrace(cmd redis.Cmder) bool {
	if cmd == nil {
		return true
	}
	return redisotel.DefaultCommandFilter(cmd) || strings.EqualFold(cmd.Name(), "ping")
}

// newRedisClientOptions 提供缓存客户端的默认设置，包括默认启用操作耗时追踪。
func newRedisClientOptions() redisClientOptions {
	return redisClientOptions{tracerProvider: otel.GetTracerProvider(), instrument: defaultRedisInstrumenter}
}

// WithRedisTracerProvider 自定义用于记录 Redis 操作耗时与调用链路的追踪提供器。
func WithRedisTracerProvider(provider trace.TracerProvider) RedisClientOption {
	return func(opts *redisClientOptions) {
		if provider != nil {
			opts.tracerProvider = provider
		}
	}
}

// OpenRedisClient 根据配置创建 Redis 客户端连接池对象（此方法仅初始化连接参数，不会向服务器发送网络探测命令）。
func OpenRedisClient(redisCfg resources.RedisConfig, options ...RedisClientOption) (redis.UniversalClient, error) {
	redisCfg.ApplyDefaults()
	return openRedisClient(redisCfg, options...)
}

// openRedisClient 根据配置创建缓存连接客户端并挂载追踪功能；如果连接地址格式不正确或服务模式不支持则返回错误。
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

// newRedisClient 根据配置参数判断是连接单台缓存服务器还是连接分布式缓存集群，并创建对应的连接客户端。
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
		// 因 Redis 集群版仅支持 0 号数据库，单机版也统一固定使用 0 号库，确保无论采用单机还是集群部署，数据存放位置与隔离逻辑保持一致。
		DB:           0,
		DialTimeout:  redisCfg.Timeout,
		ReadTimeout:  redisCfg.Timeout,
		WriteTimeout: redisCfg.Timeout,
	})
}
