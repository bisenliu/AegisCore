# Enforce architecture with depguard

## What

Use `golangci-lint` `depguard` rules to make the architecture dependency table in `AGENTS.md` and `docs/ARCHITECTURE.md` executable.

包括：

- 在根目录 `.golangci.yml` 中启用并配置 `depguard`。
- 为 `user-service/internal/features/*/domain` 禁止 Gin、Ent、Redis、runtime config、logger、HTTP response envelope、application ports 和 infrastructure adapter imports。
- 为 `user-service/internal/features/*/application` 禁止 Gin、Ent、Redis 和 `common/http/binding` imports。
- 为 `user-service/internal/features/*/transport/http` 禁止 Ent、Redis 和 SQL imports。
- 为 `user-service/internal/features/*/transport/grpc` 禁止 Ent、Redis、SQL、Gin controller、HTTP response envelope 和 external client adapter imports。
- 为 `user-service/internal/features/*/infrastructure/*` 禁止 Gin 和 HTTP response imports。
- 为 `user-service/internal/integration/*` 禁止 Gin response、Ent 和 service-owned persistence adapter imports。
- 更新 `docs/GO_LINT_AUTOMATION.md`，说明 depguard 的目的、执行方式、历史违规处理方式和分阶段治理清单。

本变更只固化架构依赖边界和对应文档，不重排 feature 目录、不调整业务流程、不修改 HTTP API、不修改数据库 schema、不引入 OpenSpec/OPSX 工件。

## Why

仓库已经在 `AGENTS.md` 和 `docs/ARCHITECTURE.md` 中明确 feature-first 分层和禁止依赖，但这些规则目前主要依赖人工 review。随着 feature、transport 和 infrastructure adapter 增加，错误 import 很容易悄悄穿透边界，例如 application 直接引入 Gin/Ent，domain 引入 runtime config，或 infrastructure 返回 HTTP response。

`depguard` 能把这些结构性规则前移到本地 lint 和 CI 日志中。它不替代架构文档，但可以让“哪些层不能依赖什么”变成可重复检查的自动化约束，减少后续结构漂移。

## Scope

包括：

- 修改 `.golangci.yml`：
  - 在 `linters.enable` 中增加 `depguard`。
  - 在 `linters.settings.depguard.rules` 下新增分层规则。
  - 使用明确的 `files` pattern 匹配 domain、application、transport/http、transport/grpc、infrastructure 和 integration 目录。
  - 使用 `deny` block 给每类禁止 import 添加可读 `desc`。
  - 保持现有 generated-code exclusions，不把 Ent 生成代码纳入新规则噪声。
- 更新 `docs/GO_LINT_AUTOMATION.md`：
  - 说明 depguard 对应的架构边界。
  - 增加 `golangci-lint config verify --config ../.golangci.yml` 的配置验证建议。
  - 说明 `make lint` 和单模块 lint 如何覆盖 depguard。
  - 如果启用后发现历史违规，记录按层分组的分阶段治理清单。
- 验证 `golangci-lint` 配置可被当前文档指定版本读取。
- 在 `common/` 和 `user-service/` 分别运行 lint 或至少运行 depguard/config 验证，确认新规则可执行。

不包括：

- 不一次性修复所有历史 lint 问题，除非某个历史违规会让新增 depguard 规则无法作为可运行基线落地。
- 不降低现有测试、lint 或 CI 要求。
- 不调整已有 feature 分层、provider wiring、route registration、Ent schema、Atlas migration 或 Swagger 产物。
- 不新增 `openspec/`、`docs/opsx/` 或其他已退役流程工件。

## Acceptance Criteria

- `.golangci.yml` 启用 `depguard`，并包含覆盖 domain、application、transport/http、transport/grpc、infrastructure 和 integration 边界的 deny rules。
- `golangci-lint config verify --config .golangci.yml` 可以成功读取配置。
- `make lint-common` 和 `make lint-user-service` 仍使用根目录 `.golangci.yml`。
- 对 `user-service/` 运行 `golangci-lint run ./...` 时，depguard 规则可执行；若存在历史违规，输出可定位到具体文件和禁止依赖。
- `docs/GO_LINT_AUTOMATION.md` 已说明 depguard 规则目的、执行命令和历史违规治理策略。
- 如果存在历史 depguard 违规，文档或变更记录中列出按层分组的分阶段治理清单，而不是在本变更中强行清理全部历史 lint findings。
- 未新增 OpenSpec/OPSX 工件。
