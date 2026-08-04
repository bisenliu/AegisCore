## 1. 收敛 CI 质量门禁

- [x] 1.1 将 `.github/workflows/lint.yml` 改为仅接受 `workflow_call` 的复用质量 workflow，提供稳定命名的 `lint` 与 `unit` job。
- [x] 1.2 在 `.github/workflows/ci.yml` 唯一调用复用 workflow，从 `verify` 删除 lint/普通单测，并将 Docker-backed 测试收敛为专用 `container-test` job。
- [x] 1.3 扩展 architecture lint 及其 fixture，检查 workflow 触发、唯一 caller 与命令计数，确认同一 PR commit 只有一个 `make lint` 和一个 `make test`，其他质量与安全 jobs 未减少。

## 2. 同步规格和文档

- [x] 2.1 更新 `delivery-operations` delta，明确唯一质量 workflow、稳定 check 名称和容器测试责任。
- [x] 2.2 更新 `docs/TESTING.md`、`docs/GO_LINT_AUTOMATION.md`、`docs/ARCHITECTURE.md` 与 `docs/opsx/CAPABILITY_MAP.md`，记录 CI 编排及分支保护迁移要求。

## 3. 验证

- [x] 3.1 运行 `openspec validate deduplicate-ci-quality-gates` 和 `make user-service-architecture-lint`。
- [x] 3.2 使用显式路径暂存本 change 的 workflow、文档和 OpenSpec artifacts，检查 `git status --short` 确认范围准确。
- [x] 3.3 在预期变更已暂存后运行 `make lint` 和 `make verify`，仅在命令全部通过后将任务标记完成。
