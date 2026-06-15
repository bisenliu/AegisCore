package redis

import "github.com/aegiscore/common/runtime/rediskey"

// KeyCatalog 构造权限 RBAC Redis adapter 私有 key 和 channel。
type KeyCatalog struct {
	builder rediskey.Builder
}

// NewKeyCatalog 根据 app name 构造权限 RBAC Redis key catalog。
func NewKeyCatalog(appName string) (KeyCatalog, error) {
	builder, err := rediskey.NewBuilder(rediskey.Options{Namespace: appName})
	if err != nil {
		return KeyCatalog{}, err
	}
	scoped, err := builder.Scoped("rbac")
	if err != nil {
		return KeyCatalog{}, err
	}
	return KeyCatalog{builder: scoped}, nil
}

// MustKeyCatalog 根据 app name 构造权限 RBAC Redis key catalog，配置无效时 panic。
func MustKeyCatalog(appName string) KeyCatalog {
	catalog, err := NewKeyCatalog(appName)
	if err != nil {
		panic(err)
	}
	return catalog
}

// PolicyVersionKey 返回 RBAC policy 版本 Redis key。
func (c KeyCatalog) PolicyVersionKey() string {
	return c.builder.MustKey("policy", "version")
}

// PolicyChannel 返回 RBAC policy 刷新 Pub/Sub channel。
func (c KeyCatalog) PolicyChannel() string {
	return c.builder.MustKey("policy", "refresh")
}
