## ADDED Requirements

### Requirement: RBAC Enforce 低基数指标

系统 MUST 为每次 permission authorization service 的 RBAC Enforce 判定导出低基数 Prometheus metrics，用于观察授权通过、授权拒绝、授权异常和授权耗时。指标 MUST 不改变 Casbin policy、用户角色缓存、policy sync、超级管理员通配授权或 HTTP 授权结果语义。

#### Scenario: 授权通过指标

- **WHEN** 已认证用户拥有当前 HTTP method 和 route template 对应权限，RBAC Enforce 返回允许
- **THEN** 系统 MUST 将 RBAC Enforce counter 记录为 `result="allow"`
- **AND** 系统 MUST 将本次 Enforce 耗时写入 RBAC Enforce latency histogram
- **AND** 指标标签 MUST 只包含 `result`、`method` 和 `route_template`

#### Scenario: 授权拒绝指标

- **WHEN** 已认证用户缺少当前 HTTP method 和 route template 对应权限，RBAC Enforce 返回拒绝
- **THEN** 系统 MUST 将 RBAC Enforce counter 记录为 `result="deny"`
- **AND** 系统 MUST 将本次 Enforce 耗时写入 RBAC Enforce latency histogram
- **AND** 系统 MUST 保持授权拒绝响应语义不变

#### Scenario: 授权异常指标

- **WHEN** RBAC Enforce 因非法 subject、context 取消、用户角色回源失败或 Casbin 执行失败返回错误
- **THEN** 系统 MUST 将 RBAC Enforce counter 记录为 `result="error"`
- **AND** 系统 MUST 将本次 Enforce 耗时写入 RBAC Enforce latency histogram
- **AND** 系统 MUST 保持 fail-closed 行为，不得因指标记录失败放行请求

#### Scenario: Enforce 指标禁止高基数字段

- **WHEN** 系统记录 RBAC Enforce metrics
- **THEN** 指标 MUST NOT 包含用户 ID、角色 ID、权限 ID、token ID、trace/span ID、raw path、IP、邮箱、用户名、Redis key、SQL、SQL 参数或原始错误
- **AND** route 标签 MUST 使用 Gin route template 或等价稳定模板，不得使用真实请求 path
