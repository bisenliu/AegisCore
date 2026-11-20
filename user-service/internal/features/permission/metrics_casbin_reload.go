package permission

import (
	"github.com/prometheus/client_golang/prometheus"

	platformmetrics "github.com/aegiscore/platform/runtime/observability/metrics"
	permissioncasbin "github.com/aegiscore/user-service/internal/features/permission/infrastructure/casbin"
)

const (
	// casbinReloadsMetricName 是 Casbin 策略重载执行结果累计 Counter 指标名。
	casbinReloadsMetricName = "aegiscore_casbin_policy_reloads_total"
	// casbinReloadLastMetricName 是最近一次策略重载是否成功的 Gauge 指标名（1 代表成功，0 代表失败）。
	casbinReloadLastMetricName = "aegiscore_casbin_policy_reload_last_success"
	// casbinReloadsMetricHelp 是策略重载执行结果指标说明。
	casbinReloadsMetricHelp = "Total number of Casbin policy reload results by status."
	// casbinReloadLastMetricHelp 是最近一次重载状态指标说明。
	casbinReloadLastMetricHelp = "Whether the latest Casbin policy reload succeeded."
)

type casbinPolicyReloadMetrics struct {
	reloads    *prometheus.CounterVec
	lastStatus prometheus.Gauge
}

// newCasbinPolicyReloadMetrics 创建用于统计权限规则加载成功率的监控指标。
func newCasbinPolicyReloadMetrics(provider *platformmetrics.Provider) permissioncasbin.ReloadMetrics {
	if provider == nil || !provider.Enabled() {
		return permissioncasbin.NopReloadMetrics()
	}
	recorder := &casbinPolicyReloadMetrics{
		reloads: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: casbinReloadsMetricName,
			Help: casbinReloadsMetricHelp,
		}, []string{platformmetrics.LabelStatus}),
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
	m.reloads.WithLabelValues(platformmetrics.StatusSuccess).Inc()
}

// ReloadFailed 记录权限规则加载失败的次数。
func (m *casbinPolicyReloadMetrics) ReloadFailed() {
	m.reloads.WithLabelValues(platformmetrics.StatusFailure).Inc()
}

// SetLastStatus 更新最近一次权限加载状态是正常还是异常。
func (m *casbinPolicyReloadMetrics) SetLastStatus(success bool) {
	if success {
		m.lastStatus.Set(1)
		return
	}
	m.lastStatus.Set(0)
}
