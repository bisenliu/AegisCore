package casbin

// ReloadMetrics 记录 permission feature 内 Casbin policy reload 结果。
type ReloadMetrics interface {
	ReloadSucceeded()
	ReloadFailed()
	SetLastStatus(success bool)
}

type nopReloadMetrics struct{}

// NopReloadMetrics 返回 policy reload metrics 的空实现。
func NopReloadMetrics() ReloadMetrics {
	return nopReloadMetrics{}
}

// ReloadSucceeded 占位记录权限规则加载成功的事件。
func (nopReloadMetrics) ReloadSucceeded() {}

// ReloadFailed 占位记录权限规则加载失败的事件。
func (nopReloadMetrics) ReloadFailed() {}

// SetLastStatus 占位更新最近一次权限规则加载的状态标记。
func (nopReloadMetrics) SetLastStatus(bool) {}
