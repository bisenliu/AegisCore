## Context

AegisCore 是 Go workspace，包含 `common/` 与 `user-services/` 两个 Go module。当前开发文档记录了测试、格式化、Ent 生成和 Atlas migration 校验，但没有统一 lint 配置、自动化执行入口、CI 门禁或存量 lint 问题治理策略。

该变更引入工程质量能力，不改变业务分层、HTTP API、配置加载、数据库 schema、Ent 生成代码或 migration 历史。主要影响仓库根目录配置、CI 配置、工程文档和团队开发流程。

## Goals / Non-Goals

**Goals:**

- 在仓库根目录提供 `.golangci.yml`，统一 Go lint 规则和排除策略。
- 支持在 `common/` 和 `user-services/` 两个模块分别运行 lint，避免误把仓库根目录当作单一 Go module。
- 提供工程整改方案文档，覆盖问题描述、整改目标、目录结构、实施步骤、示例配置、本地命令、CI/pre-commit 集成、阻断策略、存量治理和团队协作策略。
- 提供 GitHub Actions 或同等 CI 的示例配置，并说明 lint 失败是否阻断合并的推荐策略。
- 在 `docs/DEVELOPMENT.md` 补充 lint 安装、运行和排查说明。
- 如历史问题较多，允许分阶段接入，优先修复高风险问题并控制单次变更规模。

**Non-Goals:**

- 不一次性修复所有历史 lint 警告，除非 apply 阶段验证结果显示规模很小。
- 不引入业务代码重构、API 变更、数据库 migration 或 Ent schema 变更。
- 不强制所有开发者必须使用 pre-commit；pre-commit 作为推荐的本地快速反馈机制。
- 不把 lint 替代 `go test ./...`、Atlas migration validate 或人工 code review。

## Decisions

- 决策：`.golangci.yml` 放在仓库根目录。
  备选方案是在 `common/` 和 `user-services/` 各放一个配置。选择根目录配置是因为 lint 规则应统一，且 `golangci-lint run ./...` 可在各 module 目录读取上级配置，减少重复维护。

- 决策：CI 分别在 `common/` 和 `user-services/` 目录运行 `golangci-lint run ./...`。
  备选方案是在仓库根目录运行一次。选择模块目录运行是因为仓库根目录是 Go workspace 而非单一 module，分别运行更符合现有测试说明和 Go toolchain baseline。

- 决策：默认启用基础高价值检查，包括 `gofmt`、`goimports` formatter，以及 `govet`、`errcheck`、`staticcheck`、`unused` 和 `revive` linters；`gosimple` 与 `stylecheck` 作为 `staticcheck` analyzer suite 的 S/ST 规则语义覆盖。
  备选方案是使用 `golangci-lint` v1 配置继续启用 standalone `gosimple` 和 `stylecheck`。选择 v2 配置是因为本地工具链已使用 `golangci-lint` v2.12.2，v2 已移除 standalone `gosimple`/`stylecheck` 名称，继续使用 v1 配置会导致本地和 CI schema 不兼容。

- 决策：CI 是最终门禁，pre-commit 是可选本地加速反馈。
  备选方案是只依赖 pre-commit。选择 CI 作为门禁是因为本地 hook 可被跳过且环境差异较大，CI 更适合作为合并保护条件。

- 决策：阻断策略分阶段落地。
  备选方案是上线当天所有 lint failure 阻断合并。选择分阶段策略是为了在存在历史问题时保持迭代连续性：先报告、再限制新增问题、最后在存量清零或基线明确后全量阻断。

- 决策：新增一份工程整改方案文档，`docs/DEVELOPMENT.md` 只保留日常使用入口。
  备选方案是把完整整改方案写入 `docs/DEVELOPMENT.md`。选择拆分文档是为了让日常开发文档保持简洁，同时保留可执行的团队治理方案。

## Risks / Trade-offs

- 历史 lint 问题过多导致 CI 无法快速设为必需检查 -> 先统计问题、分批修复，高风险 linters 优先；必要时用临时排除并设置移除计划。
- lint 规则过严影响研发效率 -> 初期启用基础规则，团队评审后逐步增加规则，避免一次性引入低收益争议规则。
- pre-commit 影响本地提交速度 -> pre-commit 只运行较快检查，允许手动运行全量 lint；CI 保持最终一致性。
- 生成代码或第三方代码产生噪声 -> 在 `.golangci.yml` 中明确排除 Ent 生成代码、vendor 或其他生成目录，但保留 schema source 和业务代码检查。
- Go 版本或 `golangci-lint` 版本不一致导致结果漂移 -> 文档固定推荐 `golangci-lint` v2.12.2，并在升级时作为独立变更处理。
- 只做文档不落地配置 -> tasks 必须包含配置文件、CI 示例和开发文档更新的实际实施项。

## Migration Plan

1. 新增 `.golangci.yml`，先覆盖 v2 基础 linters/formatters 和必要的生成代码排除。
2. 新增工程整改方案文档，明确目录结构、本地命令、CI/pre-commit 方案、阻断策略和存量治理方式。
3. 更新 `docs/DEVELOPMENT.md`，给出安装、运行和排查入口。
4. 在本地分别对 `common/` 与 `user-services/` 运行 lint，记录现有问题规模。
5. 若问题规模较小，直接修复并让 CI 阻断合并；若问题较多，先启用非阻断报告或只阻断新增问题，再按优先级分批修复。
6. 存量问题治理完成后，将 lint CI 设为 required check。

## Open Questions

- CI 平台是否确定使用 GitHub Actions；如不是，工程方案需保留 GitLab CI、Jenkins 或其他 CI 的等价命令说明。
- 是否引入 `pre-commit` 框架配置文件，还是仅提供原生 `.git/hooks/pre-commit` 示例；推荐先文档化并可选落地框架配置。
