package application

import "context"

const (
	// MetricsOperationPolicyReload 表示 RBAC policy reload 操作。
	MetricsOperationPolicyReload = "policy_reload"
	// MetricsOperationPolicyPublish 表示 RBAC policy version 发布操作。
	MetricsOperationPolicyPublish = "policy_publish"
	// MetricsOperationWatcherReload 表示 watcher 触发的远端 reload 操作。
	MetricsOperationWatcherReload = "watcher_reload"
	// MetricsOperationWatcherVersionCheck 表示 watcher 版本补偿检查。
	MetricsOperationWatcherVersionCheck = "watcher_version_check"

	// MetricsReasonNone 表示操作成功或无需补充原因。
	MetricsReasonNone = "none"
	// MetricsReasonReloadFailed 表示 policy reload 失败。
	MetricsReasonReloadFailed = "reload_failed"
	// MetricsReasonPublishFailed 表示 policy version 发布失败。
	MetricsReasonPublishFailed = "publish_failed"
	// MetricsReasonStoreUnavailable 表示 policy version store 不可用。
	MetricsReasonStoreUnavailable = "store_unavailable"
	// MetricsReasonSystemError 表示未能稳定归类的系统异常。
	MetricsReasonSystemError = "system_error"

	// MetricsSourceLocalChange 表示本实例在线 RBAC 变更触发。
	MetricsSourceLocalChange = "local_change"
	// MetricsSourceWatcherPubSub 表示 watcher Pub/Sub 消息触发。
	MetricsSourceWatcherPubSub = "watcher_pubsub"
	// MetricsSourceWatcherVersionCheck 表示 watcher 定时版本补偿触发。
	MetricsSourceWatcherVersionCheck = "watcher_version_check"

	// MetricsRouteDiffMissing 表示已注册路由缺少正式权限。
	MetricsRouteDiffMissing = "missing"
	// MetricsRouteDiffStale 表示正式权限没有对应已注册路由。
	MetricsRouteDiffStale = "stale"
)

// Metrics 记录 permission/RBAC feature 的低基数业务指标。
type Metrics interface {
	PolicyReloadSucceeded(ctx context.Context, source string)
	PolicyReloadFailed(ctx context.Context, source string, reason string)
	PolicyPublishSucceeded(ctx context.Context)
	PolicyPublishFailed(ctx context.Context, reason string)
	WatcherCheckFailed(ctx context.Context, reason string)
	WatcherReloadSucceeded(ctx context.Context, source string)
	WatcherReloadFailed(ctx context.Context, source string, reason string)
	WatcherVersionMismatch(ctx context.Context, source string)
	RouteDiffObserved(ctx context.Context, missing int, stale int)
}

type nopMetrics struct{}

// NopMetrics 返回 permission/RBAC 业务指标空实现。
func NopMetrics() Metrics {
	return nopMetrics{}
}

func (nopMetrics) PolicyReloadSucceeded(context.Context, string)       {}
func (nopMetrics) PolicyReloadFailed(context.Context, string, string)  {}
func (nopMetrics) PolicyPublishSucceeded(context.Context)              {}
func (nopMetrics) PolicyPublishFailed(context.Context, string)         {}
func (nopMetrics) WatcherCheckFailed(context.Context, string)          {}
func (nopMetrics) WatcherReloadSucceeded(context.Context, string)      {}
func (nopMetrics) WatcherReloadFailed(context.Context, string, string) {}
func (nopMetrics) WatcherVersionMismatch(context.Context, string)      {}
func (nopMetrics) RouteDiffObserved(context.Context, int, int)         {}
