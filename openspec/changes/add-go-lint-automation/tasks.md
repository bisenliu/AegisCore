## 1. Lint Configuration

- [x] 1.1 在仓库根目录新增 `.golangci.yml`，启用 `gofmt`、`goimports` formatter，以及 `govet`、`errcheck`、`staticcheck`、`unused` 和 `revive` linters，并说明 `gosimple`、`stylecheck` 语义由 `staticcheck` 覆盖。
- [x] 1.2 为 `.golangci.yml` 或 CI 补充运行超时、输出格式、测试文件检查策略和合理的 issue 限制。
- [x] 1.3 配置生成代码和第三方目录排除规则，避免 Ent 生成代码产生噪声，同时不排除 `user-services/ent/schema/`。

## 2. Engineering Remediation Plan

- [x] 2.1 新增一份工程整改方案文档，说明缺少自动化代码规范检查的问题描述和整改目标。
- [x] 2.2 在整改方案中说明推荐目录结构和配置文件放置位置，包括 `.golangci.yml`、CI workflow、可选 pre-commit 配置和开发文档入口。
- [x] 2.3 在整改方案中提供更完整的 `.golangci.yml` 示例配置和本地执行命令示例。
- [x] 2.4 在整改方案中提供 GitHub Actions 或其他常见 CI 的示例配置，并说明分别在 `common/` 与 `user-services/` 运行 lint。
- [x] 2.5 在整改方案中说明 pre-commit 与 CI 的适用场景、推荐落地方案，以及为什么 CI 是最终合并门禁。
- [x] 2.6 在整改方案中说明 lint 失败是否阻断合并的推荐策略，包括 clean baseline 阻断、历史问题较多时分阶段阻断。
- [x] 2.7 在整改方案中说明存量 lint 问题治理方式，包括分阶段治理、优先级建议、控制单次 PR 范围和临时排除清理计划。
- [x] 2.8 在整改方案中说明团队协作中如何平衡规范严格度与研发效率。

## 3. CI And Pre-commit Integration

- [x] 3.1 新增或补充 GitHub Actions lint workflow，安装固定版本 `golangci-lint` 并分别检查 `common/` 与 `user-services/`。
- [x] 3.2 在整改方案中提供可选 pre-commit 配置或 hook 示例，明确其为本地快速反馈机制。
- [x] 3.3 如仓库已有 CI 约定与 GitHub Actions 不一致，保留等价 CI 命令并在整改方案中说明替代方式。

## 4. Development Documentation

- [x] 4.1 更新 `docs/DEVELOPMENT.md` 的前置条件或常用命令，补充 `golangci-lint` 安装说明。
- [x] 4.2 在 `docs/DEVELOPMENT.md` 中补充 `common/` 和 `user-services/` 的本地 lint 执行命令。
- [x] 4.3 在 `docs/DEVELOPMENT.md` 中补充 lint 失败排查建议，并引用完整整改方案文档。
- [x] 4.4 更新 `docs/opsx/CAPABILITY_MAP.md`，加入 `go-lint-automation` capability 及其主规格位置。

## 5. Verification

- [x] 5.1 运行 `golangci-lint run ./...` 于 `common/`，记录结果并评估是否需要分阶段治理。
- [x] 5.2 运行 `golangci-lint run ./...` 于 `user-services/`，记录结果并评估是否需要分阶段治理。
- [x] 5.3 如 lint 问题规模较小，修复高风险和低成本问题；如问题较多，保留整改方案中的分阶段治理安排，避免一次性大规模改动。
- [x] 5.4 确认本次变更不修改 HTTP API、运行时配置、数据库 schema、Ent 生成代码、migration SQL 或 `atlas.sum`。
- [x] 5.5 复查 `.golangci.yml`、CI 示例、整改方案和 `docs/DEVELOPMENT.md` 是否覆盖 `go-lint-automation` 规格场景。
