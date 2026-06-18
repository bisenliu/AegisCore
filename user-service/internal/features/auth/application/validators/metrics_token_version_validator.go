package validators

import (
	"context"
	"errors"

	commonauth "github.com/aegiscore/common/security/auth"
	authapplication "github.com/aegiscore/user-service/internal/features/auth/application"
)

type metricsTokenVersionValidator struct {
	next    commonauth.TokenVersionValidator
	metrics authapplication.Metrics
}

// NewMetricsTokenVersionValidator 为 access token version mismatch 增加业务指标记录。
func NewMetricsTokenVersionValidator(next commonauth.TokenVersionValidator, metrics authapplication.Metrics) commonauth.TokenVersionValidator {
	if metrics == nil {
		metrics = authapplication.NopMetrics()
	}
	return metricsTokenVersionValidator{next: next, metrics: metrics}
}

// ValidateTokenVersion 委托底层校验器，并在 version mismatch 时记录固定来源指标。
func (v metricsTokenVersionValidator) ValidateTokenVersion(ctx context.Context, userID string, tokenVersion int64) error {
	if v.next == nil {
		return nil
	}
	err := v.next.ValidateTokenVersion(ctx, userID, tokenVersion)
	if errors.Is(err, commonauth.ErrTokenVersionMismatch) {
		v.metrics.TokenVersionMismatch(ctx, authapplication.MetricsSourceAccessToken)
	}
	return err
}
