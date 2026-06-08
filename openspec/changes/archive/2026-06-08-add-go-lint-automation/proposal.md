## Why

当前仓库只有测试、格式化和迁移校验说明，缺少统一的自动化代码规范检查，容易让格式、导入顺序、未处理错误、静态分析问题和风格差异在 review 或主线集成后才暴露。

引入 `golangci-lint`、CI 门禁、可选 pre-commit 和存量问题治理方案，可以把 Go 代码质量检查前移，同时避免历史问题一次性治理影响正常迭代。

## What Changes

- 新增仓库级 `golangci-lint` v2 配置，覆盖 `gofmt`、`goimports` formatter，以及 `govet`、`errcheck`、`staticcheck`、`unused`、`revive` 等基础 lint 能力；`gosimple` 与 `stylecheck` 语义通过 v2 的 `staticcheck` analyzer suite 覆盖。
- 新增团队工程整改方案文档，说明问题描述、整改目标、目录结构、配置文件位置、本地执行命令、CI/pre-commit 集成建议、失败阻断策略、存量问题治理和团队协作策略。
- 在 CI 中引入 lint 检查，建议对新增和变更代码逐步建立“失败即阻断合并”的质量门禁。
- 提供 pre-commit 集成建议，作为本地快速反馈机制，不替代 CI。
- 在 `docs/DEVELOPMENT.md` 补充安装、运行和排查 lint 的说明。
- 如存在较多历史问题，采用分阶段治理、优先级排序和必要的临时排除机制，避免一次性大规模改动。

## Capabilities

### New Capabilities

- `go-lint-automation`: 管理 Go lint 配置、本地执行、CI/pre-commit 集成、存量 lint 问题治理和开发文档化要求。

### Modified Capabilities

- 无。

## Impact

- 影响配置：新增仓库根目录 `.golangci.yml`，可选新增 `.github/workflows/lint.yml` 或同等 CI 配置。
- 影响文档：新增工程整改方案文档，并更新 `docs/DEVELOPMENT.md` 的 lint 安装、运行和排查说明；必要时更新 `docs/opsx/CAPABILITY_MAP.md`。
- 影响开发流程：本地可运行 lint，CI 可对 PR 执行 lint；推荐逐步将 lint 失败作为合并阻断条件。
- 不改变 HTTP API、错误码、运行时配置、数据库 schema、Ent 生成代码或 Atlas migration 历史。
