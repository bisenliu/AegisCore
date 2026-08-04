## Why

当前 `ci.yml` 的 `verify` 与 `test` job 重复执行 `make test`，同时独立 `lint.yml` 与 `verify` 重复执行等价 lint。两个 workflow 又都监听同一组 PR 和主线 push，导致同一 commit 重复消耗 runner、延长反馈时间，并产生多组含义重叠的 required checks。

## What Changes

- 将 `lint.yml` 改为仅接受 `workflow_call` 的复用质量 workflow，由主 `ci.yml` 唯一调用。
- 复用 workflow 分别提供稳定命名的 `lint` 与 `unit` job，每个 commit 各执行一次 `make lint` 和 `make test`。
- 从 `verify` job 删除 lint 与普通单测，只保留架构检查、OpenAPI 生成和 drift 检查。
- 将 Docker-backed 测试保留为独立 `container-test` job，仅执行其专用测试入口，不重复普通单测。
- 同步测试、架构和 lint 治理文档，并明确分支保护只绑定主 CI 产生的唯一质量 check 名称。

## Capabilities

### New Capabilities

无。

### Modified Capabilities

- `delivery-operations`：收敛 GitHub Actions 质量门禁编排，保证同一 PR commit 的标准 lint 和普通单测各执行一次。

## Impact

- CI：修改 `.github/workflows/ci.yml` 和 `.github/workflows/lint.yml` 的触发与 job 编排。
- 文档：更新 `docs/TESTING.md`、`docs/GO_LINT_AUTOMATION.md`、`docs/ARCHITECTURE.md` 和能力地图。
- 分支保护：required checks 应迁移为主 CI 下稳定的 `quality / lint` 与 `quality / unit`，不再引用旧独立 lint workflow 的 matrix checks。
- 不影响 Go 代码、HTTP API、OpenAPI 生成物、数据库 schema/migration、部署清单、观测资产或安全边界。
