package localcache

// Invalidate 同步失效指定 key，并抑制此前开始的 loader。
func (c *LoadingCache[V]) Invalidate(key string) {
	c.publishMu.Lock()
	defer c.publishMu.Unlock()
	c.revision++
	c.client.Delete(key)
}

// InvalidateAll 同步失效全部 item，并抑制此前开始的 loader。
func (c *LoadingCache[V]) InvalidateAll() {
	c.publishMu.Lock()
	defer c.publishMu.Unlock()
	c.revision++
	c.client.DeleteAll()
}
