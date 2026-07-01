## 1. 生成工具与规范入口

- [x] 1.1 在涉及生成的 Go module 中显式声明 `go.uber.org/mock/mockgen` 工具入口，确保生成流程不依赖全局二进制。
- [x] 1.2 在 `common/runtime/observability/metrics` 边界实现业务中立的 metrics no-op 生成工具或生成规范，不引入 user-service 业务指标方法。
- [x] 1.3 为 `auth/application` 和 `permission/application` 的 `Metrics` interface 增加 `go:generate` 入口，生成 `metrics_nop_gen.go`。
- [x] 1.4 运行 `go generate ./...` 并检查生成的 metrics no-op 文件能编译匹配对应 feature-local `Metrics` interface。

## 2. Metrics no-op 一次性迁移

- [x] 2.1 删除 `user-service/internal/features/auth/application/metrics.go` 中手写 `nopMetrics` 方法实现，保留业务常量、`Metrics` interface 和 feature-local `NopMetrics()` 入口语义。
- [x] 2.2 删除 `user-service/internal/features/permission/application/metrics.go` 中手写 `nopMetrics` 方法实现，保留业务常量、`Metrics` interface 和 feature-local `NopMetrics()` 入口语义。
- [x] 2.3 增加或更新 auth 与 permission 的 metrics no-op 生成物测试，验证 `NopMetrics()` 可赋值给对应 `Metrics` interface 且无运行时副作用。
- [x] 2.4 运行 auth 与 permission 相关包测试，确认 metrics 禁用、policy reload、watcher、auth command、session store 和 token version 校验行为不变。

## 3. 高重复 mock 迁移

- [x] 3.1 为 `auth/application/command`、`auth/application/sessions`、`auth/application/validators` 中的高重复 ports 增加 feature-local `mockgen` 生成入口和 mock 生成物。
- [x] 3.2 删除 auth command、sessions、validators 测试中被生成 mock 替代的高重复 `authRepoStub`、`sessionStoreStub`、`credentialsStub`、`tokenVersion*Stub` 等手写 test double。
- [x] 3.3 为 `role/application/command`、`role/application/query`、`role/application/seed` 中的 store、lookup 和 notifier ports 增加 feature-local `mockgen` 生成入口和 mock 生成物。
- [x] 3.4 删除 role command、query、seed 测试中被生成 mock 替代的高重复 `stubRoleStore`、`stubUserRoleStore`、`stubRolePermissionStore`、`fakeSeed*Store` 等手写 test double。
- [x] 3.5 为 `permission/application/command`、`permission/application/policy_sync`、`permission/application/query` 和必要 transport 测试中的高重复 ports 增加 feature-local `mockgen` 生成入口和 mock 生成物。
- [x] 3.6 删除 permission 测试中被生成 mock 替代的高重复 `stubPolicy*`、`stubPermissionStore`、`stubRouteScanner`、`fakeEngine` 等手写 test double。
- [x] 3.7 评估 user 和 common 测试中的小型 test double；对高重复 interface 使用生成 mock，对复杂状态型对象保留并改用 `testHarness`、`testStore`、`recordingMetrics` 或等价描述性名称。

## 4. 生成与交付校验

- [x] 4.1 更新 Makefile、脚本或 verify 链路，使 mock 和 metrics no-op 生成物可通过仓库约定的 Go 生成命令统一刷新。
- [x] 4.2 在完整验证中加入生成物 drift 检查，确保执行生成后通过 `git diff --exit-code` 暴露未提交的 mock 或 metrics no-op 变化。
- [x] 4.3 更新或扩展 user-service 架构 lint 规则，禁止新增全局 `mocks/`、`testmocks/`、`common/mocks/` 或等价中央 mock 仓库。
- [x] 4.4 运行 `go generate ./...`，然后运行 `git diff --exit-code` 检查生成物没有未提交 drift。

## 5. 测试与最终验证

- [x] 5.1 运行被迁移 package 的相关 Go 测试，至少覆盖 auth command/sessions/validators、permission command/policy_sync/query、role command/query/seed、user command/query/transport 和受影响 common package。
- [x] 5.2 运行 `make test`，确认 common 与 user-service 全部 Go 测试通过。
- [x] 5.3 运行 `make user-service-architecture-lint`，确认 feature 边界、OPSX 文档和中央 mock 仓库禁止规则通过。
- [x] 5.4 运行 `make lint`，确认 Go lint 通过。
- [x] 5.5 运行 `make verify`，确认 lint、架构检查、测试、OpenAPI 生成和生成物 drift 检查全部通过。
- [x] 5.6 验证本 change 不产生 HTTP API、数据库 schema、OpenAPI 生成物、部署清单、Prometheus 指标名称或安全边界的非预期 diff。
