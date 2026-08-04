## 1. 规格与实现

- [x] 1.1 创建 proposal、design 和 `rbac-access-control` spec delta，明确运行时幂等初始化与 migration 纯 schema 边界。
- [x] 1.2 在 role PostgreSQL transaction helper 中实现 counter 快速递增、缺失时最大 revision 对齐、Ent 幂等创建和重试递增。
- [x] 1.3 移除基线 migration 的 counter seed DML、migration seed 断言与 PostgreSQL 测试夹具预建 counter，并刷新 `atlas.sum`。

## 2. 测试与验证

- [x] 2.1 增加空 counter、已有 revision 对齐、并发首次初始化和失败回滚测试，运行 Docker-backed role、permission 与 E2E 测试。
- [x] 2.2 运行 `openspec validate runtime-initialize-rbac-revision-counter`、`make user-service-migrate-validate` 和 `make user-service-architecture-lint`。
- [x] 2.3 将本次预期代码、测试和 OpenSpec 文件加入暂存区，运行 `make lint` 与 `make verify`，并检查生成物 drift。
