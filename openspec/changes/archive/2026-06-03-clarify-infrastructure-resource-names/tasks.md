## 1. 资源名文件整理

- [x] 1.1 将 `common/infrastructure/names.go` 重命名为 `common/infrastructure/resource_names.go`。
- [x] 1.2 为运行时资源名常量组添加中文注释，说明其用于 datastore 和 Ent 的 Fx wiring。
- [x] 1.3 确认 `NameUserDB`、`NameCommonDB`、`NameCacheRedis` 的名称和值保持不变。

## 2. 引用与文档检查

- [x] 2.1 搜索 `common/infrastructure/names.go` 旧路径引用，并更新主动维护文档中需要指向文件路径的位置。
- [x] 2.2 确认非 struct tag 的运行时资源名引用仍使用 `common/infrastructure` 中的公共常量。
- [x] 2.3 确认 `postgres.user_db`、`postgres.common_db`、`redis.cache_redis` 配置路径和 Fx name struct tag 未被改动。

## 3. 验证

- [x] 3.1 对修改过的 Go 文件运行 `gofmt`。
- [x] 3.2 在 `common/` 运行 `go test ./...`。
- [x] 3.3 在 `user-services/` 运行 `go test ./...`，确认服务侧 wiring 不受影响。
- [x] 3.4 运行 `openspec status --change "clarify-infrastructure-resource-names"`，确认变更 artifacts 和任务状态可被 apply 阶段识别。
