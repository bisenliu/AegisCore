## Context

`.github/workflows/ci.yml` 与 `.github/workflows/lint.yml` 当前都监听 `pull_request` 以及 `main`、`master` push。主 CI 的 `verify` 运行 `make lint` 和 `make test`，独立 lint workflow 再次运行两个 module 的 golangci-lint；主 CI 的 `test` 又再次运行 `make test` 后追加 Docker-backed 测试。因此同一 commit 会运行两轮 lint 和两轮普通单测。

本次变更只收敛 GitHub Actions 编排，不改变本地 `make lint`、`make test` 或 `make verify` 的完整验证语义。

## Goals / Non-Goals

**Goals:**

- 同一 PR 或主线 push commit 只运行一次标准 lint 和一次普通单测 suite。
- lint 与 unit 保持独立、稳定且唯一的 check 名称，便于并行反馈和分支保护配置。
- 架构、OpenAPI drift、Docker-backed 测试、build、race、coverage 和安全检查继续执行。
- 复用 workflow 不自行订阅重复事件，所有标准 CI 触发由主 workflow 统一拥有。

**Non-Goals:**

- 不改变 Makefile target 的本地语义。
- 不减少 race、coverage、安全扫描或容器测试覆盖。
- 不修改 Go 工具链和 lint 版本。
- 不自动修改 GitHub 仓库的分支保护设置。

## Decisions

### Decision: `lint.yml` 作为仅支持 `workflow_call` 的质量 workflow

保留现有文件路径以避免不必要的文件迁移，但将 workflow 名称改为 `quality`，触发器仅保留 `workflow_call`。该 workflow 定义 `lint` 与 `unit` 两个 job，分别运行一次仓库根 `make lint` 与 `make test`。

主 `ci.yml` 定义唯一 `quality` caller job，并继续独占 PR 与主线 push 触发。这样 lint/unit 的实现可以被未来具有不同触发条件的 workflow 复用，但当前不会因两个顶层 workflow 监听同一事件而重复执行。

不采用在两个 workflow 中用 path、条件表达式互斥，因为触发责任仍然分散且容易随后续编辑重新重叠；不把 lint 与 unit 串行放入单个 runner，因为二者可并行并能提供更快、可定位的失败反馈。

### Decision: `verify` 只负责架构、生成和 drift

主 CI 的 `verify` 删除 golangci-lint 安装、`make lint` 与 `make test`，保留 ripgrep 安装、`make user-service-architecture-lint`、`make user-service-openapi-generate` 和 `git diff --exit-code`。其 check 名称继续为 `verify`，责任从“重复完整本地 verify”收敛为生成与架构一致性。

不直接运行 `make verify`，因为该 target 有意包含 lint 和普通单测，适合作为本地合并前总入口，但在 CI 中会破坏 job 的唯一责任和重复执行约束。

### Decision: Docker-backed 测试使用专用 job

原 `test` job 改名为 `container-test`，只运行 `make -C user-service test-containers`。普通单测已由复用 workflow 的 `unit` job 覆盖，容器 job 仅保留显式启用的 PostgreSQL/Redis 与 HTTP E2E 包。

不在容器 job 再运行 `make test`，因为当前仓库的 Docker-backed 入口由 `-aegiscore.testcontainers` flag 启用，普通 `make test` 不会替代该专用入口，反而会重复普通测试。

## Risks / Trade-offs

- [Risk] 分支保护仍引用旧 `lint / golangci-lint (...)` check 时会等待不存在的状态 -> Mitigation：在合并本变更时同步将 required checks 切换到主 CI 下的 `quality / lint` 与 `quality / unit`。
- [Risk] 未来为 `lint.yml` 增加顶层事件触发后重新引入重复执行 -> Mitigation：规格要求复用 workflow 不直接监听与主 CI 相同的 PR/主线事件，review 时检查 `on.workflow_call` 边界。
- [Trade-off] lint job 使用一个 runner 顺序检查两个 module，而旧 matrix 使用两个 runner并行 -> 减少 check 与 runner 数量，换取单个 lint job 内的有限串行时间；unit 仍与 lint 并行。

## Migration Plan

1. 提交复用 workflow、主 CI 调用和文档/规格更新。
2. 在 PR 上确认只出现一组 `quality / lint` 与 `quality / unit`，且 `verify`、`container-test` 和其他门禁继续存在。
3. 将分支保护从旧独立 lint matrix check 切换到主 CI 的稳定 quality checks 后再合并。
4. 回滚时可恢复两个 workflow 的原触发与 job 内容；回滚会重新产生重复成本，但不影响应用运行时。

## Verification

- `openspec validate deduplicate-ci-quality-gates`
- `make user-service-architecture-lint`
- workflow YAML 解析和触发/job 结构断言
- 预期变更暂存后运行 `make lint` 和 `make verify`
