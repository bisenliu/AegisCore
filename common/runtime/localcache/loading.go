package localcache

import (
	"context"

	"github.com/jellydator/ttlcache/v3"
)

type flightResult[V any] struct {
	value V
}

func (c *LoadingCache[V]) load(ctx context.Context, key string) (V, error) {
	result := c.loads.DoChan(key, func() (any, error) {
		if value, ok := c.lookup(key); ok {
			return flightResult[V]{value: value}, nil
		}

		c.publishMu.Lock()
		startRevision := c.revision
		c.publishMu.Unlock()

		loadCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), c.loadTimeout)
		defer cancel()
		loaded, err := c.loader(loadCtx, key)
		if err != nil {
			c.loadError.Add(1)
			return nil, err
		}
		c.loadSuccess.Add(1)

		c.publishMu.Lock()
		defer c.publishMu.Unlock()
		if c.revision != startRevision {
			return nil, errLoadInvalidated
		}
		c.publish(key, loaded)
		return flightResult[V]{value: loaded}, nil
	})

	var zero V
	select {
	case <-ctx.Done():
		return zero, ctx.Err()
	case loaded := <-result:
		if loaded.Err != nil {
			return zero, loaded.Err
		}
		return loaded.Val.(flightResult[V]).value, nil
	}
}

// publish 必须在持有 publishMu 时调用；所有写入串行化后，容量判定和 Set 之间不会出现写竞态。
func (c *LoadingCache[V]) publish(key string, value V) {
	c.client.DeleteExpired()
	if uint64(c.client.Len()) >= c.capacity {
		c.capacityEvictions.Add(1)
	}
	c.client.Set(key, value, ttlcache.DefaultTTL)
}
