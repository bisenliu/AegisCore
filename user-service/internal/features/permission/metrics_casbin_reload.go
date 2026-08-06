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

func (m *casbinPolicyReloadMetrics) ReloadSucceeded() {
	m.reloads.WithLabelValues(commonmetrics.StatusSuccess).Inc()
}

func (m *casbinPolicyReloadMetrics) ReloadFailed() {
	m.reloads.WithLabelValues(commonmetrics.StatusFailure).Inc()
}

func (m *casbinPolicyReloadMetrics) SetLastStatus(success bool) {
	if success {
		m.lastStatus.Set(1)
		return
	}
	m.lastStatus.Set(0)
}
