# Testing

## 1. Test Entry Points

| 范围 | 命令 | 说明 |
|---|---|---|
| 全仓库 | 分别执行 `go test ./...` | 在 `common/` 和 `user-services/` 执行；仓库根目录本身不是 Go module |
| 共享模块 | `go test ./...` | 在 `common/` 执行 |
| 用户服务模块 | `go test ./...` | 在 `user-services/` 执行 |

## 2. What To Validate

- Controller：路径参数解析、校验失败、service 错误映射、成功响应。
- Service：repository 返回值到 DTO 的字段映射，repository 错误通过 `response.FromError` 转换。
- Repository：Ent not found 转 `NOT_FOUND`，其他查询错误保留 cause 并映射为 internal error。
- Middleware：trace-id 透传/生成、写入 Gin context/Go context/响应头、panic recovery 输出统一错误、request logging 携带 trace-id、CORS 处理 OPTIONS。
- Config loader：显式配置加载、`AEGISCORE_` 环境变量覆盖、命名 Redis/PostgreSQL 实例反序列化；`common/config.Load` 不应因 required/range 字段校验拒绝缺失或零值配置。
- Runtime/Infrastructure：Fx 生命周期启动与停止，HTTP server 使用配置中的 host/port/timeouts，用户服务声明 `cache_redis`、`user_db`、`common_db`，依赖不可用或底层库拒绝配置时启动失败。
- Logging：Zap logger 初始化、分类日志文件、trace-id 字段和无 request context 时的日志行为。

## 3. External Dependencies

用户服务启动会连接 Redis 和 PostgreSQL。单元测试应尽量隔离外部依赖；需要真实 Redis/PostgreSQL 的测试应明确作为集成测试，并在文档或测试名中说明依赖。

## 4. Generated Code

Ent 生成代码通常不需要逐文件测试。测试应覆盖 schema 约束、repository 行为或 API 行为。修改 `user-services/ent/schema/` 后运行 `go generate ./ent` 并再执行相关测试。

## 5. Database Migration Verification

涉及 Ent schema 或 SQL migration 的变更应验证：

1. 在 `user-services/` 运行 `go generate ./ent`。
2. 在 `user-services/` 运行 `./scripts/migrate-diff.sh <name>` 或确认现有 migration 已覆盖 schema 变化。
3. Review `user-services/migrations/*.sql`；如手工修改 SQL，运行 `atlas migrate hash --dir file://migrations`。
4. 在 `user-services/` 运行 `./scripts/migrate-validate.sh`。
5. 确认运行时没有通过 `client.Schema.Create(ctx)` 自动修改 schema。

## 6. OPSX Verification

每个 change 实现完成后至少执行：

1. 与改动范围匹配的 `go test ./...`；跨模块变更时分别在 `common/` 和 `user-services/` 执行。
2. 如涉及 Ent schema，执行 `go generate ./ent` 并检查生成结果。
3. 如涉及 migration，执行 `./scripts/migrate-validate.sh` 并检查 `atlas.sum` 与 SQL 文件一致。
4. 如涉及 HTTP API，验证成功和失败响应均符合 `common/response.Envelope`。
5. 如涉及配置或启动流程，验证 loader 行为、依赖不可用和优雅停止场景。
