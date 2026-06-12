# Design

## Overview

本变更分为两条主线：

1. 收紧用户服务数据库边界：删除 `common_db` 预留通道，让用户服务运行时只连接自己拥有的 `user_db`。
2. 补齐 CI/release 门禁：在已有 lint 和 migration workflow 之外，增加测试、构建、镜像、安全扫描和 SBOM。

设计原则：

- `common/runtime/datastore` 仍保持通用 named datastore primitive，不承载业务语义。
- 服务级资源名只能表达当前服务真实使用的资源，不预置“公共业务库”。
- 共享数据能力如果未来确有需求，应另开变更，通过明确 owner 的 HTTP/gRPC API、事件契约或 feature-owned port + adapter 表达，而不是暴露共享 DB/Ent client。
- CI 门禁优先复用现有 Makefile 和官方/主流 GitHub Actions，避免把命令逻辑复制到多个地方。

## Current Baseline

### Shared DB Path

当前用户服务代码包含以下共享数据库预留路径：

- `common/runtime/resources/resource_names.go` 定义 `NameCommonDB = "common_db"`。
- `user-service/internal/providers/postgres.go` 调用 `datastore.NewPostgresPools(..., resources.NameUserDB, resources.NameCommonDB)`。
- `user-service/internal/providers/ent.go` 从 `name:"common_db"` SQL pool 构造 `name:"common_db"` Ent client。
- `user-service/configs/config.yaml` 包含 `postgres.common_db`，并注释为“公共业务库”。
- `docs/ARCHITECTURE.md`、`docs/DEVELOPMENT.md` 和 `docs/TESTING.md` 均描述用户服务当前声明 `common_db`。

真实业务 adapter 只消费 `user_db`：

- `user-service/internal/features/user/infrastructure/postgres/user_store.go` 注入 `` *ent.Client `name:"user_db"` ``。
- `user-service/internal/features/auth/infrastructure/postgres/credential_store.go` 注入 `` *ent.Client `name:"user_db"` ``。

### CI Gates

当前 `.github/workflows` 只有：

- `lint.yml`：运行 golangci-lint，已经是硬门禁。
- `migrations.yml`：运行 `make migrate-validate`。

本地验证显示 `make lint-user-service` 当前失败：

```text
user-service/internal/bootstrap/validation_test.go:205:39: unused-parameter: parameter 'dsn' seems to be unused, consider removing or renaming it as _ (revive)
```

`Makefile` 已提供可复用入口：

- `make test`
- `make build`
- `make lint`
- `make migrate-validate`
- `docker build -f deployments/docker/user-service.Dockerfile -t aegiscore-user-services .`

## Database Boundary Changes

### Resource Names

`common/runtime/resources/resource_names.go` 目标形态：

```go
const (
    // NameUserDB 是用户数据库的具名 datastore 和 Ent 资源名。
    NameUserDB = "user_db"
    // NameCacheRedis 是缓存型运行时状态使用的具名 Redis 资源名。
    NameCacheRedis = "cache_redis"
)
```

`NameCommonDB` 应删除。`common/runtime/datastore` 测试如需验证多 PostgreSQL pool，应在测试内使用局部字符串常量，例如：

```go
const testAuditDB = "audit_db"
```

这能继续覆盖 neutral primitive 的多实例能力，同时避免在公共资源名包里预置业务语义。

### PostgreSQL Provider

`user-service/internal/providers/postgres.go` 只请求 `resources.NameUserDB`。

目标结构：

```go
type NamedPostgresPools struct {
    fx.Out

    UserDB *sql.DB `name:"user_db"`
}
```

`ProvidePostgresPools` 只返回 `UserDB`，并且缺失 `postgres.common_db` 不报错。可以保留测试证明 `pay_db` 或 `common_db` 不会被提供为 Fx named output。

### Ent Provider

`user-service/internal/providers/ent.go` 只从 `name:"user_db"` SQL pool 构造 Ent client：

```go
type NamedEntClientParams struct {
    fx.In

    Lifecycle fx.Lifecycle
    Log       *zap.Logger
    UserDB    *sql.DB `name:"user_db"`
}

type NamedEntClients struct {
    fx.Out

    UserClient *ent.Client `name:"user_db"`
}
```

`ProvideEntClients` 的 `OnStop` 只关闭 user Ent client。`closeEntClients` 可以简化为单 client close helper，也可以保留一个更通用的 helper，但不应再硬编码 `common_db`。

### Tests

Provider 测试应从“提供 user 和 common 数据库”改为“只提供用户服务数据库”：

- `TestProvidePostgresPoolsProvidesUserDatabase`
- `TestProvidePostgresPoolsDoesNotRequireCommonDBConfig`
- `TestProvidePostgresPoolsDoesNotProvideCommonDatabase`
- `TestProvidePostgresPoolsDoesNotProvidePayDatabase`
- `TestProvideEntClientsProvidesUserServiceEntClient`
- `TestPostgresPoolsAndEntClientsClosePoolOnce`

`common/runtime/datastore` 的多 pool lifecycle 测试仍可保留，但改用测试局部 name，不依赖 `resources.NameCommonDB`。

`user-service/internal/bootstrap/validation_test.go` 中测试 driver 的签名应改为：

```go
func (d *appModuleTestSQLDriver) Open(_ string) (driver.Conn, error)
```

### Config And E2E Harness

`user-service/configs/config.yaml` 删除 `postgres.common_db` 配置块和“公共业务库”注释。

`user-service/tests/e2e/harness_test.go` 的 YAML 模板只输出 `postgres.user_db`。格式化参数列表应同步删除第二组 PostgreSQL 参数，避免模板和参数错位。

### Documentation

同步更新：

- `AGENTS.md`
- `docs/ARCHITECTURE.md`
- `docs/DEVELOPMENT.md`
- `docs/TESTING.md`

目标描述：

- 用户服务当前声明并连接 `postgres.user_db`。
- 配置文件中出现其他 PostgreSQL 命名实例不代表用户服务会自动连接或迁移它们。
- 迁移目标始终是用户服务拥有的 `user_db`。
- 不再把 `common_db` 描述为当前资源或测试期望。

历史 `docs/changes/*` 已完成变更记录不需要批量改写；它们可以保留当时背景。

## CI Gate Design

### Workflow Shape

建议新增 `.github/workflows/ci.yml` 承载 test/build/image/race/coverage/security/sbom，保留现有：

- `.github/workflows/lint.yml`
- `.github/workflows/migrations.yml`

也可以拆分为 `test.yml`、`build.yml`、`security.yml`。如果拆分，应保持 job 名清晰且避免重复安装成本失控。

### Common Setup

Go jobs 使用：

```yaml
- uses: actions/setup-go@v5
  with:
    go-version: '1.26.3'
    cache: true
    cache-dependency-path: |
      common/go.sum
      user-service/go.sum
```

`permissions` 默认 `contents: read`。需要上传 CodeQL/SARIF 时，security job 可额外声明 `security-events: write`。

### Test Job

运行：

```bash
make test
```

该 job 不应要求 Docker 或真实 PostgreSQL/Redis；E2E 容器测试仍由显式环境变量触发，后续可单独加 nightly 或手动 workflow。

### Build Job

运行：

```bash
make build
```

可上传 `bin/user-service` 作为短期 artifact，但不应把本地 build artifact 提交到仓库。

### Docker Image Job

运行：

```bash
docker build -f deployments/docker/user-service.Dockerfile -t aegiscore-user-services .
```

该 job 验证 Dockerfile、build context、`go.work`、`common/` 和 `user-service/` 的复制路径一致。

### Race Job

分模块执行：

```bash
cd common && go test -race ./...
cd user-service && go test -race ./...
```

如果运行时间过长，先保留为 PR/push 门禁；后续再根据实际时长评估是否改成 main push 或 scheduled。

### Coverage Job

分模块生成 coverage：

```bash
cd common && go test -coverprofile=../coverage-common.out ./...
cd user-service && go test -coverprofile=../coverage-user-service.out ./...
```

第一阶段上传 artifact，不设置硬阈值，避免因为历史覆盖率未知阻断所有 PR。覆盖率阈值应单独通过变更设计。

### Govulncheck Job

使用官方 Go vulnerability action 或安装 `golang.org/x/vuln/cmd/govulncheck@latest`，分别执行：

```bash
cd common && govulncheck ./...
cd user-service && govulncheck ./...
```

为避免 latest 漂移，也可以固定版本；固定版本应在 workflow 注释里说明升级策略。

### Gosec Job

使用 `securego/gosec`，扫描两个 module。输出 SARIF 或 JSON artifact。若上传 SARIF，需要：

```yaml
permissions:
  contents: read
  security-events: write
```

第一阶段建议让 high severity 失败，medium/low 作为报告；具体阈值需在 workflow 中明确。

### Trivy Job

包含两个扫描：

- filesystem scan：扫描仓库依赖和配置。
- image scan：依赖 Docker image job 或在同 job 内构建镜像后扫描。

建议 high/critical vulnerability 失败。输出 table 到日志，并上传 SARIF 或 CycloneDX/SPDX artifact。

### SBOM Job

使用 Syft 或 Trivy 生成 SBOM：

```bash
syft dir:. -o cyclonedx-json=sbom-repository.cdx.json
syft aegiscore-user-services:latest -o cyclonedx-json=sbom-image.cdx.json
```

如果选择 Trivy，也可使用：

```bash
trivy fs --format cyclonedx --output sbom-repository.cdx.json .
trivy image --format cyclonedx --output sbom-image.cdx.json aegiscore-user-services
```

SBOM 作为 artifact 上传，不提交到仓库。

## Verification Strategy

本地验证：

```bash
make lint-user-service
make lint
make test
make build
docker build -f deployments/docker/user-service.Dockerfile -t aegiscore-user-services .
```

结构检查：

```bash
rg -n "NameCommonDB|common_db|name:\"common_db\"" common user-service AGENTS.md docs
rg -n "postgres\\.common_db|公共业务库" user-service/configs docs AGENTS.md
```

期望只在历史 `docs/changes/*` 或必要的测试说明中出现旧词；当前源码、配置样例、架构文档和测试文档不应再声明 `common_db`。

CI 验证：

- 新 workflow 在 PR 上成功运行。
- lint 和 migration workflow 保持现有硬门禁。
- test/build/image/race/security/SBOM jobs 的失败会使 workflow 失败；coverage 和 SBOM artifact 生成失败也应失败，覆盖率低本身第一阶段不失败。

## Risks

- 删除 `common_db` 会改变启动配置要求。当前没有真实 adapter 消费它，因此行为风险低；主要风险是测试和文档仍保留旧断言。
- `common/runtime/datastore` 测试若直接删除多 pool 覆盖，会降低 primitive 覆盖。应改用测试局部资源名保留覆盖。
- 新增安全扫描可能暴露历史依赖问题并阻断 PR。可先明确 high/critical 阈值，并把 medium/low 作为报告；如果 high/critical 已存在，应在本变更内修复或把 workflow 阈值设计成有时间盒的过渡方案。
- Race job 可能增加 CI 时间。若超时，应先拆分 module matrix，而不是取消门禁。
- Docker build job 需要 GitHub-hosted runner 支持 Docker。`ubuntu-latest` 默认可用，但本地验证可能因 Docker 未启动而无法执行，应在最终说明中记录。
