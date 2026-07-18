## ADDED Requirements

### Requirement: RBAC application constructor 缺失依赖错误
permission 和 role application constructor MUST 将可预期的缺失 policy change notifier 或其他必需 collaborator 表达为明确 error，MUST NOT 使用 panic 作为正式装配失败路径。RBAC 写侧服务 MUST 在缺失通知能力时拒绝装配，避免在线策略同步失效后继续接受写操作。

#### Scenario: permission command 缺少 policy change notifier
- **WHEN** 构造 permission command service 时缺少 policy change notifier
- **THEN** constructor MUST 返回明确错误
- **AND** Fx graph MUST 通过标准 error path 拒绝装配
- **AND** constructor MUST NOT panic

#### Scenario: role command 缺少 policy change notifier
- **WHEN** 构造 role command service 时缺少 policy change notifier
- **THEN** constructor MUST 返回明确错误
- **AND** Fx graph MUST 通过标准 error path 拒绝装配
- **AND** constructor MUST NOT panic

#### Scenario: RBAC 写后同步能力不可降级
- **WHEN** 权限、角色、角色权限或用户角色写侧服务装配完成
- **THEN** 服务 MUST 具备可用的 policy change notifier
- **AND** 系统 MUST NOT 以 no-op、nil fallback 或兼容 wrapper 静默跳过 policy reload、Redis policy version 或 watcher 同步语义
