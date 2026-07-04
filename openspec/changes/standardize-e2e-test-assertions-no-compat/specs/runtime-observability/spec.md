## ADDED Requirements

### Requirement: E2E runtime harness 断言规范
系统 MUST 保持 user-service E2E harness 对运行时启动、配置、日志目录、Gin engine、HTTP request 构造和 response envelope 解码的现有语义，并 MUST 使用 `docs/TESTING.md` 规定的语义化断言表达 harness 前置条件和失败诊断。

#### Scenario: E2E 环境开关保持不变
- **WHEN** E2E harness 判断是否运行 HTTP flow 集成测试
- **THEN** 测试 MUST 保持当前 `AEGISCORE_TEST_E2E` 和通用 Testcontainers 开关语义
- **AND** 未启用容器测试时 MUST 继续跳过，而不是通过新断言改变运行前置条件

#### Scenario: runtime 启动和配置断言
- **WHEN** E2E harness 写入测试配置、分配本地端口、启动 PostgreSQL/Redis 容器、应用 migration、构造 Fx app 并填充 Gin engine
- **THEN** 测试 MUST 使用 `require.NoError`、`require.NotNil`、`require.NotEmpty`、`require.Greater` 或等价语义化断言表达前置条件
- **AND** 迁移 MUST NOT 改变日志配置、HTTP timeout、Redis/PostgreSQL 配置、Fx app 启停或 `bootstrap.AppModule` 装配语义

#### Scenario: response envelope 解码断言
- **WHEN** E2E harness 解码 HTTP response envelope、校验 status、`success`、应用错误码、message 和 `data`
- **THEN** 测试 MUST 使用语义化 `require` / 必要 `assert` 表达 JSON decode、字段值和空数据检查
- **AND** 迁移 MUST NOT 改变共享 response envelope、公开 message、HTTP status 或 JSON 字段名称
