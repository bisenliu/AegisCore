package containers

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/aegiscore/common/runtime/resources"
)

const (
	DefaultRedisImage = "redis:7-alpine"
	defaultRedisPort  = "6379/tcp"
)

type RedisOptions struct {
	Image          string
	DB             int
	StartupTimeout time.Duration
}

type RedisContainer struct {
	ContainerID string
	Addr        string
	DB          int
}

func StartRedis(ctx context.Context, t testing.TB, opts RedisOptions) *RedisContainer {
	t.Helper()
	requireContainersEnabled(t)

	opts = normalizeRedisOptions(opts)
	startCtx, cancel := context.WithTimeout(ctx, opts.StartupTimeout)
	defer cancel()

	containerID := dockerRun(startCtx, t,
		"-d",
		"--rm",
		"-p", "127.0.0.1::6379",
		opts.Image,
	)
	t.Cleanup(func() { dockerStop(context.Background(), t, containerID) })

	host, port := dockerMappedPort(startCtx, t, containerID, defaultRedisPort)
	redisContainer := &RedisContainer{
		ContainerID: containerID,
		Addr:        host + ":" + strconv.Itoa(port),
		DB:          opts.DB,
	}

	waitForRedis(startCtx, t, redisContainer)
	return redisContainer
}

func (r RedisContainer) Options() *redis.Options {
	cfg := r.Config()
	return &redis.Options{
		Addr:         cfg.Addr,
		DB:           cfg.DB,
		DialTimeout:  cfg.Timeout,
		ReadTimeout:  cfg.Timeout,
		WriteTimeout: cfg.Timeout,
	}
}

func (r RedisContainer) Config() resources.RedisConfig {
	return resources.RedisConfig{
		Addr:    r.Addr,
		DB:      r.DB,
		Timeout: resources.DefaultRedisTimeout,
	}
}

func normalizeRedisOptions(opts RedisOptions) RedisOptions {
	if opts.Image == "" {
		opts.Image = DefaultRedisImage
	}
	if opts.StartupTimeout <= 0 {
		opts.StartupTimeout = DefaultStartupTimeout
	}
	return opts
}

func waitForRedis(ctx context.Context, t testing.TB, redisContainer *RedisContainer) {
	t.Helper()
	client := redis.NewClient(redisContainer.Options())
	defer func() { _ = client.Close() }()

	waitFor(ctx, t, "Redis ping", func(ctx context.Context) error {
		return client.Ping(ctx).Err()
	})
}
