package containers

import (
	"context"
	"fmt"
	"strconv"
	"strings"
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
	StartupTimeout time.Duration
}

type RedisContainer struct {
	ContainerID string
	Addr        string
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
		"redis-server",
		"--cluster-enabled", "yes",
		"--cluster-config-file", "nodes.conf",
		"--appendonly", "no",
		"--protected-mode", "no",
	)
	t.Cleanup(func() { dockerStop(context.Background(), t, containerID) })

	host, port := dockerMappedPort(startCtx, t, containerID, defaultRedisPort)
	redisContainer := &RedisContainer{
		ContainerID: containerID,
		Addr:        host + ":" + strconv.Itoa(port),
	}

	configureRedisClusterAnnouncement(startCtx, t, redisContainer, host, port)
	assignRedisClusterSlots(startCtx, t, redisContainer)
	waitForRedis(startCtx, t, redisContainer)
	return redisContainer
}

func (r RedisContainer) Options() *redis.Options {
	return &redis.Options{
		Addr:         r.Addr,
		DialTimeout:  resources.DefaultRedisTimeout,
		ReadTimeout:  resources.DefaultRedisTimeout,
		WriteTimeout: resources.DefaultRedisTimeout,
	}
}

func (r RedisContainer) Config() resources.RedisConfig {
	return resources.RedisConfig{
		Mode:    resources.RedisModeCluster,
		Addrs:   []string{r.Addr},
		Timeout: resources.DefaultRedisTimeout,
		Cluster: resources.RedisClusterConfig{MaxRedirects: resources.DefaultRedisClusterMaxRedirects},
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

func configureRedisClusterAnnouncement(
	ctx context.Context,
	t testing.TB,
	redisContainer *RedisContainer,
	host string,
	port int,
) {
	t.Helper()
	settings := [][2]string{
		{"cluster-announce-ip", host},
		{"cluster-announce-port", strconv.Itoa(port)},
	}
	waitFor(ctx, t, "Redis cluster announcement", func(ctx context.Context) error {
		for _, setting := range settings {
			out, stderr, err := dockerOutput(
				ctx,
				"exec", redisContainer.ContainerID,
				"redis-cli", "CONFIG", "SET", setting[0], setting[1],
			)
			if err != nil {
				return fmt.Errorf("configure %s: %w: %s", setting[0], err, strings.TrimSpace(stderr+out))
			}
			if strings.TrimSpace(out) != "OK" {
				return fmt.Errorf("configure %s: unexpected response %q", setting[0], strings.TrimSpace(out))
			}
		}
		return nil
	})
}

func assignRedisClusterSlots(ctx context.Context, t testing.TB, redisContainer *RedisContainer) {
	t.Helper()
	waitFor(ctx, t, "Redis cluster slots", func(context.Context) error {
		out, stderr, err := dockerOutput(ctx, "exec", redisContainer.ContainerID, "sh", "-c", "redis-cli cluster addslots $(seq 0 16383)")
		if err != nil && strings.Contains(out+stderr, "Slot") && strings.Contains(out+stderr, "is already busy") {
			return nil
		}
		if err != nil {
			return fmt.Errorf("assign Redis cluster slots: %w: %s", err, strings.TrimSpace(stderr+out))
		}
		return nil
	})
}
