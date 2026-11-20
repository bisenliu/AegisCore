package localcache

// Stats 是 localcache 暴露给 metrics collector 的稳定统计快照。
type Stats struct {
	Hit               uint64
	Miss              uint64
	LoadSuccess       uint64
	LoadError         uint64
	CapacityEvictions uint64
	Capacity          uint64
}

// StatsSource 定义可导出 localcache 统计快照的类型。
type StatsSource interface {
	Name() string
	Stats() Stats
}

// Stats 返回当前累计统计快照。
func (c *LoadingCache[V]) Stats() Stats {
	return Stats{
		Hit:               c.hit.Load(),
		Miss:              c.miss.Load(),
		LoadSuccess:       c.loadSuccess.Load(),
		LoadError:         c.loadError.Load(),
		CapacityEvictions: c.capacityEvictions.Load(),
		Capacity:          c.capacity,
	}
}
