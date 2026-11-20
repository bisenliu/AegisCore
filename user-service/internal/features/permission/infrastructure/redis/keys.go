package redis

import "github.com/aegiscore/platform/runtime/rediskey"

// KeyCatalog 构造权限 RBAC Redis adapter 私有 key 和 channel。
//
// 集群分片与 HashTag 说明：
// 权限策略版本号（PolicyVersionKey）与刷新通知 Pub/Sub 频道（PolicyChannel）通过 HashTag("sync")
// 显式锁定同一哈希槽，确保在多实例部署或 Redis Cluster 拓扑变迁下与 policy 同步相关的元数据处于确定的同一节点。
type KeyCatalog struct {
	policyVersionKey string
	policyChannel    string
}

// NewKeyCatalog 根据 app name 构造权限 RBAC Redis key catalog。
func NewKeyCatalog(appName string) (KeyCatalog, error) {
	builder, err := rediskey.NewBuilder(rediskey.Options{
		Namespace: appName,
		Scope:     []string{"rbac"},
	})
	if err != nil {
		return KeyCatalog{}, err
	}
	return KeyCatalog{
		policyVersionKey: builder.MustKey("policy", rediskey.HashTag("sync"), "version"),
		policyChannel:    builder.MustKey("policy", rediskey.HashTag("sync"), "refresh"),
	}, nil
}

// PolicyVersionKey 返回 RBAC policy 版本 Redis key。
func (c KeyCatalog) PolicyVersionKey() string {
	return c.policyVersionKey
}

// PolicyChannel 返回 RBAC policy 刷新 Pub/Sub channel。
func (c KeyCatalog) PolicyChannel() string {
	return c.policyChannel
}
