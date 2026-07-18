## 1. 配置预算校验

- [x] 1.1 检查 `common/runtime/config/defaults.go`、`common/runtime/config/validation.go` 和现有 config tests，确认当前 stop timeout、HTTP/gRPC shutdown timeout 与 tracing/logger 默认值来源。
- [x] 1.2 在 `common/runtime/config` 增加组合停止预算下限，覆盖 HTTP shutdown timeout、worker drain allowance、tracing flush allowance 和 shutdown safety margin，并保持错误信息包含 `runtime.lifecycle.stop_timeout` 与最低所需预算。
- [x] 1.3 如果现有默认 `runtime.lifecycle.stop_timeout` 小于新组合下限，同步调整默认值，确保未显式覆盖配置的本地和测试环境仍可启动。
- [x] 1.4 增加或更新 `common/runtime/config` 单元测试，覆盖预算不足失败、预算刚好满足通过、HTTP/gRPC 既有校验仍生效和错误路径完整性。

## 2. Worker 与 Auth 停止预算归属

- [x] 2.1 检查 `common/runtime/workerpool/pool.go` 的 `Stop(ctx)` 语义，确认不需要引入 auth 业务语义或默认业务 timeout。
- [x] 2.2 检查 `user-service/internal/features/auth/infrastructure/redis/session_store.go` 的 purge pool stop timeout，将 30 秒停止等待上限以 owning package 或服务组合层可引用的方式表达，避免与配置校验公式漂移。
- [x] 2.3 增加或更新 auth purge worker 相关测试，确认停止 timeout 仍包装或传播 context deadline，且不改变 refresh session、token version 或 Redis key 语义。

## 3. App Lifecycle 顺序测试

- [x] 3.1 检查 `user-service/internal/bootstrap/server.go`、`user-service/cmd/serve.go` 和 provider lifecycle hook 注册点，梳理 HTTP/pprof、RBAC watcher、feature worker/cache、Ent、Redis/PostgreSQL、tracing、logger 的停止顺序。
- [x] 3.2 增加 user-service App lifecycle recorder 测试，使用真实 Fx hook 机制或最小测试 module 验证 stop hooks 逆序串行执行、普通 stop error 后后续 hook 仍被调用。
- [x] 3.3 扩展 recorder 测试覆盖关键关闭顺序：HTTP/pprof 停止接收请求，RBAC watcher 与 feature worker/cache 在 Ent 和 datastore 前关闭，tracing 在 logger sync 前完成。
- [x] 3.4 增加共享 deadline 测试，验证前序 hook 消耗预算后后续 hook 只获得剩余 context，而不是重新创建完整 stop timeout。

## 4. 验证与收尾

- [x] 4.1 运行相关 Go 测试：`go test ./runtime/config ./runtime/workerpool`（在 `common/`）以及受影响的 user-service bootstrap/auth package 测试。
- [x] 4.2 运行 `make user-service-architecture-lint`，验证 OpenSpec 和架构边界变更。
- [x] 4.3 确认本 change 不需要 `make user-service-openapi-generate`、`make user-service-generate`、Atlas migration 或部署观测资产生成；如实现过程中触及对应生成物，补充运行相应生成和 drift 检查。
- [x] 4.4 将本次预期代码、测试和 OpenSpec artifact 变更加到暂存区后运行 `make lint`。
- [x] 4.5 在本次预期变更已暂存的状态下运行 `make verify`，避免最终 `git diff --exit-code` 被未暂存预期变更阻塞。
- [x] 4.6 所有实现、测试和验证完成后，把对应 checkbox 更新为 `- [x]`，并准备运行 `/opsx:archive add-shutdown-stop-budget-validation`。
