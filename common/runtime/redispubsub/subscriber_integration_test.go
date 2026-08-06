package redispubsub_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/aegiscore/common/runtime/redispubsub"
	"github.com/aegiscore/common/testing/containers"
)

type trackingClusterClient struct {
	*redis.ClusterClient
	closeCalls atomic.Int64
}

func (c *trackingClusterClient) Close() error {
	c.closeCalls.Add(1)
	return c.ClusterClient.Close()
}

func TestSubscriberReceivesClassicPubSubFromRedisCluster(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	fixture := containers.StartRedis(ctx, t, containers.RedisOptions{})
	cfg := fixture.Config()
	client := &trackingClusterClient{ClusterClient: redis.NewClusterClient(&redis.ClusterOptions{
		Addrs:        cfg.Addrs,
		DialTimeout:  cfg.Timeout,
		ReadTimeout:  cfg.Timeout,
		WriteTimeout: cfg.Timeout,
		MaxRedirects: cfg.Cluster.MaxRedirects,
	})}
	t.Cleanup(func() {
		if client.closeCalls.Load() == 0 {
			_ = client.Close()
		}
	})

	const channel = "runtime:cluster:classic-pubsub"
	subscriber, err := redispubsub.NewSubscriber(client, zap.NewNop(), redispubsub.Options{
		Name:             "cluster-integration",
		Channel:          channel,
		BufferSize:       1,
		SubscribeTimeout: 5 * time.Second,
		BackoffInitial:   10 * time.Millisecond,
		BackoffMax:       time.Second,
	})
	require.NoError(t, err)
	require.NoError(t, subscriber.Start())
	require.Eventually(t, func() bool {
		return subscriber.Status().State == redispubsub.StateConnected
	}, 10*time.Second, 10*time.Millisecond)

	require.NoError(t, client.Publish(ctx, channel, "classic-message").Err())
	select {
	case message := <-subscriber.Messages():
		require.Equal(t, redispubsub.Message{Channel: channel, Payload: "classic-message"}, message)
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}

	require.NoError(t, subscriber.Stop(ctx))
	require.Equal(t, int64(0), client.closeCalls.Load(), "subscriber must not close the shared Redis client")
	require.NoError(t, client.Close())
}
