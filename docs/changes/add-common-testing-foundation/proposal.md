# Add common testing foundation

## What

新增 `common/testing`，沉淀跨模块可复用的测试基础设施，让 `common` 和 `user-service` 可以共享 PostgreSQL、Redis 和基础测试数据生成能力。

包括：

- 新增 `common/testing/containers/postgres.go`，提供 PostgreSQL 测试容器 helper。
- 新增 `common/testing/containers/redis.go`，提供 Redis 测试容器 helper。
- 新增 `common/testing/fixtures/faker.go`，提供无业务语义的基础测试数据生成 helper。
- 将当前可复用的测试容器启动、等待、连接信息提取和清理逻辑收敛到 `common/testing`。
- 为 `common/testing` 补充基本使用示例或测试，证明 helper 可被 Go test 直接消费。
- 更新 integration/e2e 测试说明，明确什么时候使用 `common/testing/containers`，什么时候继续使用轻量 fake、SQLite 或 `miniredis`。

本变更不要求所有现有测试立刻迁移到新 helper，也不把用户、认证、会话等业务 fixture 放入 `common`。

## Why

当前测试里已经存在多种基础设施替身：

- `common/runtime/datastore` 和 `user-service/internal/bootstrap` 中有用于 Redis ping/lifecycle 的测试 TCP server。
- `user-service` 的 Redis adapter 测试使用 `miniredis` 覆盖命令语义。
- `user-service` 的 PostgreSQL adapter 测试目前使用 Ent + SQLite 覆盖可移植查询语义。
- 未来真正需要 PostgreSQL/Redis 行为差异验证的 integration/e2e 测试，会重复处理容器启动、超时、DSN、清理和跳过策略。

把跨模块稳定的测试基础设施放到 `common/testing` 后，服务可以选择性复用同一套容器 helper，减少每个 feature 或模块自行维护 Docker/testcontainers 细节的成本。`common/testing` 只提供基础设施和通用数据原语，不拥有业务 fixture，避免把服务语义反向沉淀进共享模块。

## Scope

包括：

- 新增 `common/testing/containers` 包。
- 在 `containers/postgres.go` 中提供 PostgreSQL 容器启动 helper、连接配置输出、DSN 输出和清理封装。
- 在 `containers/redis.go` 中提供 Redis 容器启动 helper、地址输出、`redis.Options` 或 config 输出和清理封装。
- helper 应接收 `testing.TB` 和可选配置，使用 `t.Helper()`、`t.Cleanup()`、上下文超时和明确错误信息。
- helper 应支持在 Docker/testcontainers 不可用时按约定跳过 integration 测试，避免普通单元测试环境被外部依赖卡死。
- 新增 `common/testing/fixtures` 包。
- 在 `fixtures/faker.go` 中提供基础随机或确定性 helper，例如唯一后缀、邮箱、用户名、短名称、UUID 字符串等。
- fixtures 不依赖 `user-service`、Ent、Redis、Gin 或服务内 domain 包。
- 为 `common/testing` 添加基础测试或 example，展示 PostgreSQL 和 Redis helper 的调用方式；如果实际容器测试默认跳过，应保留可在 integration 环境启用的示例。
- 更新 `docs/TESTING.md`，补充 integration/e2e 外部依赖说明和 `common/testing` 使用边界。
- 必要时更新 `docs/ARCHITECTURE.md` 和根目录 `AGENTS.md`，把 `common/testing` 纳入 common 组织说明。

不包括：

- 不强制迁移所有现有测试。
- 不把用户资料、认证会话、密码、token、Ent entity seed 等业务 fixture 放入 `common/testing`。
- 不改变现有业务逻辑、HTTP route、Redis key、数据库 schema 或 migration。
- 不用容器 helper 替代所有轻量测试替身；单元测试仍应优先使用 fake、SQLite、`miniredis` 或接口 stub。
- 不新增 `openspec/` 或 `docs/opsx/` 工件。

## Acceptance Criteria

- 存在 `common/testing/containers/postgres.go`，提供可复用 PostgreSQL 测试容器 helper，并封装启动、等待、连接信息和清理。
- 存在 `common/testing/containers/redis.go`，提供可复用 Redis 测试容器 helper，并封装启动、等待、地址/config 输出和清理。
- 存在 `common/testing/fixtures/faker.go`，只包含跨模块通用、无业务语义的测试数据生成 helper。
- `common/testing` 有基本测试或 example，说明 helper 的使用方式和 integration 启用方式。
- `user-service` 可选择性导入 `github.com/aegiscore/common/testing/containers` 复用容器 helper，不需要复制容器启动逻辑。
- `docs/TESTING.md` 说明 integration/e2e 测试如何启用外部依赖、如何跳过 Docker 不可用场景，以及 `common/testing` 与轻量 fake 的取舍。
- `common/` 中 `go test ./...` 通过；如容器测试默认需要环境变量或 build tag，应在未启用 integration 时稳定跳过。
- 没有新增业务 fixture 到 `common`，没有新增 OpenSpec/OPSX 工件。
