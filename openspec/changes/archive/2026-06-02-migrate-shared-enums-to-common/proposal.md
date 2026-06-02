## Why

当前项目中部分枚举型常量和公共契约值已经集中在 `common`，但仍存在运行时资源名称、认证 Bearer 边界常量和认证失败公开文案分散维护的问题。将这些可复用公共枚举/常量统一迁移或整合到 `common` 可以降低跨服务 wiring、认证处理和测试断言中的字符串漂移风险。

## What Changes

- 全量审查 Go workspace 中的 enum-like 类型、`const` 组和重复字符串常量，区分公共枚举、服务内业务文案和 Ent 生成常量。
- 在 `common` 模块集中提供可复用的运行时资源名称常量，覆盖 `user_db`、`common_db`、`cache_redis` 等跨 bootstrap、Ent client、repository 和测试复用的依赖名称。
- 在 `common` 模块整合认证边界常量，避免 `Bearer` token type、`Bearer ` prefix 和 `Authorization` header 在不同包或调用方中重复硬编码。
- 将通用认证失败公开文案提升为 `common` 可复用常量，并同步中间件、响应测试或服务侧引用。
- 明确不迁移 `user-services/internal/apperror` 中的用户域业务文案，除非未来引入共享业务错误文案或 i18n 能力。
- 明确不手写或迁移 `user-services/ent/` 下的生成常量；若 Ent schema 字段或表名变更，必须通过 Ent schema 和生成流程处理。
- 不改变 HTTP API 响应字段、错误码数值、配置 key、Redis/PostgreSQL 命名实例字符串或数据库 schema。

## Capabilities

### New Capabilities
- `shared-enum-contracts`: 定义项目可复用枚举和公共常量的归属、迁移边界、引用更新和兼容性要求。

### Modified Capabilities
- `shared-infrastructure`: 运行时 Redis/PostgreSQL/Ent 依赖名称应复用 `common` 中的公共资源名称常量，同时保持 `redis.<name>`、`postgres.<name>` 和 Fx 注入名称语义不变。
- `user-authentication`: Bearer/Authorization 边界常量和认证失败公开文案应复用 `common` 中的统一定义，同时保持认证行为和失败响应兼容。
- `api-response-contract`: 共享认证失败文案与响应错误码常量应继续由 `common` 集中维护，不改变响应信封、业务码和 HTTP status 映射。

## Impact

- 影响代码范围：`common/jwt`、`common/contextutil`、`common/middleware`、`common/response`、`common/infrastructure` 或新增的 `common` 常量归属包；`user-services/internal/bootstrap`、`user-services/internal/entclient`、`user-services/internal/repository`、`user-services/internal/service` 及相关测试。
- 迁移清单：运行时资源名称 `user_db`、`common_db`、`cache_redis`；认证边界常量 `Authorization`、`Bearer`、`Bearer `；通用认证失败文案 `登录状态无效或已过期，请重新登录`。
- 非迁移清单：`common/response.Code` 等已在 `common` 的标准响应码；`common/validation.Enum` 和 validation tag 常量；用户服务私有业务文案；Ent 生成常量。
- 兼容性注意：外部 JSON 响应、错误码数值、header 名、Bearer 文本、配置 YAML/env key、Fx name tag 字符串和数据库字段必须保持原值；Go API 命名调整可能影响内部导入和测试，但不应造成外部协议破坏。
