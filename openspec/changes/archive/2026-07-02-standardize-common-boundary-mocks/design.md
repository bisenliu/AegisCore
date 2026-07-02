## Context

`user-service` 已通过 package-local `mock_generate.go`、`go tool mockgen` 和提交的生成物维护测试 mock；`common` 模块仍只有 `test`、`lint`、`verify` 目标，尚未暴露 `generate` 入口。当前 `common/security/casbin/authorizer_test.go`、`common/http/middleware/casbin_test.go` 和 `common/http/middleware/auth_test.go` 中的 recording double 主要用于记录调用次数、入参和注入错误，这类交互更适合由 gomock expectation 表达。

本 change 不引入新的业务能力，也不改变 `common` 的生产 interface。它只把已存在的边界 interface 测试替身迁移到可复现生成物，并让 common 的生成命令进入模块级和仓库级验证链路。

受影响路径：

- `common/security/casbin/authorizer_test.go`
- `common/security/casbin/mock_generate.go`
- `common/http/middleware/casbin_test.go`
- `common/http/middleware/auth_test.go`
- `common/http/middleware/mock_generate.go`
- `common/go.mod`、`common/go.sum`
- `common/Makefile`
- 根 `Makefile`
- `openspec/changes/standardize-common-boundary-mocks/`

不影响 `user-service` 业务代码、HTTP API、数据库 migration、OpenAPI 生成物、部署清单或观测资产。

## Goals / Non-Goals

**Goals:**

- 为 `common/security/casbin.Enforcer`、`common/http/middleware.CasbinAuthorizer` 和 `common/security/auth.TokenVersionValidator` 建立可复现 mockgen 入口。
- 删除纳入范围的手写 recording double，并在测试中使用 generated mock 表达授权、token version 校验的调用次数、入参和错误路径。
- 在 `common/Makefile` 暴露 `generate` 和带 drift 校验的 `verify`，在根 `Makefile` 暴露 `common-generate` 并纳入完整 `make verify`。
- 保持生成物 package-local，不创建 `common/mocks`、全局 `mocks/` 或中央 mock 仓库。

**Non-Goals:**

- 不改变 `common/security/casbin.Enforcer`、`common/http/middleware.CasbinAuthorizer` 或 `common/security/auth.TokenVersionValidator` 的生产接口语义。
- 不改变 JWT 解析、token version mismatch、Casbin 三元组授权、HTTP middleware 响应、日志字段或错误映射。
- 不迁移 scheduler 等需要状态型内存 harness 的测试对象。
- 不移动或重写已有 `user-service` mock 生成物。

## Decisions

### Decision: 复用 `go tool mockgen`

`common` 将采用与 `user-service` 一致的 `go:generate go tool mockgen ...` 形式，并在 `common/go.mod` 中声明 `go.uber.org/mock/mockgen` tool 依赖。

理由：该方式由 Go module 记录工具版本，不依赖开发者机器的全局 `mockgen`，并能被 `go generate ./...` 复现。备选方案是要求安装全局 `mockgen` 或用 `go run go.uber.org/mock/mockgen@...`，前者不可复现，后者会把版本约束分散到注释和命令中。

### Decision: mock 生成物留在消费侧 package

`common/security/casbin` 的 `Enforcer` mock 留在 `common/security/casbin` 测试包内；`common/http/middleware` 消费的 `CasbinAuthorizer` 与 `auth.TokenVersionValidator` mock 留在 `common/http/middleware` 测试包内。

理由：测试只需要替换本 package 的边界依赖，package-local mock 能避免跨包测试辅助对象成为共享 API。备选方案是创建 `common/mocks` 或集中测试包，但这会违反 common 的业务中立边界，并增加跨 package 测试耦合。

### Decision: 只迁移 interaction-style recording double

本次只迁移用于调用记录、入参断言和错误注入的 `recordingEnforcer`、`recordingCasbinAuthorizer` 和 `recordingTokenVersionValidator`。状态型测试 harness、复杂内存 fixture 或比 expectation 更清晰的领域测试对象不纳入本 change。

理由：gomock 对简单协作者交互更清晰，但不应为了生成 mock 而扁平化复杂状态测试。备选方案是迁移所有手写测试对象，这会扩大风险并可能降低部分测试可读性。

### Decision: common 生成纳入模块级和仓库级验证

`common/Makefile` 增加 `generate`，`verify` 依赖 `lint generate test` 并通过 `git diff --exit-code` 暴露 drift；根 `Makefile` 增加 `common-generate`，完整 `verify` 在 `user-service-generate` 之外执行 common 生成。

理由：只新增 `go:generate` 不足以防止生成物过期；完整验证必须能重新生成并发现 diff。备选方案是只要求人工执行 `go generate ./...`，但这无法形成稳定交付门禁。

## Risks / Trade-offs

- [Risk] gomock expectation 可能让表格测试更啰嗦 -> Mitigation：只迁移简单 interaction-style double，并在测试 helper 中集中构造 mock controller 或公共请求。
- [Risk] 新增 common `generate` 后完整验证耗时增加 -> Mitigation：当前范围只包含少量 mockgen 入口，成本小于生成物 drift 的维护风险。
- [Risk] `git diff --exit-code` 会在未暂存预期变更时失败 -> Mitigation：tasks 明确要求实现完成、清理临时文件并暂存本次预期变更后再执行最终 `make verify`。
- [Risk] mock 文件放错位置会形成新的共享测试 API -> Mitigation：spec 明确禁止 `common/mocks`、全局 `mocks/` 和中央 mock 仓库。

## Migration Plan

1. 在 `common` 增加 mockgen tool 依赖与 package-local `mock_generate.go`。
2. 运行 `make common-generate` 或 `make -C common generate` 生成 mock 文件。
3. 将目标测试中的 recording double 替换为 generated mock expectation，并删除对应手写类型。
4. 更新 `common/Makefile` 与根 `Makefile`，使 common 生成物进入模块级和仓库级 drift 校验。
5. 运行 `make common-test`、`make common-verify`；暂存本次预期变更后运行 `make verify`。

回滚方式：撤回本 change 的 common mock 生成入口、生成物、测试改造、Makefile 目标和 go.mod/go.sum 工具依赖，恢复原 recording double。由于不涉及数据库、HTTP API 或部署资产，不需要运行时迁移或发布顺序调整。

## Open Questions

无。
