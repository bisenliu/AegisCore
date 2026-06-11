# Design

## Overview

本变更新增 `common/testing`，作为跨模块测试基础设施入口：

- `common/testing/containers`：启动和管理真实 PostgreSQL、Redis 测试容器，返回调用方可直接使用的连接信息。
- `common/testing/fixtures`：提供通用测试数据生成 helper，只包含无业务语义的值，例如唯一后缀、邮箱、用户名、短名称和 UUID 字符串。

`common/testing` 只服务测试代码。它可以依赖测试容器库、Redis/PostgreSQL client 配置类型和标准库 testing，但不得依赖 `user-service`、Ent schema、Gin controller、feature domain 或业务 DTO。服务内业务 fixture 仍保留在对应 feature 的测试文件或测试包内。

## Package Layout

目标目录：

```text
common/testing/
  containers/
    postgres.go
    redis.go
    containers_test.go
  fixtures/
    faker.go
    faker_test.go
```

如果测试容器依赖导致普通 `go test ./...` 必须下载或连接 Docker，可把真实容器测试拆到带 build tag 的文件，例如 `containers_integration_test.go`，但 helper 源码本身仍位于上述包路径。

## Containers API

`containers` 包提供面向 Go test 的小 API，隐藏 testcontainers 细节。建议形态如下：

```go
package containers

type PostgresOptions struct {
    Image    string
    Database string
    Username string
    Password string
}

type PostgresContainer struct {
    Host     string
    Port     string
    Database string
    Username string
    Password string
    DSN      string
}

func StartPostgres(ctx context.Context, t testing.TB, opts PostgresOptions) *PostgresContainer
```

Redis helper 建议形态如下：

```go
type RedisOptions struct {
    Image string
    DB    int
}

type RedisContainer struct {
    Addr string
    DB   int
}

func StartRedis(ctx context.Context, t testing.TB, opts RedisOptions) *RedisContainer
```

具体返回结构可以根据实现需要补充方法，例如：

- `PostgresContainer.URL() string`
- `PostgresContainer.Config() config.PostgresConfig`
- `RedisContainer.Options() *redis.Options`
- `RedisContainer.Config() config.RedisConfig`

返回 `config.PostgresConfig` 和 `config.RedisConfig` 是允许的，因为这些是 `common/runtime/config` 的共享运行时配置类型；但 `containers` 不应返回服务内 bootstrap params 或 Ent client。

## Defaults And Skipping

默认值应稳定、显式、可覆盖：

- PostgreSQL 默认镜像使用当前 Atlas dev database 接近的主版本，例如 `postgres:15-alpine`。
- Redis 默认镜像使用当前运行时兼容版本，例如 `redis:7-alpine`。
- 默认数据库名、用户名、密码使用测试专用值，例如 `aegiscore_test`、`aegiscore`、`secret`。
- 默认超时应短而明确，例如 30 秒启动超时，避免 Docker 不可用时长时间挂起。

integration 容器测试不应影响普通单元测试稳定性。推荐采用环境变量启用：

```text
AEGISCORE_TEST_CONTAINERS=1 go test ./...
```

当未启用该变量时，真实容器 example/test 可调用 `t.Skip`。当变量已启用但 Docker/testcontainers 不可用时，helper 应用清晰错误失败，避免误以为集成验证已执行。文档中需要明确这一区分。

## Lifecycle And Cleanup

helper 必须接收 `testing.TB` 并执行：

- `t.Helper()` 标记调用栈。
- 基于传入 ctx 派生启动超时。
- 等待服务可连接，例如 PostgreSQL `sql.Open` + `PingContext`，Redis `PING`。
- 使用 `t.Cleanup()` 关闭 client/pool 或终止容器。
- 清理失败时使用 `t.Errorf` 或 `t.Fatalf` 给出容器名和错误上下文。

如果容器启动过程中创建了中间资源，应保证失败路径也会释放已创建容器。

## Fixtures API

`fixtures` 包只提供跨模块通用值，不表达业务状态。建议 API：

```go
package fixtures

type Faker struct {
    suffix string
}

func NewFaker(t testing.TB) *Faker
func (f *Faker) UniqueSuffix() string
func (f *Faker) Username(prefix string) string
func (f *Faker) Email(prefix string) string
func (f *Faker) Name(prefix string) string
func (f *Faker) UUIDString() string
```

约束：

- 生成值应适合并行测试，默认带唯一后缀。
- 输出应可读，便于测试失败时定位。
- 不生成 `user-service` 专用字段组合，例如用户创建 command、认证 session payload、密码 hash、token version 或 Ent model。
- 不依赖随机全局状态导致 flaky；如果使用随机数，应结合 test name 或 UUID 并保持格式稳定。

## Optional User-Service Adoption

本变更的目标是让 `user-service` 可选择性复用 helper，而不是大规模改写现有测试。

可以优先添加一个低风险示例：

- 在 `user-service` 新增或调整一个 integration-only 示例测试，导入 `github.com/aegiscore/common/testing/containers` 启动 Redis 或 PostgreSQL，并验证可连接。
- 或在文档中给出 user-service 使用片段，实际迁移留给后续需要真实容器的 feature 变更。

现有测试是否迁移的建议：

- `app` 和 `domain` 单元测试继续使用 stub/fake。
- Redis adapter 命令语义测试可以继续用 `miniredis`，只有需要真实 Redis 行为差异时再用容器。
- PostgreSQL adapter 当前 SQLite 覆盖可移植 Ent 查询语义；只有需要 PostgreSQL-specific SQL、migration 或 constraint 行为时再用容器。
- bootstrap lifecycle 测试可以继续用 fake driver/listener，除非目标是验证真实连接配置。

## Documentation Updates

`docs/TESTING.md` 增加 integration/e2e 章节：

- 普通 `go test ./...` 不要求 Docker。
- 需要真实 PostgreSQL/Redis 的测试使用 `common/testing/containers`。
- 使用 `AEGISCORE_TEST_CONTAINERS=1` 或约定 build tag 启用容器测试。
- Docker 不可用时的 skip/fail 规则。
- 业务 fixture 不进入 `common/testing`。

`docs/ARCHITECTURE.md` 的 Common Organization 增加：

- `common/testing`：跨模块测试基础设施和无业务语义 fixture，仅供测试使用。

根目录 `AGENTS.md` 的 Repository Shape 可同步补充 `testing` 分类，继续强调 `common` 不作为服务特定 helper 兜底目录。

## Dependency Considerations

实现时可以引入 testcontainers-go 相关依赖到 `common/go.mod`。应尽量把重量级依赖限制在 `common/testing/containers`，避免生产 runtime 包导入测试容器库。

如果项目希望避免生产模块依赖测试容器库，可以使用以下策略之一：

- 将容器 helper 文件保持在普通源码中，但仅被测试包导入；接受 `common` module 的 test helper 依赖。
- 使用 build tag 隔离容器 helper 与测试，例如 `//go:build integration`，同时提供文档说明。
- 优先使用 `testcontainers-go/modules/postgres` 与 `testcontainers-go/modules/redis`，减少手写 wait strategy。

最终选择应以 `make test` 和普通开发体验稳定为准。

## Verification Strategy

- `common/testing/fixtures`：
  - 测试唯一后缀非空。
  - 测试用户名、邮箱、名称格式稳定且包含前缀。
  - 测试并行调用不产生明显冲突。
- `common/testing/containers`：
  - 未启用 integration 时，真实容器测试稳定 skip。
  - 启用 integration 时，PostgreSQL helper 返回可 ping 的 DSN/config。
  - 启用 integration 时，Redis helper 返回可 ping 的 addr/config。
  - 清理逻辑通过 `t.Cleanup` 注册，不泄漏 client/container。
- 模块验证：
  - 在 `common/` 执行 `go test ./...`。
  - 如有 user-service 示例导入，在 `user-service/` 执行 `go test ./...`。
  - 在有 Docker 的环境下执行 `AEGISCORE_TEST_CONTAINERS=1 go test ./...` 或文档指定命令。
- 文档验证：
  - `docs/TESTING.md` 明确 integration/e2e 外部依赖策略。
  - 确认没有新增 `openspec/` 或 `docs/opsx/`。
