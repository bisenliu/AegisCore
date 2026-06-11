# Testing

## 1. Test Entry Points

| 范围 | 命令 | 说明 |
|---|---|---|
| 全仓库 | `make test` | 从仓库根目录分别在 `common/` 和 `user-service/` 执行测试；仓库根目录本身不是 Go module |
| 共享模块 | `go test ./...` | 在 `common/` 执行 |
| 用户服务模块 | `go test ./...` | 在 `user-service/` 执行 |

## 2. What To Validate

- Controller：路径参数解析、校验失败、application service 错误映射、成功响应。
- Application service：infrastructure adapter 返回值到 DTO 的字段映射，领域或应用错误通过 `contract/errors.FromError` 转换。
- Infrastructure adapter：Ent not found 转应用层 not found 错误，其他查询错误保留 cause 并映射为 internal error；Redis adapter 覆盖 key、TTL、索引和清理语义。
- Middleware：trace-id 透传/生成、写入 Gin context/Go context/响应头、panic recovery 输出统一错误、request logging 携带 trace-id、CORS 处理 OPTIONS。
- Config loader：显式配置加载、`AEGISCORE_` 环境变量覆盖、命名 Redis/PostgreSQL 实例反序列化；`common/runtime/config.Load` 不应因 required/range 字段校验拒绝缺失或零值配置。
- Runtime/Infrastructure：Fx 生命周期启动与停止，HTTP server 使用配置中的 host/port/timeouts，用户服务声明 `cache_redis`、`user_db`、`common_db`，依赖不可用或底层库拒绝配置时启动失败。
- Logging：Zap logger 初始化、分类日志文件、trace-id 字段和无 request context 时的日志行为。

## 3. External Dependencies

用户服务启动会连接 Redis 和 PostgreSQL。单元测试应尽量隔离外部依赖；需要真实 Redis/PostgreSQL 的测试应明确作为集成测试，并在文档或测试名中说明依赖。

## 4. Integration And E2E Dependencies

普通 `go test ./...` 和 `make test` 不要求 Docker，也不默认启动真实 PostgreSQL/Redis。需要验证真实外部依赖语义的 integration/e2e 测试应使用 `common/testing/containers`：

```go
postgres := containers.StartPostgres(ctx, t, containers.PostgresOptions{})
redis := containers.StartRedis(ctx, t, containers.RedisOptions{})
```

容器测试通过环境变量显式启用：

```bash
AEGISCORE_TEST_CONTAINERS=1 go test ./...
```

未设置 `AEGISCORE_TEST_CONTAINERS` 时，`common/testing/containers` 的 integration-gated tests 会跳过。设置该变量后，如果 Docker 或容器镜像不可用，测试应失败并报告明确的启动、端口映射、ping 或清理错误，避免误判集成验证已经执行。

使用取舍：

- `domain` 和 `application` 单元测试优先使用 stub/fake，不连接外部服务。
- Redis 命令语义测试可以继续使用 `miniredis`；只有需要真实 Redis 行为差异时再使用 `common/testing/containers.StartRedis`。
- Ent/PostgreSQL adapter 当前可继续用 SQLite 覆盖可移植查询语义；只有需要 PostgreSQL-specific SQL、migration、constraint 或连接配置行为时再使用 `common/testing/containers.StartPostgres`。
- `user-service` 可选择性导入 `github.com/aegiscore/common/testing/containers`，但不要求现有测试立即迁移。
- 业务 fixture 仍放在对应 feature 的测试文件或测试包内；`common/testing/fixtures` 只提供用户名、邮箱、名称、UUID 等无业务语义的基础值。

## 5. Generated Code

Ent 生成代码通常不需要逐文件测试。测试应覆盖 schema 约束、infrastructure adapter 行为或 API 行为。修改 `user-service/ent/schema/` 后运行 `go generate ./ent` 并再执行相关测试。

## 6. Database Migration Verification

涉及 Ent schema 或 SQL migration 的变更应验证：

1. 在 `user-service/` 运行 `go generate ./ent`。
2. 在 `user-service/` 运行 `./scripts/migrate-diff.sh <name>` 或确认现有 migration 已覆盖 schema 变化。
3. Review `user-service/migrations/*.sql`；如手工修改 SQL，运行 `atlas migrate hash --dir file://migrations`。
4. 在 `user-service/` 运行 `./scripts/migrate-validate.sh`。
5. 确认运行时没有通过 `client.Schema.Create(ctx)` 自动修改 schema。

## 7. Change Verification

每个 change 实现完成后至少执行：

1. 与改动范围匹配的测试；全仓库使用 `make test`，跨模块变更也可分别在 `common/` 和 `user-service/` 执行 `go test ./...`。
2. 如涉及 Ent schema，执行 `go generate ./ent` 并检查生成结果。
3. 如涉及 migration，执行 `./scripts/migrate-validate.sh` 并检查 `atlas.sum` 与 SQL 文件一致。
4. 如涉及 HTTP API，验证成功和失败响应均符合 `common/contract/response.Envelope`。
5. 如涉及配置或启动流程，验证 loader 行为、依赖不可用和优雅停止场景。
