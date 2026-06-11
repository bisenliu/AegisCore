# Go Lint 自动化整改方案

## 1. 问题描述

当前仓库已经有 Go 测试、格式化、Ent 生成和 Atlas migration 校验说明，但缺少统一的自动化代码规范检查。代码格式、导入顺序、未处理错误、静态分析告警和风格问题主要依赖人工 review 发现，容易出现反馈滞后、不同 reviewer 标准不一致和问题进入主线后再修复的情况。

## 2. 整改目标

- 使用 `golangci-lint` 统一 Go lint 规则。
- 在本地提供明确安装、运行和排查方式。
- 在 CI 中对 PR 和主线提交执行 lint，逐步形成合并门禁。
- 提供可选 pre-commit 方案，把低成本问题前移到提交前发现。
- 对存量 lint 问题采用分阶段治理，避免一次性超大改动影响正常迭代。
- 在 `docs/DEVELOPMENT.md` 保留日常入口，在本文档保留完整整改策略。

## 3. 推荐目录结构

```text
.
  .golangci.yml
  .github/
    workflows/
      lint.yml
  docs/
    DEVELOPMENT.md
    GO_LINT_AUTOMATION.md
  common/
    go.mod
  user-service/
    go.mod
```

配置放置原则：

- `.golangci.yml` 放在仓库根目录，统一 `common/` 和 `user-service/` 的 lint 规则。
- CI workflow 放在 `.github/workflows/lint.yml`；如果团队使用 GitLab CI、Jenkins 或其他平台，保留同等命令即可。
- `docs/DEVELOPMENT.md` 放日常命令和排查入口，本文档放完整治理方案。
- pre-commit 可以只作为文档建议，也可以后续落地 `.pre-commit-config.yaml`。

## 4. 示例配置

当前仓库根目录使用 `.golangci.yml`。本方案使用 `golangci-lint` v2 配置；`gofmt` 和 `goimports` 在 v2 中属于 formatter，`gosimple` 与 `stylecheck` 的 S/ST 规则语义由 `staticcheck` analyzer suite 覆盖。

核心配置包括：

```yaml
formatters:
  enable:
    - gofmt
    - goimports

linters:
  default: standard
  enable:
    - revive
```

完整配置应同时包含运行超时、输出格式、测试文件检查策略、issue 限制和生成代码排除规则。生成代码排除需要避免扫描 Ent codegen 输出，但不能排除 `user-service/ent/schema/`，因为 schema source 是开发者维护的代码。

## 5. 本地执行命令

安装 `golangci-lint`，建议使用与 CI 一致的版本：

```bash
go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2
```

分别检查两个 Go module：

```bash
cd common
golangci-lint run ./...
```

```bash
cd user-service
golangci-lint run ./...
```

不建议把仓库根目录 `golangci-lint run ./...` 作为唯一命令，因为仓库根目录是 Go workspace，不是单一 Go module。CI 和本地开发都应分别进入 `common/` 与 `user-service/` 执行。

## 6. CI 集成建议

推荐使用 CI 作为最终合并门禁。GitHub Actions 示例：

```yaml
name: lint

on:
  pull_request:
  push:
    branches:
      - main
      - master

jobs:
  golangci-lint:
    runs-on: ubuntu-latest
    strategy:
      matrix:
        module:
          - common
          - user-service
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.26.3'
      - uses: golangci/golangci-lint-action@v8
        with:
          version: v2.12.2
          working-directory: ${{ matrix.module }}
          args: --config ../.golangci.yml ./...
```

其他 CI 平台可使用等价命令：

```bash
cd common && golangci-lint run ./...
cd ../user-service && golangci-lint run ./...
```

## 7. Pre-commit 集成建议

pre-commit 适合在提交前快速发现格式、导入顺序和基础 lint 问题，但它可被跳过，且不同开发机环境可能不一致，因此不能替代 CI。

可选 `.pre-commit-config.yaml` 示例：

```yaml
repos:
  - repo: https://github.com/golangci/golangci-lint
    rev: v2.12.2
    hooks:
      - id: golangci-lint
        name: golangci-lint common
        args: [--config, ../.golangci.yml, ./...]
        files: ^common/.*\.go$
      - id: golangci-lint
        name: golangci-lint user-service
        args: [--config, ../.golangci.yml, ./...]
        files: ^user-service/.*\.go$
```

如果不引入 pre-commit 框架，也可以在 `.git/hooks/pre-commit` 本地脚本中执行：

```bash
#!/usr/bin/env bash
set -euo pipefail

(cd common && golangci-lint run ./...)
(cd user-service && golangci-lint run ./...)
```

推荐落地方式：先启用 CI，确保团队有统一门禁；pre-commit 作为自愿使用的本地提速工具，待团队认可后再考虑提交框架配置。

## 8. 是否阻断合并

推荐策略：

- 如果 lint 基线已经清零或问题规模很小，CI lint failure 应直接阻断 PR 合并，并在分支保护中设置 required check。
- 如果历史问题较多，先让 CI 以报告模式运行或只对新增问题设置阻断，再逐步清理存量问题。
- 不建议在存量问题未分类时立即全量阻断，否则容易让正常业务迭代被大量低风险历史问题阻塞。

当前仓库首次接入时，`common/` 和 `user-service/` 均存在存量 lint findings，因此 `.github/workflows/lint.yml` 暂时对 lint step 设置 `continue-on-error: true`。完成存量治理或建立新增问题阻断机制后，应移除该设置，并在分支保护中将 lint workflow 设为 required check。

## 9. 存量问题治理方式

分阶段治理：

1. 先运行 `golangci-lint run ./...`，统计 `common/` 与 `user-service/` 的问题数量、类型和路径。
2. 第一阶段修复高风险问题：`govet`、`staticcheck`、真实未处理错误、可能 panic 或资源泄露的问题。
3. 第二阶段修复低成本问题：`gofmt`、`goimports`、`unused`、`gosimple`。
4. 第三阶段处理风格类问题：`stylecheck`、`revive` 中争议较大的规则先团队确认，再分批落地。
5. 每个 PR 只处理一个模块、一类问题或一组相关文件，避免大规模无关 diff 混入业务变更。
6. 如果必须临时排除某些路径或规则，需要在整改方案或 issue 中记录原因、责任人和清理时间。

首次验证基线：

- `common/` 当前存在 45 个 lint findings，主要包括 `gofmt`、`goimports`、`govet`、`revive` 和 `staticcheck`。
- `user-service/` 当前存在 47 个 lint findings，主要包括 `goimports`、`revive` 和 `unused`。
- 本次变更不直接修复这些存量问题，避免在自动化接入 PR 中混入大规模格式化和行为相关修改。

优先级建议：

- P0：可能导致编译失败、运行时错误、资源泄露或安全风险的问题。
- P1：未处理错误、静态分析真实 bug、导入和格式问题。
- P2：命名、注释和风格类问题。
- P3：规则争议较大或收益不明确的问题，先观察或关闭。

## 10. 团队协作策略

- 初期只启用基础高价值规则，避免一次性引入大量低收益规则。
- 对生成代码、历史遗留目录和第三方代码使用明确排除，而不是降低整体规则质量。
- lint 规则升级、版本升级或新增严格规则应作为独立变更推进。
- code review 聚焦设计、行为和风险；格式、导入和常见静态问题交给自动化检查。
- 对业务紧急修复，可以允许临时绕过非阻断 pre-commit，但 CI 门禁仍应保持一致。

## 11. 开发文档补充建议

`docs/DEVELOPMENT.md` 应包含：

- `golangci-lint` 安装命令。
- `common/` 和 `user-service/` 的本地 lint 命令。
- 常见失败排查方式。
- 本文档路径，作为完整整改方案入口。

## 12. 常见排查

- `gofmt` 失败：运行 `gofmt -w <files>`。
- `goimports` 失败：安装 `goimports` 后运行 `goimports -w <files>`，或按 lint 输出调整 imports。
- `errcheck` 失败：显式处理错误；确认为安全忽略时，用 `_ = fn()` 表达有意忽略并在必要时补充说明。
- `staticcheck` 或 `govet` 失败：优先按真实 bug 处理，不建议直接排除。
- 生成代码报错：确认 `.golangci.yml` 排除的是 Ent codegen 输出，而不是 `user-service/ent/schema/`。
- CI 与本地结果不一致：检查 Go 版本、`golangci-lint` 版本、执行目录和配置路径是否一致。
