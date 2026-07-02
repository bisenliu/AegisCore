## ADDED Requirements

### Requirement: session lifecycle 测试使用生成 mock 表达端口契约

`auth-session-management` 的 `user-service/internal/features/auth/application/sessions` 包测试 MUST 使用已有 gomock 生成物或真实领域值验证 session lifecycle 端口交互，不得在 `lifecycle_test.go` 中继续保留 `sessionUserTestStore`、`authSessionTestStore` 或 `tokenVersionRecordingInvalidator` 手写兼容路径。测试 MUST 通过 expectation 明确表达 token version 回源、refresh session 旋转、全量撤销和本地 token version cache 失效语义。

#### Scenario: token version 回源测试

- **WHEN** session lifecycle 测试覆盖 Redis token version 投影 miss 后回源 PostgreSQL 并回填投影的路径
- **THEN** 测试 MUST 使用生成的 `UserTokenVersionStore` 和 `TokenVersionCache` mock 表达读取、回源和回填 expectation
- **AND** 测试 MUST NOT 使用手写 store 状态字段替代这些端口调用断言

#### Scenario: refresh session 旋转测试

- **WHEN** session lifecycle 测试覆盖 refresh session rotation 创建新 session 并替换旧 session 的路径
- **THEN** 测试 MUST 使用生成的 `RefreshSessionStore` mock 表达 session 参数、rotation 结果和失败分支 expectation
- **AND** 测试 MUST 使用 matcher 或 `DoAndReturn` 捕获需要校验的领域值

#### Scenario: 全量撤销和本地缓存失效测试

- **WHEN** session lifecycle 测试覆盖全部会话退出、token version 提升、Redis token version 投影刷新和本地 cache 失效
- **THEN** 测试 MUST 使用生成的 store mock 和 `TokenVersionLocalInvalidator` mock 表达调用顺序、失败信号和 cache invalidation expectation
- **AND** 测试 MUST NOT 通过 `tokenVersionRecordingInvalidator` 记录字段表达本地 cache 失效
