package auth

import (
	"context"
	"fmt"

	"github.com/prometheus/client_golang/prometheus"

	platformmetrics "github.com/aegiscore/platform/runtime/observability/metrics"
	authapplication "github.com/aegiscore/user-service/internal/features/auth/application"
)

const (
	// authOperationsMetricName 是认证核心操作（登录、改密、会话刷新等）结果计数指标名。
	authOperationsMetricName = "aegiscore_user_service_auth_operations_total"
	// authTokenVersionMismatchMetricName 是令牌版本号比对不一致计数指标名。
	authTokenVersionMismatchMetricName = "aegiscore_user_service_auth_token_version_mismatches_total" // #nosec G101 -- 指标名称，不包含真实凭据。
	// authSessionPurgeSubmitMetricName 是后台会话清理任务提交失败计数指标名。
	authSessionPurgeSubmitMetricName = "aegiscore_user_service_auth_session_purge_submit_failures_total"
	// authPasswordChangeConsumeMetricName 是强制改密一次性会话核销失败计数指标名。
	authPasswordChangeConsumeMetricName = "aegiscore_user_service_auth_password_change_session_consume_failures_total" // #nosec G101 -- 指标名称，不包含真实凭据。
	// authPasswordChangeReuseMetricName 是重复使用已被核销的改密会话被拦截计数指标名。
	authPasswordChangeReuseMetricName = "aegiscore_user_service_auth_password_change_session_reuse_rejections_total" // #nosec G101 -- 指标名称，不包含真实凭据。
	// authPasswordChangeRevocationMetricName 是改密后投影清理会话失败计数指标名。
	authPasswordChangeRevocationMetricName = "aegiscore_user_service_auth_password_change_revocation_projection_failures_total" // #nosec G101 -- 指标名称，不包含真实凭据。
	// authPasswordChangeCompensationMetricName 是改密会话补偿清理重试失败计数指标名。
	authPasswordChangeCompensationMetricName = "aegiscore_user_service_auth_password_change_revocation_compensation_failures_total" // #nosec G101 -- 指标名称，不包含真实凭据。
	// authOperationsMetricHelp 是认证核心操作指标说明。
	authOperationsMetricHelp = "Total number of auth operation results by fixed operation, result, and reason."
	// authTokenVersionMismatchMetricHelp 是令牌版本不匹配指标说明。
	authTokenVersionMismatchMetricHelp = "Total number of auth token version mismatches by fixed token source."
	// authSessionPurgeSubmitMetricHelp 是清理任务提交失败指标说明。
	authSessionPurgeSubmitMetricHelp = "Total number of auth session purge task submission failures."
	// authPasswordChangeConsumeMetricHelp 是改密会话核销失败指标说明。
	authPasswordChangeConsumeMetricHelp = "Total number of password-change one-time session consume failures by fixed reason."
	// authPasswordChangeReuseMetricHelp 是改密会话重放拦截指标说明。
	authPasswordChangeReuseMetricHelp = "Total number of password-change one-time session reuse rejections."
	// authPasswordChangeRevocationMetricHelp 是改密撤销投影失败指标说明。
	authPasswordChangeRevocationMetricHelp = "Total number of password-change revocation projection failures by fixed step."
	// authPasswordChangeCompensationMetricHelp 是改密补偿失败指标说明。
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

// newAuthMetrics 创建用于统计用户登录、凭证续期、退出等操作成功和失败频次的监控指标。
func newAuthMetrics(provider *platformmetrics.Provider) (authapplication.Metrics, error) {
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

// LoginSucceeded 记录用户成功登录的次数。
func (m *prometheusMetrics) LoginSucceeded(_ context.Context) {
	m.operations.WithLabelValues(authapplication.MetricsOperationLogin, platformmetrics.StatusSuccess, authapplication.MetricsReasonNone).Inc()
}

// LoginFailed 记录用户登录失败的次数及具体失败原因。
func (m *prometheusMetrics) LoginFailed(_ context.Context, reason string) {
	m.operations.WithLabelValues(authapplication.MetricsOperationLogin, platformmetrics.StatusFailure, authReason(reason)).Inc()
}

// RefreshSucceeded 记录凭证续期成功的次数。
func (m *prometheusMetrics) RefreshSucceeded(_ context.Context) {
	m.operations.WithLabelValues(authapplication.MetricsOperationRefresh, platformmetrics.StatusSuccess, authapplication.MetricsReasonNone).Inc()
}

// RefreshFailed 记录凭证续期失败的次数及原因。
func (m *prometheusMetrics) RefreshFailed(_ context.Context, reason string) {
	m.operations.WithLabelValues(authapplication.MetricsOperationRefresh, platformmetrics.StatusFailure, authReason(reason)).Inc()
}

// LogoutSucceeded 记录用户退出登录成功的次数（区分仅退出当前设备还是退出所有设备）。
func (m *prometheusMetrics) LogoutSucceeded(_ context.Context, operation string) {
	m.operations.WithLabelValues(authLogoutOperation(operation), platformmetrics.StatusSuccess, authapplication.MetricsReasonNone).Inc()
}

// LogoutFailed 记录用户退出登录失败的次数。
func (m *prometheusMetrics) LogoutFailed(_ context.Context, operation string, reason string) {
	m.operations.WithLabelValues(authLogoutOperation(operation), platformmetrics.StatusFailure, authReason(reason)).Inc()
}

// TokenVersionMismatch 记录用户携带的凭证版本与服务器最新记录不一致的拦截次数（通常表示用户在其他地方改密或已作废旧凭证）。
func (m *prometheusMetrics) TokenVersionMismatch(_ context.Context, source string) {
	m.tokenVersionMismatches.WithLabelValues(authSource(source)).Inc()
}

// SessionPurgeSubmitFailed 记录将清理过期会话的任务提交到后台队列失败的次数。
func (m *prometheusMetrics) SessionPurgeSubmitFailed(context.Context) {
	m.sessionPurgeFailures.Inc()
}

// PasswordChangeSessionConsumeFailed 记录改密专用凭证核销失败的次数。
func (m *prometheusMetrics) PasswordChangeSessionConsumeFailed(_ context.Context, reason string) {
	m.passwordChangeConsumeFailures.WithLabelValues(authPasswordChangeReason(reason)).Inc()
}

// PasswordChangeSessionReuseRejected 记录已使用过的改密凭证被企图二次使用时被成功拦截的次数（防止重放攻击）。
func (m *prometheusMetrics) PasswordChangeSessionReuseRejected(context.Context) {
	m.passwordChangeReuseRejections.Inc()
}

// PasswordChangeRevocationProjectionFailed 记录修改密码后通知其他服务作废旧凭证失败的次数。
func (m *prometheusMetrics) PasswordChangeRevocationProjectionFailed(_ context.Context, step string) {
	m.passwordChangeRevocationFailures.WithLabelValues(authPasswordChangeRevocationStep(step)).Inc()
}

// PasswordChangeRevocationCompensationFailed 记录改密后重试作废旧凭证仍未成功的异常次数。
func (m *prometheusMetrics) PasswordChangeRevocationCompensationFailed(_ context.Context, reason string) {
	m.passwordChangeCompensationFailures.WithLabelValues(authPasswordChangeReason(reason)).Inc()
}

// authLogoutOperation 规范化退出操作的类型（仅退当前设备或退出全部设备），防止监控图表分类过于零散。
func authLogoutOperation(operation string) string {
	switch operation {
	case authapplication.MetricsOperationLogoutCurrent, authapplication.MetricsOperationLogoutAll:
		return operation
	default:
		return authapplication.MetricsOperationLogoutCurrent
	}
}

// authPasswordChangeReason 规范化改密失败原因，便于监控图表归类展示。
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

// authPasswordChangeRevocationStep 规范化改密后凭证作废的执行环节标识。
func authPasswordChangeRevocationStep(step string) string {
	if step == authapplication.MetricsPasswordChangeRevocationProjection {
		return step
	}
	return authapplication.MetricsPasswordChangeRevocationProjection
}

// authSource 规范化凭证的类型（是短期访问凭证还是长期续期凭证）。
func authSource(source string) string {
	switch source {
	case authapplication.MetricsSourceAccessToken, authapplication.MetricsSourceRefreshToken:
		return source
	default:
		return authapplication.MetricsSourceAccessToken
	}
}

// authReason 规范化用户身份验证失败的统计标签。
func authReason(reason string) string {
	switch reason {
	case authapplication.MetricsReasonNone,
		authapplication.MetricsReasonValidationFailed,
		authapplication.MetricsReasonCredentialInvalid,
		authapplication.MetricsReasonLoginLocked,
		authapplication.MetricsReasonLoginLockoutUnavailable,
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
		authapplication.MetricsReasonAccessDenylistFailed,
		authapplication.MetricsReasonAccessTokenDenied,
		authapplication.MetricsReasonSessionRevokeFailed,
		authapplication.MetricsReasonSystemError:
		return reason
	default:
		return authapplication.MetricsReasonSystemError
	}
}
