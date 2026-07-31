package application

import (
	"context"
	"testing"
	"time"
)

func TestNopMetricsImplementsMetrics(t *testing.T) {
	t.Helper()
	metrics := NopMetrics()
	ctx := context.Background()

	metrics.PolicyReloadSucceeded(ctx, MetricsSourceLocalChange)
	metrics.PolicyReloadFailed(ctx, MetricsSourceWatcherPubSub, MetricsReasonReloadFailed)
	metrics.PolicyPublishSucceeded(ctx)
	metrics.PolicyPublishFailed(ctx, MetricsReasonPublishFailed)
	metrics.WatcherCheckFailed(ctx, MetricsReasonStoreUnavailable)
	metrics.WatcherReloadSucceeded(ctx, MetricsSourceWatcherVersionCheck)
	metrics.WatcherReloadFailed(ctx, MetricsSourceWatcherPubSub, MetricsReasonSystemError)
	metrics.WatcherVersionMismatch(ctx, MetricsSourceWatcherPubSub)
	metrics.DispatcherOperationObserved(ctx, MetricsOperationDispatcherClaim, MetricsResultSuccess, MetricsReasonNone, MetricsKindNone)
	metrics.DispatcherBacklogObserved(ctx, 1, time.Second)
	metrics.DispatcherRunningObserved(ctx, true)
}
