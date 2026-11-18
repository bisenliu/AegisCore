package permission

import (
	"github.com/prometheus/client_golang/prometheus"

	commonmetrics "github.com/aegiscore/common/runtime/observability/metrics"
	permissioncasbin "github.com/aegiscore/user-service/internal/features/permission/infrastructure/casbin"
)

const (
	casbinReloadsMetricName    = "aegiscore_casbin_policy_reloads_total"
	casbinReloadLastMetricName = "aegiscore_casbin_policy_reload_last_success"
	casbinReloadsMetricHelp    = "Total number of Casbin policy reload results by status."
	casbinReloadLastMetricHelp = "Whether the latest Casbin policy reload succeeded."
)

type casbinPolicyReloadMetrics struct {
	reloads    *prometheus.CounterVec
	lastStatus prometheus.Gauge
}

// newCasbinPolicyReloadMetrics 创建用于统计权限规则加载成功率的监控指标。
func newCasbinPolicyReloadMetrics(provider *commonmetrics.Provider) permissioncasbin.ReloadMetrics {
	if provider == nil || !provider.Enabled() {
		return permissioncasbin.NopReloadMetrics()
	}
	recorder := &casbinPolicyReloadMetrics{
		reloads: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: casbinReloadsMetricName,
			Help: casbinReloadsMetricHelp,
		}, []string{commonmetrics.LabelStatus}),
		lastStatus: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: casbinReloadLastMetricName,
			Help: casbinReloadLastMetricHelp,
		}),
	}
	provider.MustRegister(recorder.reloads)
	provider.MustRegister(recorder.lastStatus)
	return recorder
}

// ReloadSucceeded 记录权限规则加载成功的次数。
func (m *casbinPolicyReloadMetrics) ReloadSucceeded() {
	m.reloads.WithLabelValues(commonmetrics.StatusSuccess).Inc()
}

// ReloadFailed 记录权限规则加载失败的次数。
func (m *casbinPolicyReloadMetrics) ReloadFailed() {
	m.reloads.WithLabelValues(commonmetrics.StatusFailure).Inc()
}

// SetLastStatus 更新最近一次权限加载状态是正常还是异常。
func (m *casbinPolicyReloadMetrics) SetLastStatus(success bool) {
	if success {
		m.lastStatus.Set(1)
		return
	}
	m.lastStatus.Set(0)
}
