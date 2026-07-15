## Why

role 和 permission 的 application command 当前把仅用于 Fx DI 的 `fx.In`、`name` 和 `optional` metadata 暴露在应用层参数对象中，导致 application 层依赖组合框架并削弱 feature 分层边界。现在需要移除 application 层的 DI metadata，使没有 named/optional 或配置转换需求的普通构造器可由 feature composition 直接注册，并保持 application/domain 生产代码框架无关。

## What Changes

- 移除 `permission/application/command` 和 `role/application/command` 生产代码中仅为 DI 服务的 Fx import、`fx.In` 嵌入和 Fx struct tag。
- 让 application command 构造器接收强类型普通参数，保留现有 port 所有权、依赖必需性和业务行为。
- 在各 feature 的 `fx.go` 中直接注册无需 DI metadata 的普通 application 构造器；只有确有 named/optional 或配置转换需求时才由 composition adapter 承载并转换。
- 审计 user/auth/role/permission 的 application/domain 生产包，防止新增 Fx import、`fx.In` 或 Fx DI tag。
- 调整 role/permission 直接构造单元测试和正式 Fx module 测试，确保单元测试无需 Fx，module 构图仍通过。
- 更新 `rbac-access-control` capability 的分层约束；同步补充架构文档和 architecture lint fixture，使 application/domain 新增 Fx 依赖会被命中。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `rbac-access-control`: 增加 RBAC role/permission application/domain 层不得承载 Fx DI metadata 的分层约束，并要求 feature composition 负责框架适配。

## Impact

- 影响代码：`user-service/internal/features/permission/application/command`、`user-service/internal/features/role/application/command`、`user-service/internal/features/permission/fx.go`、`user-service/internal/features/role/fx.go` 及相关测试。
- 影响规则：`user-service/scripts/architecture-lint.sh` 和对应违规 fixture 需要覆盖 application/domain Fx import、`fx.In` 和 Fx tag 禁止规则。
- 影响文档与规格：`docs/ARCHITECTURE.md` 和 `openspec/changes/move-feature-fx-metadata-to-composition/specs/rbac-access-control/spec.md` 需要描述新的分层约束。
- 不影响 HTTP API、OpenAPI、数据库 schema、migration、transport DTO、application port 所有权、RBAC command/query 业务行为或部署资产。
