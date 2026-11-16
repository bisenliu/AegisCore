package application

import (
	"context"
	"testing"
)

func TestNopMetricsImplementsMetrics(t *testing.T) {
	t.Helper()
	metrics := NopMetrics()
	ctx := context.Background()

	metrics.PolicyReloadSucceeded(ctx, MetricsSourceLocalChange)
	metrics.PolicyReloadFailed(ctx, MetricsSourceWatcherPubSub, MetricsReasonReloadFailed)
	metrics.PolicyPublishSucceeded(ctx)
	metrics.PolicyPublishFailed(ctx, MetricsReasonPublishFailed)
	metrics.WatcherCheckFailed(ctx, MetricsSourceWatcherRevisionCheck, MetricsReasonRevisionStoreUnavailable)
	metrics.WatcherReloadSucceeded(ctx, MetricsSourceWatcherRevisionCheck)
	metrics.WatcherReloadFailed(ctx, MetricsSourceWatcherPubSub, MetricsReasonSystemError)
	metrics.WatcherVersionMismatch(ctx, MetricsSourceWatcherPubSub, MetricsReasonRevisionMismatch)
}
