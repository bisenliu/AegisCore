## 1. Redis Key Prefix

- [x] 1.1 在 `user-services/internal/service` 中提供由 `config.App.Name` 裁剪得到 key prefix 的 `RedisKeyBuilder`，不新增配置校验和 default。
- [x] 1.2 将 token version 缓存、Refresh Token 会话记录和用户活跃会话索引的 key 构造改为统一走 `RedisKeyBuilder`，非空 prefix 时生成 `<app.name>:auth:*`，空 prefix 时保持 `auth:*`。
- [x] 1.3 保持 `repository.AuthSessionRepository` 抽象、Redis client 注入、TTL、ZSet member/score 和过期 member 清理行为不变。

## 2. Startup Logging

- [x] 2.1 在 `user-services/internal/bootstrap/server.go` 的 `NewHTTPServer` 启动日志中保留现有 `addr` 字段。
- [x] 2.2 在同一条 HTTP server 启动日志中追加 `service`、`environment` 和 `timezone` 字段，分别来自 `config.App.Name`、`config.App.Environment` 和 `config.System.Timezone`。
- [x] 2.3 保持 HTTP listener 绑定、启动失败返回、异步 serve、`http.ErrServerClosed` 忽略和 graceful shutdown 行为不变。

## 3. Tests

- [x] 3.1 更新 `user-services/internal/repository/redis` 单元测试，覆盖非空 `app.name` 时所有认证会话 Redis key 使用服务名前缀。
- [x] 3.2 增加空 `app.name` 测试，确认 Redis key 保持无前缀 `auth:*` 且不会使用代码级默认服务名。
- [x] 3.3 增加或更新 HTTP server 启动日志测试，确认日志保留 `addr` 并包含 `service`、`environment`、`timezone` 字段。

## 4. Verification

- [x] 4.1 在 `user-services` 模块运行 `go test ./...`。
- [x] 4.2 如改动触及 common 编译边界，在 `common` 模块运行 `go test ./...`。
- [x] 4.3 运行 `gofmt` 格式化所有修改的 Go 文件。
