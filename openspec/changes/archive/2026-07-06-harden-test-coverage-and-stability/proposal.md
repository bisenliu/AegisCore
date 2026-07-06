## Why

当前仓库存在已确认的测试保障缺口：`tools/openapi-convert` 无单元测试且覆盖率为 0%，根验证链未纳入该工具模块；role infrastructure 默认测试覆盖率虽然达到 55.6%，但 `RolePermissionStore` 的 PostgreSQL 绑定替换、批量同步和事务异常路径在默认环境下因容器测试跳过而实际未覆盖。另有若干测试依赖固定 `time.Sleep` 或手动 `os.Setenv` 恢复全局环境，虽未稳定复现失败，但会在高负载或并行化后增加 flaky 和环境污染风险。

本 change 需要一次性收紧交付验证与测试契约，使关键路径具备默认可执行的回归保护，并移除不必要的兼容测试路径和时间等待假设。

## What Changes

- 为 `tools/openapi-convert` 增加 CLI 回归测试，覆盖参数解析、必填参数错误、`root-path` 与 `root-server` 约束、输入文件错误、输出文件生成和生成内容基本断言。
- 调整 `tools/openapi-convert` CLI 结构，使参数解析和执行逻辑可测试；`main()` 仅负责进程退出，测试直接调用可注入 stdout/stderr 的执行函数。
- 将 `tools/openapi-convert` 纳入根 `make test` 和 `make verify` 验证链，不再仅依赖 user-service OpenAPI 生成脚本间接执行该工具。
- 为 role infrastructure 补齐默认可执行的 `RolePermissionStore` 非锁定查询、删除、系统绑定同步、失败保持和 helper 映射测试；真实 `FOR UPDATE` 绑定替换路径继续由显式 PostgreSQL 集成测试覆盖，不为了 SQLite 测试修改生产锁语义。
- 对 `common/runtime/localcache`、`common/runtime/workerpool`、`common/runtime/scheduler`、`common/runtime/timezone`、auth token version validator 和 auth Redis session store 测试移除固定 `time.Sleep` 或手动 `os.Setenv` 恢复模式，改用 `require.Eventually`、条件通道、确定性时间/score 或 `t.Setenv`。
- **BREAKING**：不保留仅用于旧测试形态的兼容 helper、手写等待分支或隐式不纳入验证的工具模块行为。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `delivery-operations`：根测试和完整验证必须覆盖 `tools/openapi-convert`，仓库级 OpenAPI 转换 CLI 必须具备可测试的参数、错误和文件生成回归保护。
- `rbac-access-control`：角色权限绑定持久化测试必须默认覆盖不依赖 PostgreSQL 行锁的查询、删除、同步、缺失权限和失败保持路径；依赖 `FOR UPDATE` 的替换路径由显式 PostgreSQL 集成测试覆盖。
- `shared-platform-primitives`：共享 runtime primitive 的测试必须避免固定 sleep 和手动环境恢复导致的非确定性，使用可观察条件、通道或测试环境隔离表达异步与全局状态断言。
- `auth-session-management`：认证会话和 token version 测试必须避免 wall clock sleep 作为顺序或过期断言依据，使用确定性 score/clock 或 eventually-style 条件断言。

## Impact

- 受影响代码：`tools/openapi-convert/`、根 `Makefile`、`user-service/internal/features/role/infrastructure/postgres/`、`common/runtime/localcache/`、`common/runtime/workerpool/`、`common/runtime/scheduler/`、`common/runtime/timezone/`、`user-service/internal/features/auth/application/validators/`、`user-service/internal/features/auth/infrastructure/redis/`。
- API 影响：不改变 HTTP API、OpenAPI 外部契约、数据库 schema、Redis key schema、Casbin 授权语义或部署运行时行为。
- 交付影响：`make test` 和 `make verify` 将执行 `tools/openapi-convert` 测试；工具模块测试失败会阻止完整验证通过。
- 测试影响：role infrastructure 默认覆盖率目标提升到 70%+ 且不修改生产锁语义，`tools/openapi-convert` 覆盖率从 0% 提升到覆盖 CLI 主路径和错误路径；固定 sleep 和手动 `os.Setenv` 命中应被清除或收敛到无全局污染的测试隔离模式。
