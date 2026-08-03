## 1. 子包结构

- [x] 1.1 创建 `user-service/internal/providers/datastore`、`observability`、`security` 和 `transport` 子包，并为每个子包定义清晰的 Fx module 入口。
- [x] 1.2 将 `providers` 根包改为仅组合子包 `WiringModule`、`RuntimeModule` 和 `Module`，不保留具体 provider 构造器 wrapper、alias 或兼容分支。

## 2. 代码迁移

- [x] 2.1 将 PostgreSQL、Redis、Ent client、Ent plugins、Ent SQL log、Ent metrics 和 Ent tracing provider 迁移到 `providers/datastore`。
- [x] 2.2 将 health checks、runtime dependency metrics、metrics provider 和 tracing provider 接线迁移到 `providers/observability`。
- [x] 2.3 将 JWT service、认证 token policy 和 password service 接线迁移到 `providers/security`。
- [x] 2.4 将 Gin mode、Gin engine、routes 和 API rate limiters 迁移到 `providers/transport`。
- [x] 2.5 更新 `user-service/internal/bootstrap/app.go` 和 provider 内部 imports，确保正式 App 仍通过 `providers.WiringModule` 与 `providers.RuntimeModule` 接入完整运行时链路。

## 3. 测试迁移

- [x] 3.1 将 datastore 相关测试迁移到 `providers/datastore`，并确认不新增测试专用生产 API。
- [x] 3.2 将 observability 相关测试迁移到 `providers/observability`，覆盖 health checks 和 runtime dependency metrics。
- [x] 3.3 将 security 相关测试迁移到 `providers/security`，覆盖 JWT policy 行为。
- [x] 3.4 将 Gin、routes、auth middleware、ratelimit 和 HTTP middleware 相关测试迁移到 `providers/transport`。
- [x] 3.5 保留或新增仅验证根包 module 汇总语义的 `providers` 根包测试。

## 4. 文档与规格同步

- [x] 4.1 更新 `docs/ARCHITECTURE.md` 中 `user-service/internal/providers/` 的职责描述，列出四个子包边界。
- [x] 4.2 更新 `docs/opsx/CAPABILITY_MAP.md` 中相关 capability 的主要代码位置，避免继续只指向过宽根目录。
- [x] 4.3 检查 `openspec/changes/split-user-service-providers-by-concern/` artifacts 与最终实现一致，必要时更新 proposal、design、spec delta 和 tasks。

## 5. 验证

- [x] 5.1 执行 `go test ./user-service/internal/providers/...`，确认拆包后的 provider 测试通过。
- [x] 5.2 执行 `go test ./user-service/internal/bootstrap`，确认正式 App composition root 与 lifecycle 链路仍可构建和验证。
- [x] 5.3 执行 `make user-service-architecture-lint`，确认目录结构、分层边界和正式架构来源一致。
- [x] 5.4 将本次预期代码、文档和 OpenSpec artifacts 加到暂存区。
- [x] 5.5 执行 `make lint`，确认 lint 门禁通过。
- [x] 5.6 执行 `make verify`，确认完整验证和 drift 检查通过。
