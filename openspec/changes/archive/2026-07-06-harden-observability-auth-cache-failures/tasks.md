## 1. Redis Metrics Context 语义

- [x] 1.1 更新 `common/runtime/observability/metrics/redis.go` 中 `Collect` 与 `CollectContext` 的注释，明确标准 `Collect` 是 background context fallback，真实 scrape 取消只通过 `Provider.GatherContext` 或 `Provider.HTTPHandler` 传递。
- [x] 1.2 在 `common/runtime/observability/metrics` 测试中增加阻塞 Redis pinger，验证经 `Provider.GatherContext` 采集时 request context 取消会传入 Redis PING。
- [x] 1.3 在 `common/runtime/observability/metrics` 测试中验证直接调用 Redis collector 的标准 `Collect` 不依赖 scrape context，只由 collector timeout 结束并保持 metric family、label 与数值语义稳定。

## 2. Token Version 本地失效错误处理

- [x] 2.1 修改 `user-service/internal/features/auth/application/validators.TokenVersionLocalInvalidator` 接口，使 `InvalidateTokenVersion(userID string)` 返回 `error`。
- [x] 2.2 修改 `TokenVersionValidator.InvalidateTokenVersion`，返回并包装 `localcache.Delete` 失败，删除所有忽略错误的逻辑。
- [x] 2.3 修改 `user-service/internal/features/auth/application/sessions` 的本地 token version 失效调用，使每次失败记录包含 `user_id` 的日志并加入 `projectionErr`，且不中断后续 Redis 投影刷新或 refresh session 删除。
- [x] 2.4 同步更新 `sessions` 包 gomock 测试桩、expectation 和 validator 测试，使新接口签名、成功路径和失败路径均可编译并被断言。

## 3. 测试与验证

- [x] 3.1 增加 validator 测试：关闭 `localcache` 后调用 `InvalidateTokenVersion` MUST 返回 `localcache.ErrClosed` 包装错误。
- [x] 3.2 增加 sessions revocation 测试：本地 token version cache 失效失败 MUST 出现在 `RevokeUserSessionsAtVersion` 返回错误中，并且后续 Redis 投影刷新和 refresh session 删除仍会执行。
- [x] 3.3 运行 `go test ./common/runtime/observability/metrics ./user-service/internal/features/auth/application/validators ./user-service/internal/features/auth/application/sessions` 并通过。
- [x] 3.4 运行 `make user-service-architecture-lint` 并通过。

## 4. 最终验证

- [x] 4.1 检查 `git diff`，确认未修改 Ent 生成物、OpenAPI 生成物、数据库 migration 或部署资产。
- [x] 4.2 将本 change 的预期代码、测试和 OpenSpec artifact 变更加到暂存区。
- [x] 4.3 运行 `make lint` 并通过。
- [x] 4.4 运行 `make verify` 并通过，确认最终无未暂存预期 drift。
