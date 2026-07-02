## ADDED Requirements

### Requirement: common 边界 mock 生成规范

系统 MUST 为 `common` 中适合 expectation 表达的边界 interface 测试 double 提供 package-local mockgen 生成入口。生成 mock MUST 仅用于对应 package 的测试，系统 MUST NOT 创建 `common/mocks`、全局 `mocks/`、`testmocks/` 或等价中央 mock 仓库。

#### Scenario: common 授权 enforcer 测试使用生成 mock

- **WHEN** `common/security/casbin` 测试需要模拟 `Enforcer` 的允许、拒绝、错误或调用参数
- **THEN** 测试 MUST 使用该 package 内的 `go.uber.org/mock/mockgen` 生成 mock 表达 expectation
- **AND** 测试 MUST NOT 保留被迁移的手写 recording double 兼容路径

#### Scenario: common HTTP middleware 测试使用生成 mock

- **WHEN** `common/http/middleware` 测试需要模拟 `CasbinAuthorizer` 或 `auth.TokenVersionValidator`
- **THEN** 测试 MUST 使用 `common/http/middleware` package-local 生成 mock 表达调用次数、入参和错误返回
- **AND** 生成 mock MUST NOT 作为跨 package 共享测试 API 暴露

#### Scenario: 保持 common 生产语义不变

- **WHEN** 测试 double 迁移为 generated mock
- **THEN** 系统 MUST 保持 Casbin 三元组授权、`ErrNotConfigured`、`ErrDenied`、JWT 解析、token version mismatch、HTTP 响应和日志语义不变
- **AND** 系统 MUST NOT 仅为测试新增业务无关生产接口、adapter、分支或 `NewXForTest` 入口

#### Scenario: 状态型测试 harness 不强制迁移

- **WHEN** `common` 测试对象需要复杂内存状态、并发协调、scheduler 生命周期或比 expectation mock 更清晰的测试夹具
- **THEN** 测试 MAY 保留 package-local 状态型 harness
- **AND** 该 harness MUST NOT 被迁移到中央 mock 仓库或导出为跨 package 测试依赖
