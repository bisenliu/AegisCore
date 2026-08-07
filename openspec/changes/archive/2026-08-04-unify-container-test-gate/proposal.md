## Why

仓库文档和部分主规格仍把 `AEGISCORE_TEST_CONTAINERS=1` 描述为真实依赖测试开关，但 `common/testing/containers` 实际只读取 `-aegiscore.testcontainers` Go flag。当前 CI 又只调用 user-service 的专用 target，导致 common 自身的 PostgreSQL/Redis 容器集成测试没有进入阻塞式门禁，开发者执行文档命令时也无法启用这些测试。

## What Changes

- 将根 `make test-containers` 设为完整真实依赖测试的唯一仓库入口，由它分别调用 common 与 user-service 的专用容器测试 target。
- common 与 user-service target 均显式向测试进程传递 `-args -aegiscore.testcontainers`，输出实际执行的测试名并禁用测试缓存。
- CI 的 `container-test` job 改为调用根入口，覆盖 common PostgreSQL/Redis fixture、permission/role PostgreSQL 集成测试和 user-service HTTP E2E。
- 通过架构 lint 固定 CI 只调用一次根容器门禁，禁止退回 service-local 入口或把普通 `make test` 混入容器 job。
- 删除当前文档和主规格中的 `AEGISCORE_TEST_CONTAINERS=1` 契约，统一记录 flag 由 Make target 负责传递；启用后任何 Docker、容器、migration 或配置失败必须失败，不得静默 skip。

## Capabilities

### New Capabilities

无。

### Modified Capabilities

- `shared-platform-primitives`：统一 common 容器测试基础设施的启用方式和失败语义。
- `delivery-operations`：新增仓库级完整容器门禁入口并让 CI 覆盖 common 与 user-service。
- `rbac-access-control`：更新 RBAC 真实依赖验收的统一运行契约。

## Impact

- 测试入口：修改根 `Makefile`、`common/Makefile` 和 `user-service/Makefile`。
- CI：修改 `.github/workflows/ci.yml`，容器 job 的负载增加 common PostgreSQL/Redis fixture 验收，但不重复普通单测。
- 架构检查：修改 `user-service/scripts/architecture/lint.sh` 及其 fixture 测试。
- 文档和规格：修改 `docs/TESTING.md`、`docs/ARCHITECTURE.md` 以及三个现有 capability 的主规格 delta。
- 不影响生产 Go 代码、HTTP API、OpenAPI 生成物、数据库 schema/migration、部署清单、观测资产或安全边界。
