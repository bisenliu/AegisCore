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

func (nopReloadMetrics) ReloadSucceeded()   {}
func (nopReloadMetrics) ReloadFailed()      {}
func (nopReloadMetrics) SetLastStatus(bool) {}
