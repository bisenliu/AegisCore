## ADDED Requirements

### Requirement: Provide repository golangci-lint configuration

系统 SHALL 在仓库根目录维护统一的 `golangci-lint` 配置，用于 `common/` 和 `user-services/` 两个 Go module 的自动化代码规范检查。

#### Scenario: Lint configuration exists at repository root

- **WHEN** 开发者或 CI 从仓库读取 lint 配置
- **THEN** 仓库根目录 MUST 存在 `.golangci.yml`
- **THEN** 配置 MUST 可被 `common/` 和 `user-services/` 目录下的 `golangci-lint run ./...` 复用

#### Scenario: Baseline checks are enabled

- **WHEN** 开发者查看 `.golangci.yml`
- **THEN** 配置 MUST 启用 `gofmt` 和 `goimports` formatter
- **THEN** 配置 MUST 启用 `govet`、`errcheck`、`staticcheck`、`unused` 和 `revive` linters
- **THEN** 配置 MUST 说明 `gosimple` 和 `stylecheck` 语义由 `staticcheck` analyzer suite 覆盖
- **THEN** 配置或 CI MUST 包含运行超时、输出格式、测试文件检查策略和必要的排除规则

#### Scenario: Generated code noise is controlled

- **WHEN** lint 执行扫描生成代码或第三方目录
- **THEN** 配置 MUST 避免 Ent 生成代码、vendor 或其他生成目录产生不可治理噪声
- **THEN** 配置 MUST NOT 排除 `user-services/ent/schema/` 下的 Ent schema source

### Requirement: Support local lint execution

系统 SHALL 为开发者提供明确的本地 lint 安装、执行和排查方式。

#### Scenario: Developer installs lint tool

- **WHEN** 开发者阅读 `docs/DEVELOPMENT.md` 或工程整改方案文档
- **THEN** 文档 MUST 说明如何安装 `golangci-lint`
- **THEN** 文档 MUST 建议团队使用固定版本或与 CI 保持一致的版本

#### Scenario: Developer runs lint per module

- **WHEN** 开发者需要本地执行 lint
- **THEN** 文档 MUST 提供在 `common/` 执行 `golangci-lint run ./...` 的命令
- **THEN** 文档 MUST 提供在 `user-services/` 执行 `golangci-lint run ./...` 的命令
- **THEN** 文档 MUST NOT 将仓库根目录 `golangci-lint run ./...` 作为唯一推荐命令

#### Scenario: Developer troubleshoots lint failures

- **WHEN** lint 执行失败
- **THEN** 文档 MUST 说明常见排查方式，包括格式化、导入排序、未处理错误、静态分析告警、误报处理和临时排除规则申请方式

### Requirement: Integrate lint with CI and pre-commit guidance

系统 SHALL 提供 CI lint 集成和可选 pre-commit 集成建议，明确两者适用场景和推荐落地方案。

#### Scenario: CI runs lint checks

- **WHEN** pull request 或主线分支触发 CI
- **THEN** CI SHOULD 分别在 `common/` 和 `user-services/` 运行 `golangci-lint run ./...`
- **THEN** CI 配置或工程整改方案 MUST 包含 GitHub Actions 或其他常见 CI 的示例配置

#### Scenario: CI is the merge gate

- **WHEN** 团队启用 lint 质量门禁
- **THEN** CI lint SHOULD 作为合并前最终一致性检查
- **THEN** pre-commit MUST NOT 被视为唯一合并门禁

#### Scenario: Pre-commit provides fast local feedback

- **WHEN** 开发者希望在提交前获得快速反馈
- **THEN** 工程整改方案 MUST 说明 pre-commit 适用于本地快速发现格式、导入和基础 lint 问题
- **THEN** 工程整改方案 MUST 说明 pre-commit 可被跳过且环境不稳定，因此不能替代 CI

### Requirement: Define lint failure blocking strategy

系统 SHALL 明确 lint 失败是否阻断合并，以及历史问题较多时的平滑接入策略。

#### Scenario: New repository or clean baseline blocks on lint

- **WHEN** 当前 lint 基线已清零或问题规模可控
- **THEN** CI lint failure SHOULD 阻断 pull request 合并
- **THEN** 分支保护或等价机制 SHOULD 将 lint 设置为 required check

#### Scenario: Existing repository has many historical issues

- **WHEN** 首次接入 lint 发现大量历史问题
- **THEN** 团队 MUST 先统计和分类问题
- **THEN** 团队 MUST 优先治理格式、导入、编译级 vet/staticcheck、未处理错误和真实 bug 风险
- **THEN** 团队 MUST 避免通过一次性超大 PR 修复所有低风险风格问题
- **THEN** 团队 MAY 暂时使用非阻断报告、路径级排除或分阶段 required check，并 MUST 为临时排除设置清理计划

#### Scenario: New code does not add lint debt

- **WHEN** 存量问题尚未完全清理
- **THEN** 新增或修改代码 MUST NOT 引入新的 lint 问题
- **THEN** 工程整改方案 MUST 说明如何通过分阶段治理平衡规范严格度与研发效率

### Requirement: Document lint remediation plan

系统 SHALL 提供一份可执行的工程整改方案，指导团队落地自动化代码规范检查。

#### Scenario: Team reads remediation plan

- **WHEN** 团队查看工程整改方案文档
- **THEN** 文档 MUST 包含问题描述、整改目标、推荐目录结构、配置文件放置位置和实施步骤
- **THEN** 文档 MUST 包含 `.golangci.yml` 示例配置和本地执行命令示例
- **THEN** 文档 MUST 包含 GitHub Actions 或其他常见 CI 示例配置
- **THEN** 文档 MUST 包含 CI/pre-commit 集成建议、失败阻断策略、存量问题治理方式和开发文档补充建议

#### Scenario: Development guide links daily lint workflow

- **WHEN** 开发者阅读 `docs/DEVELOPMENT.md`
- **THEN** 开发文档 MUST 提供 lint 安装、运行和排查的日常说明
- **THEN** 开发文档 SHOULD 引用更完整的工程整改方案文档
