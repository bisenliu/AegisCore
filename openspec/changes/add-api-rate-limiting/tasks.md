## 1. 共享契约与本地限流 primitive

- [x] 1.1 在 `common/go.mod` 引入 `golang.org/x/time/rate`，并更新 `common/go.sum`。
- [x] 1.2 在 `common/contract/errors` 新增限流 `Kind`、`Reason`、`Code` 和 factory，使用 60xxx 错误码段。
- [x] 1.3 在 `common/http/response` 将限流错误映射为 `429 Too Many Requests`，并补充 response 单元测试。
- [x] 1.4 在 `common/http/middleware` 实现业务中立的本地限流 store，支持分片 map、每 key 独立 `rate.Limiter`、key TTL、后台 janitor 和显式关闭。
- [x] 1.5 在 `common/http/middleware` 实现 Gin 限流 middleware 和 key resolver，支持客户端 IP key 与已认证 User ID key。
- [x] 1.6 为本地限流 store 和 Gin middleware 增加并发、超限、缺失 key、TTL 清理、janitor 停止和响应 envelope 测试。

## 2. user-service 配置与资源接线

- [x] 2.1 在 `user-service/internal/config` 新增 API 限流配置结构、默认值和严格字段校验，不读取旧字段或提供兼容别名。
- [x] 2.2 增加配置测试，覆盖未配置默认值、禁用限流、启用后非法 rate/burst/ttl/cleanup interval/shard count 的字段路径错误。
- [x] 2.3 在 `user-service/internal/providers` 或等价 composition 边界构造匿名 IP limiter 与已认证 User ID limiter，并通过 Fx lifecycle 启动和停止 janitor。
- [x] 2.4 确保限流资源关闭只停止自身后台 goroutine，且不关闭 Redis、PostgreSQL、Ent、Casbin 或其他共享资源。

## 3. 路由挂载与行为测试

- [x] 3.1 在 `user-service/internal/router/router.go` 将匿名 IP 限流挂载到 `/api/v1/auth` 公开路由组，位于 auth controller 前。
- [x] 3.2 在已认证 route group 中将 User ID 限流挂载到 `AuthWithTokenVersionValidator` 之后、`permissionhttp.Authorize` 之前。
- [x] 3.3 确认 health、startup/readiness、metrics、OpenAPI 和 pprof 路由不进入业务限流链。
- [x] 3.4 增加 router registration 或 HTTP 行为测试，覆盖公开认证接口按 IP 超限、认证失败不消费 User ID 限流、已认证业务接口超限时不调用 RBAC authorizer。
- [x] 3.5 确认未超限的已认证业务请求仍继续执行当前 RBAC 授权和 controller 行为。

## 4. 文档、规格与验证

- [x] 4.1 更新 `docs/opsx/CAPABILITY_MAP.md`，加入 `api-rate-limiting` capability 与相关路径映射。
- [x] 4.2 运行 `make test`，修复所有失败。
- [x] 4.3 运行 `make user-service-architecture-lint`，确认架构边界和 OpenSpec artifacts 合规。
- [x] 4.4 检查是否需要运行 `make user-service-openapi-generate`；若生成物变化，检查 diff；若无 API 路由形态变化，记录无需生成。
- [x] 4.5 将本次预期代码、配置、测试、文档和 OpenSpec 变更加到暂存区。
- [x] 4.6 运行 `make lint`，修复所有失败。
- [x] 4.7 运行 `make verify`，修复所有失败并确认最终无未暂存的预期 drift。
