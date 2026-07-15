## ADDED Requirements

### Requirement: Permission route diff 指标反映正式模块执行结果

系统通过正式 `permission.Module` 执行 route diff 查询时 MUST 使用该模块注入的 `permissionapplication.Metrics` 实现，并在差异计算成功后记录准确的 missing 与 stale 数量。指标记录 MUST NOT 改变 route diff 的业务判定、只读语义、HTTP response、权限目录或 Casbin policy。

#### Scenario: 正式模块记录 route diff 结果

- **WHEN** 正式 `permission.Module` 构造的 `PermissionQueryService` 成功完成一次 route diff 查询
- **THEN** 系统 MUST 调用所注入 Metrics 实现的 `RouteDiffObserved`
- **AND** 调用参数 MUST 等于本次结果中的 missing 与 stale 数量

#### Scenario: 模块级 spy 验证指标协作者

- **WHEN** 测试从正式 `permission.Module` 构图注入 spy `permissionapplication.Metrics` 并执行 route diff 查询
- **THEN** spy MUST 观察到 `RouteDiffObserved` 调用
- **AND** 测试 MUST NOT 通过直接手工构造 query service 绕过正式模块接线

#### Scenario: route diff 业务语义保持不变

- **WHEN** 系统接入必选 Metrics 依赖并记录 route diff 数量
- **THEN** missing、stale、mismatch 判定、排序、错误传播和只读行为 MUST 保持不变
- **AND** 系统 MUST NOT 因记录指标而创建权限、修改权限状态、绑定角色或刷新 Casbin policy
