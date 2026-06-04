## Why

`common/runtime/infrastructure` 当前同时承载配置 Fx provider、日志 Fx provider、Redis/PostgreSQL provider 和运行时资源名常量，包名过宽且调用方需要使用 `commoninfra` 别名，后续继续加入基础设施能力时容易演变为运行时杂包。

本变更通过更细粒度的 runtime 包边界拆分纯逻辑、Fx adapter 和资源名契约，使 `shared-infrastructure` 与 `common-module-organization` 的职责边界更清晰，同时保持现有运行时行为稳定。

## What Changes

- 将 `common/runtime/infrastructure` 的职责拆分为更明确的包边界：`configfx`、`loggerfx`、`datastore`、`datastorefx` 和 `resources`。
- 保持 `common/runtime/config` 只负责 YAML 与 `AEGISCORE_` 环境变量加载，不引入 Fx 依赖。
- 保持 `common/runtime/logger` 只负责 Zap logger 初始化、context logger 和日志字段处理，不引入 Fx provider 职责。
- 将 Redis/PostgreSQL client 或连接池构造逻辑放入 `common/runtime/datastore`，将 Fx 命名 provider 与 lifecycle 注册放入 `common/runtime/datastorefx`。
- 将 `user_db`、`common_db`、`cache_redis` 等运行时资源名常量迁移到 `common/runtime/resources`。
- 更新用户服务 bootstrap 等调用方 import 与 provider 使用位置，移除对泛化 `commoninfra` 包别名的依赖。
- 不改变 YAML key、`AEGISCORE_` 环境变量、Redis/PostgreSQL 命名实例、Fx named injection、Zap logger 行为、Redis/PostgreSQL ping/close lifecycle 或 Ent client wiring。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `shared-infrastructure`：调整共享配置、日志、datastore provider 和运行时资源名的包边界要求，保持运行时契约不变。
- `common-module-organization`：更新 `common/runtime` 分类目录约束，要求纯逻辑 runtime 包与 Fx adapter 包分离。

## Impact

- 影响代码位置：`common/runtime/infrastructure/`、新增 `common/runtime/configfx/`、`common/runtime/loggerfx/`、`common/runtime/datastore/`、`common/runtime/datastorefx/`、`common/runtime/resources/`、`user-services/internal/bootstrap/` 及相关测试。
- Go import 路径会调整，但 `common/go.mod` 与 `user-services/go.mod` module 边界保持不变。
- 外部配置、环境变量、HTTP API、错误码、数据库 schema、migration 历史和 Fx named injection 名称不变。
- 需要更新 `docs/opsx/CAPABILITY_MAP.md`、相关 OpenSpec 主规格或变更规格，以反映新的 runtime 目录组织。
