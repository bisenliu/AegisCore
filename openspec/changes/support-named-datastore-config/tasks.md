## 1. 配置结构与示例配置

- [x] 1.1 将 `user-services/configs/config.yaml` 改为使用 Redis 与 PostgreSQL YAML anchor/merge 基础模板，并声明 `redis.cache_redis`、`redis.queue_redis`、`postgre.user_db`、`postgre.pay_db`、`postgre.common_db`。
- [x] 1.2 更新 `common/config.Config`，将 Redis 与 PostgreSQL 建模为命名实例 map，并将 PostgreSQL 顶层配置路径映射为 `postgre`。
- [x] 1.3 更新 Redis/PostgreSQL 配置查找方法，使调用方可以按实例名获取 Redis 配置和 PostgreSQL 单实例 DSN/连接池参数。
- [x] 1.4 更新配置校验逻辑，覆盖命名实例必填字段、端口、DB 编号、连接池大小和 timeout 范围。

## 2. 共享基础设施 Provider

- [x] 2.1 修改 `common/infrastructure.Module`，使其只固定提供配置与日志，不固定提供未命名 Redis client。
- [x] 2.2 修改 Redis provider，使 `common` 保留按实例名创建 Redis client、注册 ping/close lifecycle 的基础代码。
- [x] 2.3 修改 PostgreSQL provider 以使用新的 `postgre.<name>` 实例配置，并保持按服务声明的单实例连接模式。
- [x] 2.4 确保 Redis/PostgreSQL provider 在实例名不存在时返回包含实例名的清晰错误。

## 3. 用户服务装配

- [x] 3.1 更新 `user-services/internal/bootstrap` 中 PostgreSQL pools 声明，使 `user_db` 和 `common_db` 分别读取 `postgre.user_db` 与 `postgre.common_db`。
- [x] 3.2 如用户服务需要保留 Redis 启动依赖，则在用户服务 module 中声明所需 Redis 命名 client；否则确保没有代码依赖 common module 自动提供 Redis。
- [x] 3.3 保持 `user-services/internal/entclient` 使用具名 `user_db` 和 `common_db` `*sql.DB` 创建 Ent clients，不修改 Ent 生成代码。

## 4. 测试与验证

- [x] 4.1 更新 `common/config` 测试，覆盖命名 Redis/PostgreSQL 配置加载、YAML merge 后字段读取和 `AEGISCORE_` 环境变量覆盖。
- [x] 4.2 更新 `common/infrastructure` 测试，覆盖 PostgreSQL/Redis 实例名不存在、DSN 生成、连接池参数和 lifecycle 注册行为。
- [x] 4.3 更新 `user-services/internal/bootstrap` 测试，验证用户服务只声明并连接所需 PostgreSQL/Redis 实例，不连接 `pay_db` 或未声明 Redis 实例。
- [x] 4.4 在 `common/` 运行 `go test ./...`。
- [x] 4.5 在 `user-services/` 运行 `go test ./...`。
- [x] 4.6 对修改过的 Go 文件运行 `gofmt -w`。
