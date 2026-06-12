# Go Lint 自动化整改方案

## 1. 问题描述

当前仓库使用 `golangci-lint` 统一执行 Go 格式、导入顺序、未处理错误、静态分析、风格检查和架构依赖边界检查。Lint 是本地开发和 CI 合并门禁的一部分，避免格式、静态问题或错误分层依赖进入主线。

## 2. 整改目标

- 使用 `golangci-lint` 统一 Go lint 规则。
- 在本地提供明确安装、运行和排查方式。
- 在 CI 中对 PR 和主线提交执行 lint，并作为合并门禁。
- 提供可选 pre-commit 方案，把低成本问题前移到提交前发现。
- 对未来新增 lint 规则或历史分支带回的问题采用分阶段治理，避免一次性超大改动影响正常迭代。
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
    - depguard
```

完整配置应同时包含运行超时、输出格式、测试文件检查策略、issue 限制、生成代码排除规则和架构依赖边界规则。生成代码排除需要避免扫描 Ent codegen 输出，但不能排除 `user-service/ent/schema/`，因为 schema source 是开发者维护的代码。

`depguard` 规则用于把 `docs/ARCHITECTURE.md` 的分层依赖表转为可执行检查：

- `domain` 不得导入 Gin、Ent、Redis、runtime config、logger、HTTP response envelope、application ports 或 infrastructure adapter。
- `application` 不得导入 Gin、Ent、Redis 或 `common/http/binding`。
- `transport/http` 不得导入 Ent、Redis 或 SQL/pgx package。
- `transport/grpc` 不得导入 Ent、Redis、SQL/pgx、Gin、HTTP response envelope、HTTP controller 或 external integration adapter。
- `infrastructure/*` 不得导入 Gin 或 HTTP response envelope。
- `integration/*` 不得导入 Gin response、Ent 或 feature persistence adapter。

新增或调整分层规则时，先更新 `AGENTS.md` 和 `docs/ARCHITECTURE.md` 的长期规则，再同步 `.golangci.yml` 中对应的 depguard deny list。

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

验证根目录配置是否能被当前版本读取：

```bash
golangci-lint config verify --config .golangci.yml
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

当前策略：

- CI lint failure 直接阻断 PR 合并和主线 push；`.github/workflows/lint.yml` 不使用 `continue-on-error`。
- 分支保护应将 lint workflow 设为 required check。
- 本地提交前应运行 `make lint`，或至少运行受影响模块的 `make lint-common` / `make lint-user-service`。
- 如果未来新增严格规则导致大量历史问题，应先作为独立变更设计治理范围；不要在业务 PR 中混入大规模无关 lint 清理。

## 9. 新增规则和历史问题治理方式

未来新增 lint 规则、升级工具版本或合并历史分支时，如果重新出现批量 findings，按以下方式分阶段治理：

1. 先运行 `make lint-common` 和 `make lint-user-service`，统计 `common/` 与 `user-service/` 的问题数量、类型和路径。
2. 第一阶段修复高风险问题：`govet`、`staticcheck`、真实未处理错误、可能 panic 或资源泄露的问题。
3. 第二阶段修复低成本问题：`gofmt`、`goimports`、`unused`、`gosimple`。
4. 第三阶段处理风格类问题：`stylecheck`、`revive` 中争议较大的规则先团队确认，再分批落地。
5. 每个 PR 只处理一个模块、一类问题或一组相关文件，避免大规模无关 diff 混入业务变更。
6. 如果必须临时排除某些路径或规则，需要在整改方案或 issue 中记录原因、责任人和清理时间。

当前验证基线：

- `make lint-common` 应通过。
- `make lint-user-service` 应通过。
- `make lint` 应通过。
- `cd user-service && golangci-lint run --config ../.golangci.yml --enable-only depguard ./...` 应通过。

Depguard 分层违规基线：

- 当前不保留已知 depguard 违规。
- Auth Redis key builder 接收 plain app name；runtime config 提取留在 Redis infrastructure adapter，domain 不导入 `common/runtime/config`。

Depguard 历史违规治理建议：

1. 优先处理 domain 违规，因为 domain 是最内层边界，错误依赖会向所有外层扩散。
2. 对每个违规只移动最小依赖：把配置读取、logger、Gin/HTTP response、Ent/Redis client 或 SQL 访问移到 application、transport 或 infrastructure 的正确层。
3. 如果确有例外需求，先更新 `docs/ARCHITECTURE.md` 的 dependency table 和边界说明，再调整 depguard；不要只用 `nolint` 绕过架构规则。
4. 清理完成后重新运行 `cd user-service && golangci-lint run --config ../.golangci.yml --enable-only depguard ./...`。

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
- `depguard` 失败：先确认当前文件所属层，再把被禁止 import 移到允许依赖该包的层；例如 controller 通过 command/query 调 application，application 通过 port 调 infrastructure，domain 不读取 runtime config。
- 生成代码报错：确认 `.golangci.yml` 排除的是 Ent codegen 输出，而不是 `user-service/ent/schema/`。
- CI 与本地结果不一致：检查 Go 版本、`golangci-lint` 版本、执行目录和配置路径是否一致。
