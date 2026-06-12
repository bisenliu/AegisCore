# Close architecture lint gate

## What

Turn the existing architecture lint automation into a real passing gate.

包括：

- 清理当前 `make lint-common` 的存量 lint failures，使共享模块 lint 可以通过。
- 清理当前 `make lint-user-service` 的存量 lint failures，使用户服务 lint 可以通过。
- 修复 `user-service/internal/features/auth/domain/rediskeys.go` 的 depguard 违规，让 auth domain 不再依赖 runtime config。
- 移除 `.github/workflows/lint.yml` 中的 `continue-on-error: true`，让 CI lint failure 阻断 PR 和主线 push。
- 更新 `docs/GO_LINT_AUTOMATION.md` 和必要的开发文档，使文档从“报告型检查/存量问题快照”切换为“可执行门禁”。

本变更只关闭现有 lint 与架构依赖门禁，不改变 HTTP API、数据库 schema、Ent migration、Swagger 契约、feature 目录结构或 OpenSpec/OPSX 流程。仓库规则已明确不再维护 OpenSpec/OPSX 工件，因此本 change 使用 `docs/changes/close-architecture-lint-gate/`，不新增 `openspec/` 或 `docs/opsx/`。

## Why

`docs/ARCHITECTURE.md` 已经定义了分层依赖规则，`.golangci.yml` 也已经通过 depguard 把这些规则转为 lint 配置。但当前执行链没有闭环：

- `make lint-common` 当前失败，共 52 个 lint findings。
- `make lint-user-service` 当前失败，共 65 个 lint findings。
- `user-service` 中仍有 2 个 depguard 违规：auth domain 直接导入 `common/runtime/config`。
- `.github/workflows/lint.yml` 当前设置 `continue-on-error: true`，因此 CI 只报告 lint 失败，不形成阻断门禁。

这会让架构规则停留在“可观察但不可执行”的状态。关闭门禁后，后续 PR 只要引入错误 import、格式化漂移、未处理错误或静态分析问题，就会在本地 `make lint` 和 CI 中被明确拦截。

## Scope

包括：

- 对 `common/` 运行并清理 lint 输出中的：
  - `errcheck`
  - `gofmt`
  - `goimports`
  - `govet`
  - `revive`
  - `staticcheck`
- 对 `user-service/` 运行并清理 lint 输出中的：
  - `depguard`
  - `goimports`
  - `revive`
- 将 `RedisKeyBuilder` 从 config-aware constructor 调整为 plain value constructor 或 infrastructure/application 层适配，保持 domain 不导入 runtime config。
- 重新运行 `make lint-common`、`make lint-user-service` 和 `make lint`，确认两个模块都通过。
- 移除 lint workflow 的 `continue-on-error: true` 和“报告型检查”描述，使 workflow 自然失败即阻断。
- 更新 lint 文档中的存量问题快照、是否阻断合并策略和 depguard 违规记录。

不包括：

- 不扩大或放松 `.golangci.yml` 中的架构 deny rules 来绕过问题。
- 不新增 `nolint` 作为关闭门禁的主要手段；只有对确认无风险且规则收益低的个别位置，才可用明确注释解释。
- 不重构 feature 层级、providers、router、Ent schema、Atlas migration 或 Swagger 产物。
- 不新增 CI 平台以外的新工具链要求。
- 不新增 `openspec/`、`docs/opsx/` 或其他已退役流程工件。

## Acceptance Criteria

- `make lint-common` 成功通过。
- `make lint-user-service` 成功通过。
- `make lint` 成功通过，并顺序覆盖两个 Go module。
- `cd user-service && golangci-lint run --config ../.golangci.yml --enable-only depguard ./...` 成功通过。
- `user-service/internal/features/auth/domain` 不再导入 `github.com/aegiscore/common/runtime/config`。
- `.github/workflows/lint.yml` 不再包含 `continue-on-error: true`，lint job 失败会让 workflow 失败。
- `docs/GO_LINT_AUTOMATION.md` 不再把当前基线描述为仍有未清理存量 lint findings；文档说明 lint 已作为合并门禁运行。
- 变更未新增 OpenSpec/OPSX 工件。
