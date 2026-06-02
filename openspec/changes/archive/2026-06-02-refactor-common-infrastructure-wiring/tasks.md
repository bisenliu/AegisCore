## 1. 基础设施 Provider 整理

- [x] 1.1 将 `ProvideNamedRedis` 从 `common/infrastructure/postgres.go` 移到 Redis 相关文件，并移除 `postgres.go` 中不再需要的 Redis import
- [x] 1.2 保持 `ProvideNamedPostgres` 在 PostgreSQL 相关文件中，确认两个导出 helper 的函数签名不变
- [x] 1.3 删除 `common/infrastructure/module.go`，确保 `common` 不再导出 `commoninfra.Module`

## 2. 用户服务启动装配

- [x] 2.1 在 `user-services/internal/bootstrap/bootstrap.go` 中移除对 `commoninfra.Module` 的引用
- [x] 2.2 在用户服务 Fx app 中显式提供 `commoninfra.NewConfig` 和 `commoninfra.NewLogger`
- [x] 2.3 确认用户服务仍只声明并初始化 `cache_redis`、`user_db`、`common_db` 和现有 Ent clients

## 3. 测试更新

- [x] 3.1 更新或删除引用 `commoninfra.Module` 的测试，改为验证显式公共 provider 不会自动创建 Redis/PostgreSQL 依赖
- [x] 3.2 保留 `ProvideNamedPostgres` 只提供声明连接池的测试覆盖
- [x] 3.3 保留 `ProvideNamedRedis` 只提供声明 client 的测试覆盖
- [x] 3.4 增加或调整用户服务 bootstrap 测试，验证启动装配不再依赖 `common/infrastructure.Module`

## 4. 文档与规格同步

- [x] 4.1 更新 `docs/ARCHITECTURE.md`，将运行时流程改为用户服务显式提供公共配置和 Zap logger
- [x] 4.2 更新 `docs/opsx/CAPABILITY_MAP.md`，移除 `common/infrastructure/module.go` 作为共享基础设施入口点
- [x] 4.3 确认无需修改 Ent schema、无需运行 `go generate ./ent`，且无需生成 Atlas migration

## 5. 验证

- [x] 5.1 对修改的 Go 文件运行 `gofmt`
- [x] 5.2 在 `common/` 执行 `go test ./...`
- [x] 5.3 在 `user-services/` 执行 `go test ./...`
- [x] 5.4 全仓搜索 `commoninfra.Module` 和 `ProvideNamedRedis`，确认旧 module 引用已清除且 Redis helper 位于 Redis 相关文件
