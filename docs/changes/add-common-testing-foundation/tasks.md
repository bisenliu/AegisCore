# Tasks

## Implementation

- [x] 新增 `common/testing/containers/` 目录。
- [x] 新增 `common/testing/containers/postgres.go`，提供 PostgreSQL 测试容器启动 helper、默认配置、DSN/config 输出和清理逻辑。
- [x] 新增 `common/testing/containers/redis.go`，提供 Redis 测试容器启动 helper、默认配置、addr/config 输出和清理逻辑。
- [x] 为容器 helper 设计启用开关，例如 `AEGISCORE_TEST_CONTAINERS=1`，并明确未启用时测试跳过策略。
- [x] 为容器 helper 添加启动超时、连接等待、错误上下文和失败路径清理。
- [x] 如引入 testcontainers-go，更新 `common/go.mod` 和 `common/go.sum`，并确认生产 runtime 包没有导入测试容器依赖。
- [x] 新增 `common/testing/fixtures/` 目录。
- [x] 新增 `common/testing/fixtures/faker.go`，提供唯一后缀、用户名、邮箱、名称、UUID 字符串等无业务语义 helper。
- [x] 确认 `fixtures` 不导入 `user-service`、Ent、Gin、Redis client 或业务 domain 包。
- [x] 新增 `common/testing/fixtures/faker_test.go`，覆盖基础格式、唯一性和并行使用稳定性。
- [x] 新增 `common/testing/containers` 的基本 example 或 integration-gated test，展示 PostgreSQL helper 用法。
- [x] 新增 `common/testing/containers` 的基本 example 或 integration-gated test，展示 Redis helper 用法。
- [x] 可选：在 `user-service` 添加一个 integration-gated 示例测试或文档片段，证明服务模块可导入 `github.com/aegiscore/common/testing/containers`。
- [x] 不迁移所有现有测试；只迁移低风险、明显复用收益的容器启动逻辑或保留为后续迁移。
- [x] 运行 `gofmt -w` 处理新增或修改的 Go 文件。
- [x] 更新 `docs/TESTING.md`，补充 integration/e2e 测试、外部依赖、启用开关和 Docker 不可用策略。
- [x] 更新 `docs/ARCHITECTURE.md` 中 `common/testing` 的职责说明。
- [x] 必要时更新根目录 `AGENTS.md` 中 `common/` Repository Shape 描述。

## Verification

- [x] 在 `common/` 执行 `go test ./...`，确认普通测试不依赖 Docker。
- [x] 如 user-service 有示例或导入变化，在 `user-service/` 执行 `go test ./...`。
- [x] 在可用 Docker 环境中执行 `AEGISCORE_TEST_CONTAINERS=1 go test ./...` 或文档指定的 integration 命令。
- [x] 检查 PostgreSQL helper 返回的 DSN/config 可被 `database/sql` 或现有 datastore helper 使用。
- [x] 检查 Redis helper 返回的 addr/config 可被 `go-redis` 或现有 datastore helper 使用。
- [x] 检查容器 helper 在失败时提供明确错误信息，并通过 `t.Cleanup` 释放资源。
- [x] 检查 fixtures 输出没有业务语义，不包含 user/auth/session 专用结构。
- [x] 运行 `rg -n "openspec|docs/opsx" .`，确认没有新增 OpenSpec/OPSX 工件。
- [x] 检查 `git diff`，确认没有业务逻辑、HTTP route、数据库 schema、migration 或 Redis key 的非预期变化。

## Review Notes

- [x] 确认 `common/testing/containers` 只承载测试基础设施，不承载服务业务 fixture。
- [x] 确认 `common/testing/fixtures` 只生成通用值，不生成用户、认证、会话、密码 hash、token 或 Ent entity。
- [x] 确认现有单元测试仍优先使用 fake、stub、SQLite 或 `miniredis`，真实容器仅用于需要外部依赖语义的 integration/e2e 测试。
- [x] 确认 `docs/TESTING.md` 没有要求普通 `make test` 必须安装 Docker。
- [x] 确认新增文档继续说明仓库不维护 OpenSpec/OPSX 工件。
