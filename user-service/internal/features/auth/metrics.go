package auth

import (
	"context"
	"fmt"

	"github.com/prometheus/client_golang/prometheus"

	commonmetrics "github.com/aegiscore/common/runtime/observability/metrics"
	authapplication "github.com/aegiscore/user-service/internal/features/auth/application"
)

const (
	authOperationsMetricName           = "aegiscore_user_service_auth_operations_total"
	authTokenVersionMismatchMetricName = "aegiscore_user_service_auth_token_version_mismatches_total"
	authSessionPurgeSubmitMetricName   = "aegiscore_user_service_auth_session_purge_submit_failures_total"
)

type prometheusMetrics struct {
	operations             *prometheus.CounterVec
	tokenVersionMismatches *prometheus.CounterVec
	sessionPurgeFailures   prometheus.Counter
}

func newAuthMetrics(provider *commonmetrics.Provider) (authapplication.Metrics, error) {
	if provider == nil || !provider.Enabled() {
		return authapplication.NopMetrics(), nil
	}
	recorder := &prometheusMetrics{
		operations: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: authOperationsMetricName,
			Help: "Total number of auth business operation results.",
		}, []string{"operation", "result", "reason"}),
		tokenVersionMismatches: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: authTokenVersionMismatchMetricName,
			Help: "Total number of auth token version mismatches.",
		}, []string{"source"}),
		sessionPurgeFailures: prometheus.NewCounter(prometheus.CounterOpts{
			Name: authSessionPurgeSubmitMetricName,
			Help: "Total number of auth session purge submit failures.",
		}),
	}
	if err := provider.Register(recorder.operations); err != nil {
		return nil, fmt.Errorf("register auth operations metrics: %w", err)
	}
	if err := provider.Register(recorder.tokenVersionMismatches); err != nil {
		return nil, fmt.Errorf("register auth token version mismatch metrics: %w", err)
	}
	if err := provider.Register(recorder.sessionPurgeFailures); err != nil {
		return nil, fmt.Errorf("register auth session purge metrics: %w", err)
	}
	return recorder, nil
}

func (m *prometheusMetrics) LoginSucceeded(_ context.Context) {
	m.operations.WithLabelValues(authapplication.MetricsOperationLogin, commonmetrics.StatusSuccess, authapplication.MetricsReasonNone).Inc()
}

func (m *prometheusMetrics) LoginFailed(_ context.Context, reason string) {
	m.operations.WithLabelValues(authapplication.MetricsOperationLogin, commonmetrics.StatusFailure, authReason(reason)).Inc()
}

func (m *prometheusMetrics) RefreshSucceeded(_ context.Context) {
	m.operations.WithLabelValues(authapplication.MetricsOperationRefresh, commonmetrics.StatusSuccess, authapplication.MetricsReasonNone).Inc()
}

func (m *prometheusMetrics) RefreshFailed(_ context.Context, reason string) {
	m.operations.WithLabelValues(authapplication.MetricsOperationRefresh, commonmetrics.StatusFailure, authReason(reason)).Inc()
}

func (m *prometheusMetrics) LogoutSucceeded(_ context.Context, operation string) {
	m.operations.WithLabelValues(authLogoutOperation(operation), commonmetrics.StatusSuccess, authapplication.MetricsReasonNone).Inc()
}

func (m *prometheusMetrics) LogoutFailed(_ context.Context, operation string, reason string) {
	m.operations.WithLabelValues(authLogoutOperation(operation), commonmetrics.StatusFailure, authReason(reason)).Inc()
}

func (m *prometheusMetrics) TokenVersionMismatch(_ context.Context, source string) {
	m.tokenVersionMismatches.WithLabelValues(authSource(source)).Inc()
}

func (m *prometheusMetrics) SessionPurgeSubmitFailed(context.Context) {
	m.sessionPurgeFailures.Inc()
}

func authLogoutOperation(operation string) string {
	switch operation {
	case authapplication.MetricsOperationLogoutCurrent, authapplication.MetricsOperationLogoutAll:
		return operation
	default:
		return authapplication.MetricsOperationLogoutCurrent
	}
}

func authSource(source string) string {
	switch source {
	case authapplication.MetricsSourceAccessToken, authapplication.MetricsSourceRefreshToken:
		return source
	default:
		return authapplication.MetricsSourceAccessToken
	}
}

func authReason(reason string) string {
	switch reason {
	case authapplication.MetricsReasonNone,
		authapplication.MetricsReasonValidationFailed,
		authapplication.MetricsReasonCredentialInvalid,
		authapplication.MetricsReasonUserStatusRejected,
		authapplication.MetricsReasonPasswordChangeRequiredIssueFailed,
		authapplication.MetricsReasonTokenIssueFailed,
		authapplication.MetricsReasonSessionCreateFailed,
		authapplication.MetricsReasonRefreshTokenInvalid,
		authapplication.MetricsReasonRefreshTokenExpired,
		authapplication.MetricsReasonRefreshSessionInvalid,
		authapplication.MetricsReasonRefreshSessionMismatch,
		authapplication.MetricsReasonTokenVersionMismatch,
		authapplication.MetricsReasonSessionRotateFailed,
		authapplication.MetricsReasonAuthContextMissing,
		authapplication.MetricsReasonSessionDeleteFailed,
		authapplication.MetricsReasonSessionRevokeFailed,
		authapplication.MetricsReasonSystemError:
		return reason
	default:
		return authapplication.MetricsReasonSystemError
	}
}
