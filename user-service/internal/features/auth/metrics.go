package auth

import (
	"context"
	"fmt"

	"github.com/prometheus/client_golang/prometheus"

	commonmetrics "github.com/aegiscore/common/runtime/observability/metrics"
	authapplication "github.com/aegiscore/user-service/internal/features/auth/application"
)

const (
	authOperationsMetricName                 = "aegiscore_user_service_auth_operations_total"
	authTokenVersionMismatchMetricName       = "aegiscore_user_service_auth_token_version_mismatches_total" // #nosec G101 -- 指标名称，不包含真实凭据。
	authSessionPurgeSubmitMetricName         = "aegiscore_user_service_auth_session_purge_submit_failures_total"
	authPasswordChangeConsumeMetricName      = "aegiscore_user_service_auth_password_change_session_consume_failures_total"         // #nosec G101 -- 指标名称，不包含真实凭据。
	authPasswordChangeReuseMetricName        = "aegiscore_user_service_auth_password_change_session_reuse_rejections_total"         // #nosec G101 -- 指标名称，不包含真实凭据。
	authPasswordChangeRevocationMetricName   = "aegiscore_user_service_auth_password_change_revocation_projection_failures_total"   // #nosec G101 -- 指标名称，不包含真实凭据。
	authPasswordChangeCompensationMetricName = "aegiscore_user_service_auth_password_change_revocation_compensation_failures_total" // #nosec G101 -- 指标名称，不包含真实凭据。
	authOperationsMetricHelp                 = "Total number of auth operation results by fixed operation, result, and reason."
	authTokenVersionMismatchMetricHelp       = "Total number of auth token version mismatches by fixed token source."
	authSessionPurgeSubmitMetricHelp         = "Total number of auth session purge task submission failures."
	authPasswordChangeConsumeMetricHelp      = "Total number of password-change one-time session consume failures by fixed reason."
	authPasswordChangeReuseMetricHelp        = "Total number of password-change one-time session reuse rejections."
	authPasswordChangeRevocationMetricHelp   = "Total number of password-change revocation projection failures by fixed step."
	authPasswordChangeCompensationMetricHelp = "Total number of password-change revocation compensation failures by fixed reason."
)

type prometheusMetrics struct {
	operations                         *prometheus.CounterVec
	tokenVersionMismatches             *prometheus.CounterVec
	sessionPurgeFailures               prometheus.Counter
	passwordChangeConsumeFailures      *prometheus.CounterVec
	passwordChangeReuseRejections      prometheus.Counter
	passwordChangeRevocationFailures   *prometheus.CounterVec
	passwordChangeCompensationFailures *prometheus.CounterVec
}

func newAuthMetrics(provider *commonmetrics.Provider) (authapplication.Metrics, error) {
	if provider == nil || !provider.Enabled() {
		return authapplication.NopMetrics(), nil
	}
	recorder := &prometheusMetrics{
		operations: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: authOperationsMetricName,
			Help: authOperationsMetricHelp,
		}, []string{"operation", "result", "reason"}),
		tokenVersionMismatches: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: authTokenVersionMismatchMetricName,
			Help: authTokenVersionMismatchMetricHelp,
		}, []string{"source"}),
		sessionPurgeFailures: prometheus.NewCounter(prometheus.CounterOpts{
			Name: authSessionPurgeSubmitMetricName,
			Help: authSessionPurgeSubmitMetricHelp,
		}),
		passwordChangeConsumeFailures:      prometheus.NewCounterVec(prometheus.CounterOpts{Name: authPasswordChangeConsumeMetricName, Help: authPasswordChangeConsumeMetricHelp}, []string{"reason"}),
		passwordChangeReuseRejections:      prometheus.NewCounter(prometheus.CounterOpts{Name: authPasswordChangeReuseMetricName, Help: authPasswordChangeReuseMetricHelp}),
		passwordChangeRevocationFailures:   prometheus.NewCounterVec(prometheus.CounterOpts{Name: authPasswordChangeRevocationMetricName, Help: authPasswordChangeRevocationMetricHelp}, []string{"step"}),
		passwordChangeCompensationFailures: prometheus.NewCounterVec(prometheus.CounterOpts{Name: authPasswordChangeCompensationMetricName, Help: authPasswordChangeCompensationMetricHelp}, []string{"reason"}),
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
	if err := provider.Register(recorder.passwordChangeConsumeFailures); err != nil {
		return nil, fmt.Errorf("register auth password change consume metrics: %w", err)
	}
	if err := provider.Register(recorder.passwordChangeReuseRejections); err != nil {
		return nil, fmt.Errorf("register auth password change reuse metrics: %w", err)
	}
	if err := provider.Register(recorder.passwordChangeRevocationFailures); err != nil {
		return nil, fmt.Errorf("register auth password change revocation metrics: %w", err)
	}
	if err := provider.Register(recorder.passwordChangeCompensationFailures); err != nil {
		return nil, fmt.Errorf("register auth password change compensation metrics: %w", err)
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

func (m *prometheusMetrics) PasswordChangeSessionConsumeFailed(_ context.Context, reason string) {
	m.passwordChangeConsumeFailures.WithLabelValues(authPasswordChangeReason(reason)).Inc()
}

func (m *prometheusMetrics) PasswordChangeSessionReuseRejected(context.Context) {
	m.passwordChangeReuseRejections.Inc()
}

func (m *prometheusMetrics) PasswordChangeRevocationProjectionFailed(_ context.Context, step string) {
	m.passwordChangeRevocationFailures.WithLabelValues(authPasswordChangeRevocationStep(step)).Inc()
}

func (m *prometheusMetrics) PasswordChangeRevocationCompensationFailed(_ context.Context, reason string) {
	m.passwordChangeCompensationFailures.WithLabelValues(authPasswordChangeReason(reason)).Inc()
}

func authLogoutOperation(operation string) string {
	switch operation {
	case authapplication.MetricsOperationLogoutCurrent, authapplication.MetricsOperationLogoutAll:
		return operation
	default:
		return authapplication.MetricsOperationLogoutCurrent
	}
}

func authPasswordChangeReason(reason string) string {
	switch reason {
	case authapplication.MetricsPasswordChangeReasonNotFound,
		authapplication.MetricsPasswordChangeReasonMismatch,
		authapplication.MetricsPasswordChangeReasonSystemError:
		return reason
	default:
		return authapplication.MetricsPasswordChangeReasonSystemError
	}
}

func authPasswordChangeRevocationStep(step string) string {
	if step == authapplication.MetricsPasswordChangeRevocationProjection {
		return step
	}
	return authapplication.MetricsPasswordChangeRevocationProjection
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
