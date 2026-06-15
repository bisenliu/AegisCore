package redis

import "sync/atomic"

// VersionTracker 记录本实例已成功应用的 RBAC policy 版本。
type VersionTracker struct {
	applied atomic.Int64
}

// NewVersionTracker 构造 RBAC policy 版本跟踪器。
func NewVersionTracker() *VersionTracker { return &VersionTracker{} }

// Applied 返回本实例已应用的 RBAC policy 版本。
func (t *VersionTracker) Applied() int64 { return t.applied.Load() }

// MarkApplied 在版本更新时记录更大的已应用版本。
func (t *VersionTracker) MarkApplied(version int64) {
	for {
		current := t.applied.Load()
		if version <= current {
			return
		}
		if t.applied.CompareAndSwap(current, version) {
			return
		}
	}
}
