## 1. 统一容器测试入口

- [x] 1.1 在根、common 和 user-service Makefile 建立分层 `test-containers` targets，由模块 target 显式传递 `-v -count=1 -args -aegiscore.testcontainers` 并覆盖各自 Docker-backed 测试包。
- [x] 1.2 将 `.github/workflows/ci.yml` 的 `container-test` job 切换为根 `make test-containers`，保持普通单测不重复执行。
- [x] 1.3 扩展 architecture lint 与 fixture，要求 CI 恰好调用一次根容器入口并拒绝 service-local 入口。

## 2. 同步规格和文档

- [x] 2.1 更新 `docs/TESTING.md` 与 `docs/ARCHITECTURE.md`，删除 `AEGISCORE_TEST_CONTAINERS` 契约并记录根入口、覆盖范围、verbose 输出、禁用缓存和启用后禁止 skip 的语义。
- [x] 2.2 更新 `delivery-operations`、`shared-platform-primitives` 与 `rbac-access-control` delta，统一真实依赖门禁契约。

## 3. 验证

- [x] 3.1 运行 `openspec validate unify-container-test-gate` 和 `make user-service-architecture-lint`。
- [x] 3.2 使用 dry-run 检查根、common 与 user-service `test-containers` targets 展开的命令，确认包范围和 flag 传递准确；在 Docker 可用时运行根 `make test-containers`。
- [x] 3.3 运行相关普通测试，确认未启用 flag 时 Docker-backed 测试仍按既有约定 skip，且其他单元测试不受影响。

## 4. 合并前门禁

- [x] 4.1 使用显式路径暂存本 change 的 Makefile、workflow、lint、文档和 OpenSpec artifacts，检查 `git status --short` 确认范围准确。
- [x] 4.2 在全部预期变更已暂存后运行 `make lint`；仅在命令通过后将本任务标记完成。
- [x] 4.3 在全部预期变更已暂存后运行 `make verify`；仅在相关测试、生成检查和最终 `git diff --exit-code` 全部通过后将本任务及 change 标记完成。
