## 1. 固化审计基线与清理边界

- [x] 1.1 使用 `rg`、`go list` 和生产/测试引用统计重新枚举 `common/`、`user-service/`、`tools/` 中的 `mock_generate.go`、test-only 命名、仅测试引用导出 symbol、nil/default 测试兜底和手写 fake/stub，确认 23 个生成入口及 design 表中的高置信候选仍与当前代码一致。
- [x] 1.2 对每个新增候选检查 `openspec/specs/`、`docs/ARCHITECTURE.md`、生产调用方和 Git 历史；只纳入同时满足四重证据且不扩大 capability 范围的项，并在 `design.md` 更新删除或保留理由。
- [x] 1.3 记录并复核明确保留项：Casbin 白名单与 `OPTIONS` 旁路、Bearer refresh 输入、response wrapper、request ID/metrics/logger/pprof fallback、Ent SQL 局部时间源、协议模拟器、并发记录器和同步执行 fixture，确保后续 diff 不改变这些行为。

## 2. 隔离 mock 生成入口

- [x] 2.1 为 `common/http/middleware/mock_generate.go`、`common/security/casbin/mock_generate.go` 增加 `//go:build generate`，保持原 package 归属、目标 `_test.go` 文件和 mockgen 接口列表不变。
- [x] 2.2 为 `user-service/cmd/mock_generate.go` 及 auth 6 个、permission 7 个、role 4 个、user 3 个 `mock_generate.go` 增加 `//go:build generate`，不得移动为仓库级集中生成脚本。
- [x] 2.3 运行 `make common-generate` 和 `make user-service-generate`，检查全部 mock 可重新生成；使用 `go list` 或门禁测试确认普通 package `GoFiles` 不包含 `mock_generate.go`，并检查生成物 diff 只包含预期变化。

## 3. 收紧 CLI 依赖注入边界

- [x] 3.1 在 `user-service/cmd/main_test.go` 或测试 helper 中新增完整 `testRootCommandDependencies` fixture，为未被目标测试覆盖的 runner/factory 提供会明确失败的 inert 实现，避免误执行真实 bootstrap、RBAC 或 fxgraph 逻辑。
- [x] 3.2 更新 `user-service/cmd/main_test.go` 和 `user-service/cmd/rbac_test.go`，所有 `newRootCommand` 调用从完整 fixture 开始并只覆盖当前测试目标；保持 command graph、flag 默认值、env、参数归一化和错误传播断言不变。
- [x] 3.3 从 `user-service/cmd/main.go` 删除 `rootCommandDependencies.withDefaults` 和 `runServe` 的 nil `appFactory` fallback，保留 `defaultRootCommandDependencies`、runner factory、`lifecycleAppFactory` 和优雅关闭语义。
- [x] 3.4 运行 `(cd user-service && go test ./cmd)`，确认 serve、RBAC seed、assign-super-admin、create-super-admin、fxgraph 和 healthcheck 当前契约全部通过。

## 4. 缩小生产公开 API

- [x] 4.1 将 `common/runtime/workerpool.NewUnmanaged` 改为包内 `newUnmanaged`，更新 `New` 与 `pool_test.go` fixture，保持参数校验、Fx stop hook、blocking backpressure、stats、panic/error 记录和 `Stop` drain/timeout/幂等语义不变。
- [x] 4.2 删除 `user-service/internal/features/permission/infrastructure/redis.NewStoreWithInstance`，让同包 watcher/store 测试通过 `newStore(client, MustKeyCatalog(appName), instanceID, log)` 构造确定性实例，保持 policy version、Pub/Sub payload、instance ID 和 watcher 补偿语义不变。
- [x] 4.3 运行 `(cd common && go test ./runtime/workerpool)` 和 `(cd user-service && go test ./internal/features/permission/infrastructure/redis)`，并使用 `rg` 确认两个旧导出 symbol 已无定义或引用且未增加兼容 alias。

## 5. 重构宽接口测试替身

- [x] 5.1 在 auth credentials 消费包的 generate-only 入口中加入 `PasswordService` mock 生成指令，重新生成 mock，并将 `verifier_test.go` 的 `stubPasswordService` 替换为明确的 `VerifyContext`/`HashContext` expectation；保留真实 `password.Service` 的 Argon2id 成功路径测试。
- [x] 5.2 重构 `user-service/internal/features/auth/fx_test.go` 的 `fakeTokenVersionStore`、`fakeAuthStore` 和无关 `not implemented` 方法，优先复用 application port 生成 mock或更窄的局部 fixture，明确 disabled cache 回源、Redis miss、回填、失效和 stats expectation。
- [x] 5.3 复核其余手写 test double：只替换隐藏关键端口调用/失败顺序的宽接口 fake；保留 datastore 协议模拟器、scheduler 并发记录器、OTel exporter、同步 purge executor、只读 stats source 和真实轻量 service，并在 `design.md` 记录任何与初始分类不同的结论。
- [x] 5.4 运行 `(cd user-service && go test ./internal/features/auth/application/credentials ./internal/features/auth)`，并补跑受 mock 指令变化影响的 auth package 测试，确认 dummy hash、KDF busy、token version 和 Fx wiring 行为不变。

## 6. 增加防回归门禁与文档

- [x] 6.1 扩展 `user-service/scripts/architecture-lint.sh` 或仓库现有交付检查，拒绝缺少 `//go:build generate` 的 `mock_generate.go`，并覆盖 `common/` 与 `user-service/` 的全部人工维护 package。
- [x] 6.2 增加 architecture lint fixture/测试，证明未隔离的 mock 生成文件以及 `ForTest`、`set*ForTest`、`testHook` 等明确 test-only 正式 symbol 会被定位并拒绝，同时避免匹配 `_test.go`、`common/testing` 和 Ent/OpenAPI 生成物。
- [x] 6.3 更新 `docs/TESTING.md`，说明 generate-only mock 入口、完整测试依赖 fixture、生成 mock 与应保留的协议/并发/真实 adapter double 选择规则；不得新增共享 testing facade 或兼容 helper。
- [x] 6.4 运行 `make user-service-architecture-lint` 和相关 lint fixture 测试，确认现有合法 fallback、hook、fixture 与生成物不产生误报。

## 7. 规格与聚焦验证

- [x] 7.1 运行 `openspec validate remove-test-only-production-adapters`、`openspec list --specs` 和 `openspec validate --specs`，修复 proposal、design、delta specs 与 tasks 的不一致。
- [x] 7.2 运行 `make common-test`、`make user-service-test` 和 `make test`，确认共享 primitive、CLI、auth、permission、role、router 与 E2E harness 的核心测试仍通过；容器测试未运行时记录所需 `AEGISCORE_TEST_CONTAINERS=1`、Docker 或兼容容器运行时前置条件。
- [x] 7.3 运行 `make common-generate`、`make user-service-generate` 和 `make user-service-openapi-generate`，确认 mock、Ent 和 OpenAPI 生成链路无非预期 drift；本 change 不应产生 Ent schema、Atlas migration、OpenAPI 契约或部署资产变化。
- [x] 7.4 使用 `rg` 复扫非 `_test.go` 文件中的 test-only 命名、测试专用注释、nil/default 兜底和兼容关键词，对所有剩余命中逐项记录真实运行时或规格依据，并输出实施变更摘要所需的删除、重构和保留清单。

## 8. 暂存与全量交付验证

- [x] 8.1 检查 `git diff --check`、`git status --short` 和生成物 diff，只将本 change 的代码、测试、文档、规格与预期生成物加入暂存区，不纳入用户的无关修改。
- [x] 8.2 在本次预期变更已暂存后运行 `make lint`；未通过时修复并重新暂存，直到全部 lint 和 architecture lint 通过。
- [x] 8.3 在本次预期变更已暂存后运行 `make verify`，确认最终 `git diff --exit-code` 无未暂存生成 drift；任何测试、生成、lint 或 verify 未通过时不得将本 change 标记完成。
- [x] 8.4 输出最终变更摘要，逐项说明删除/降级的生产代码、隔离的生成入口、重构的测试、明确保留的业务边界及理由，并确认 HTTP API、CLI、数据库、OpenAPI、部署、观测和安全行为未发生非预期变化。
