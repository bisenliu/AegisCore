## ADDED Requirements

### Requirement: RBAC application/domain 框架无关分层

role 和 permission feature 的 application/domain 生产包 MUST 保持框架无关，不得承载仅服务于 Fx DI 的 import、`fx.In` 或 Fx struct tag。无 DI metadata 需求的普通 application 构造器 MUST 由 feature composition 直接注册；确有 Fx Params、named/optional metadata 或配置转换需求时，对应 adapter MUST 位于 feature composition 边界，例如各 feature 的 `fx.go` 或等价 composition 文件。该约束 MUST 不改变 RBAC command/query 业务行为、application port 所有权、transport DTO、policy reload、用户角色缓存失效或跨副本 policy sync 语义。

#### Scenario: application command 直接构造无需 Fx

- **WHEN** role 或 permission command service 在单元测试或非 Fx 调用方中被直接构造
- **THEN** 调用方 MUST 能使用强类型普通参数提供 `RoleStore`、`UserRoleStore`、`RolePermissionStore`、`PermissionLookup`、`PermissionStore` 和 `PolicyChangeNotifier` 等协作者
- **AND** 调用方 MUST NOT 需要 import `go.uber.org/fx`、嵌入 `fx.In` 或提供 Fx DI tag
- **AND** 缺少必需 `PolicyChangeNotifier` 时的 fail-fast 或拒绝装配语义 MUST 保持不变

#### Scenario: feature composition 注册普通构造器
- **WHEN** user-service 通过正式 feature module 组装 role 或 permission command service
- **THEN** 无 `name`/`optional` tag 或配置转换需求的 application command 构造器 MUST 由 feature composition 直接注册
- **AND** 只有存在真实 DI metadata 或配置转换需求时，composition adapter 才 MUST 把 Fx graph 输入转换为 application command 构造器需要的普通依赖
- **AND** 正式 module MUST 继续成功构图和启动

#### Scenario: lint 阻止 application/domain Fx 回归
- **WHEN** user/auth/role/permission 的 application/domain 生产包新增 `go.uber.org/fx` import、`fx.In` 或仅用于 Fx DI 的 struct tag
- **THEN** `make user-service-architecture-lint` MUST 失败并指出对应违规
- **AND** lint fixture MUST 覆盖至少一个 application/domain Fx 违规样例，证明规则可命中

#### Scenario: RBAC 业务语义保持不变
- **WHEN** role/permission command service 完成 DI metadata 下沉到 composition 边界
- **THEN** 权限目录、角色、角色权限绑定、用户角色绑定、policy reload、用户角色缓存失效、Redis policy version 发布和 Casbin 授权结果 MUST 与变更前保持一致
- **AND** 本变更 MUST NOT 修改 HTTP API、OpenAPI、数据库 schema、Atlas migration 或部署资产
