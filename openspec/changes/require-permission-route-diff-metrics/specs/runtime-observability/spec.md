## ADDED Requirements

### Requirement: Permission Metrics 正式依赖图必须完整

user-service 的正式 permission 模块 MUST 向 `PermissionQueryService` 提供唯一且明确的单值 `permissionapplication.Metrics` 依赖。该依赖 MUST 在 Fx/Dig 图中作为必选输入边存在，MUST NOT 使用 variadic、optional 或 slice/group annotation 表达可缺失的 Metrics。

#### Scenario: metrics 启用时注入真实实现

- **WHEN** user-service 以 metrics 启用配置构造正式 App
- **THEN** permission 模块 MUST 向 `PermissionQueryService` 注入当前 Prometheus Metrics 实现
- **AND** route diff 查询 MUST 能更新既有 `aegiscore_user_service_permission_route_diff` 指标

#### Scenario: metrics 禁用时注入 Nop 实现

- **WHEN** user-service 以 metrics 禁用配置构造正式 App
- **THEN** permission 模块 MUST 向 `PermissionQueryService` 注入现有 `permissionapplication.NopMetrics()` 实现
- **AND** 正式 App MUST 完成构图且 MUST NOT 注册或更新 permission Prometheus 指标

#### Scenario: DOT 图展示明确 Metrics 输入边

- **WHEN** 测试生成包含正式 `permission.Module` 的 Fx/DOT 依赖图
- **THEN** `PermissionQueryService` 构造节点 MUST 存在明确的 `permissionapplication.Metrics` 输入边
- **AND** 依赖图 MUST NOT 依赖 variadic、错误的 optional 或 slice/group annotation 补偿该输入

#### Scenario: 指标契约保持不变

- **WHEN** permission Metrics 的正式依赖接线被修复
- **THEN** 既有 metric family、指标名称、label key、label value 和低基数约束 MUST 保持不变
- **AND** 系统 MUST NOT 新增 metrics backend、dashboard 或 alert
