# Go Lint 自动化整改方案

## Goal

仓库使用 `golangci-lint` 统一执行 Go 格式、导入顺序、未处理错误、静态分析、风格检查和架构依赖边界检查。Lint 是本地开发和 CI 合并门禁的一部分。

## Layout

```text
.
  .golangci.yml
  common/
    go.mod
  tools/
    openapi-convert/
      go.mod
    nacos-config-seed/
      go.mod
  user-service/
    go.mod
  docs/
    DEVELOPMENT.md
    GO_LINT_AUTOMATION.md
```

根 `.golangci.yml` 统一约束 `common/`、`user-service/`、`tools/openapi-convert/` 和 `tools/nacos-config-seed/`。CI 和本地开发应分别进入四个 Go module 执行；根目录不是单一 Go module，不应把根目录 `golangci-lint run ./...` 作为唯一命令。

## Local Commands

```bash
go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2
go install github.com/securego/gosec/v2/cmd/gosec@v2.28.0
make lint
make common-lint
make user-service-lint
make tools-openapi-convert-lint
make tools-nacos-config-seed-lint
```

直接排查时可分别执行：

```bash
cd common && golangci-lint run ./...
cd user-service && golangci-lint run ./...
cd tools/openapi-convert && golangci-lint run ./...
cd tools/nacos-config-seed && golangci-lint run ./...
```

## Architecture Lint

`depguard` 规则应镜像 `docs/ARCHITECTURE.md` 和 `openspec/specs/delivery-operations/spec.md` 的分层依赖表：

- `domain` 不得导入 Gin、Ent、Redis、runtime config/logger、response envelope、application ports 或 infrastructure adapter。
- `application` 不得导入 Gin、Ent、Redis 或 `common/http/binding`。
- `transport/http` 不得导入 Ent、Redis 或 SQL/pgx package。
- `transport/grpc` 不得导入 Ent、Redis、SQL/pgx、Gin controller、HTTP response envelope 或 external integration adapter。
- `infrastructure/*` 不得导入 Gin 或 HTTP response envelope。
- `integration/*` 不得导入 Gin response、Ent 或 feature persistence adapter。

Import 检查之外，code review 还应检查类型、构造函数、provider、handler、helper 的顺序是否服务阅读主线。Ent、OpenAPI 等生成代码不因排序手写调整。

## Generated Code Search

日常人工搜索、审查和 lint 诊断应把 Ent codegen 输出视为生成物，默认排除 `user-service/internal/persistence/ent/` 下除 `schema/` 之外的文件。该目录中的 `panic(err)`、builder、mutation、predicate、runtime hook 和 migration helper 由 Ent 生成流程维护，不作为手写业务代码 finding 处理。

人工编辑入口只包括：

- `user-service/internal/persistence/ent/schema/`
- `user-service/migrations/*.sql`
- `user-service/migrations/atlas.sum`

搜索工具建议使用排除参数聚焦人工代码，例如 `rg --glob '!user-service/internal/persistence/ent/**' --glob 'user-service/internal/persistence/ent/schema/**' 'panic\(err\)'`。若代码审查工具支持 generated-code filter，应将 `user-service/internal/persistence/ent/` 下除 `schema/` 之外的文件标记为 generated，并确保 `schema/`、SQL migration 和 `atlas.sum` 仍参与正常审查。

Architecture-lint 还必须检查 `openspec/specs/`、`openspec/changes/` 和 `docs/opsx/` 下的 Markdown 文档，拒绝保留默认英文 OpenSpec 模板占位、占位注释或非必要英文模板说明。OpenSpec 主规格、change artifacts 和 OPSX 相关文档的正文、标题、需求、场景、任务和说明必须使用简体中文；技术标识符、路径、命令、Go symbol 以及 OpenSpec 解析所需的 `Requirement`、`Scenario`、`ADDED Requirements` 等约定性关键字可保留英文原文。

## Governance

- CI lint failure 阻断 PR 合并和主线 push。
- CI 的 `govulncheck` 和 `gosec` matrix 必须覆盖四个 Go module；工具 module 不得低于业务 module 的静态安全门禁。
- `.github/workflows/lint.yml` 仅提供 `workflow_call`，由主 CI 唯一调用；分支保护绑定主 CI 下稳定的 `quality / lint` check，不得再绑定或触发独立的重复 lint matrix。
- 本地提交前运行 `make lint`，或至少运行受影响模块的 lint。
- 新增严格规则导致大量历史 findings 时，应先单独设计治理范围，不要混入业务 PR。
- CI 安全门禁和 lint 工具必须固定版本，不得使用 `@latest`；`gosec` 等工具通过 `renovate.json` 定期升级并由 CI 验证。
- 生成代码排除规则只能覆盖 Ent codegen 输出，不得排除 `user-service/internal/persistence/ent/schema/`、SQL migration 或 `atlas.sum`。
- 新增或调整分层规则时，同步更新 `AGENTS.md`、`docs/ARCHITECTURE.md`、相关 OpenSpec 主规格和 `.golangci.yml`。
- 新增或更新 OpenSpec/OPSX 文档时，必须同步保持简体中文语言约束，并运行 `make user-service-architecture-lint` 防止英文模板内容进入主线。
