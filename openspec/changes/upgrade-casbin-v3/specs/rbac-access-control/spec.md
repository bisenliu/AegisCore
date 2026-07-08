## ADDED Requirements

### Requirement: Casbin v3 依赖升级兼容性

系统 MUST 将 user-service 直接依赖的 Casbin 主版本升级到最新稳定 v3，并保持 RBAC 授权、policy loader、超级管理员通配授权、用户角色缓存和 policy sync 的既有业务语义不变。升级后系统 MUST NOT 保留对 `github.com/casbin/casbin/v2` 或其子包的直接引用。

#### Scenario: 使用稳定 v3 模块路径
- **WHEN** 实现升级 Casbin 依赖
- **THEN** `user-service/go.mod` MUST 直接依赖 `github.com/casbin/casbin/v3` 的最新稳定版本
- **AND** user-service 代码和测试 MUST NOT import `github.com/casbin/casbin/v2` 或 `github.com/casbin/casbin/v2/model`

#### Scenario: 授权判断语义保持不变
- **WHEN** 已认证用户访问 RBAC 保护路由
- **THEN** 授权判断 MUST 继续使用用户当前启用角色、`role:<role_uuid>` subject、Gin route template 和 HTTP method 执行 Casbin `Enforce`
- **AND** policy 未加载、用户无启用角色、角色无匹配权限或底层 Casbin 返回错误时 MUST NOT 默认放行

#### Scenario: 超级管理员通配策略保持可用
- **WHEN** 用户拥有 `internal/shared/rbacbaseline` 中稳定的内置超级管理员角色
- **THEN** 升级后的 Casbin v3 enforcer MUST 继续识别 wildcard policy 并允许访问受保护业务接口
- **AND** 超级管理员角色常量 MUST 仍只由 `internal/shared/rbacbaseline` 提供

### Requirement: Casbin v3 API 变更检查与适配记录

实现 MUST 全面检查当前代码中所有 Casbin 旧用法，并对模块路径、model 子包、enforcer 构造、policy 写入、授权执行和测试 helper 给出明确适配。检查结果 MUST 记录在实现任务或提交说明中，并通过编译、测试和全仓搜索验证没有遗漏旧主版本引用。

#### Scenario: 旧用法替换完整
- **WHEN** 实现完成 Casbin v3 升级
- **THEN** `rg "github.com/casbin/casbin/v2|casbin/v2" common user-service tools --glob '*.go' --glob 'go.mod' --glob 'go.sum'` MUST 无命中
- **AND** 所有直接 Casbin API 调用 MUST 使用 v3 模块下的类型或函数

#### Scenario: 关键 API 行为验证
- **WHEN** 实现适配 `NewEnforcer`、`AddPolicy`、`Enforce` 和 `model` import
- **THEN** 测试 MUST 覆盖允许、拒绝、底层错误、未配置、通配策略、context 取消和 policy reload 失败路径
- **AND** 失败路径 MUST 保留当前错误区分和错误包装语义

#### Scenario: v3 新能力不改变线上授权路径
- **WHEN** 实现评估 Casbin v3 新增的 `Explain()`、detector 或其他诊断能力
- **THEN** 这些能力 MUST NOT 成为 request-time 授权 allow/deny 的必要依赖
- **AND** 如需采用这些能力优化诊断，MUST 仅作为离线 route diff、管理端排障或后续独立 change 的候选方案记录
