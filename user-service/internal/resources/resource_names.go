package resources

const (
	// NamePrimaryDB 是 user-service 主读写 PostgreSQL datastore 和 Ent 资源名。
	NamePrimaryDB = "primary_db"
	// NameCacheRedis 是用户服务缓存型运行时状态使用的具名 Redis 资源名。
	NameCacheRedis = "cache_redis"
)
