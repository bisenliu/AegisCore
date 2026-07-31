package redis

import "sync/atomic"

// VersionTracker 记录本实例已成功应用的 RBAC policy revision。
type VersionTracker struct {
	applied atomic.Int64
}

// NewVersionTracker 构造 RBAC policy revision 跟踪器。
func NewVersionTracker() *VersionTracker { return &VersionTracker{} }

// Applied 返回本实例已应用的 RBAC policy revision。
func (t *VersionTracker) Applied() int64 { return t.applied.Load() }

// MarkApplied 仅记录更大的已应用 revision，较小 revision 不会导致倒退。
func (t *VersionTracker) MarkApplied(revision int64) {
	for {
		current := t.applied.Load()
		if revision <= current {
			return
		}
		if t.applied.CompareAndSwap(current, revision) {
			return
		}
	}
}
