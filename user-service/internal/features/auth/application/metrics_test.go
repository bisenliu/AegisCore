package application

import (
	"context"
	"testing"
)

func TestNopMetricsImplementsMetrics(t *testing.T) {
	t.Helper()
	metrics := NopMetrics()
	ctx := context.Background()

	metrics.LoginSucceeded(ctx)
	metrics.LoginFailed(ctx, MetricsReasonCredentialInvalid)
	metrics.RefreshSucceeded(ctx)
	metrics.RefreshFailed(ctx, MetricsReasonRefreshTokenInvalid)
	metrics.LogoutSucceeded(ctx, MetricsOperationLogoutCurrent)
	metrics.LogoutFailed(ctx, MetricsOperationLogoutAll, MetricsReasonSessionRevokeFailed)
	metrics.TokenVersionMismatch(ctx, MetricsSourceAccessToken)
	metrics.SessionPurgeSubmitFailed(ctx)
}
