## ADDED Requirements

### Requirement: Permission HTTP method allowlist 不得暴露共享可写状态
permission domain 中用于权限目录和授权对象的 HTTP method allowlist MUST 使用不暴露共享可写底层状态的表达方式。实现 MUST 保持允许方法、大小写归一化、非法方法错误语义、route diff 和 RBAC 授权 action 语义不变。

#### Scenario: HTTP method allowlist 不可被包内误写
- **WHEN** permission domain 校验权限方法或构造 route identity
- **THEN** 允许的 HTTP method 集合 MUST 使用 `switch`、私有查询函数或等价不可共享写入的表达方式
- **AND** 系统 MUST NOT 暴露可被同包未来代码直接写入的 package-level map 作为 allowlist 权威来源

#### Scenario: method 校验语义保持不变
- **WHEN** 调用方传入当前允许的 HTTP method
- **THEN** 系统 MUST 继续接受并按当前规则归一化 method
- **AND** 调用方传入不允许的 method 时，系统 MUST 继续返回当前非法 method 错误语义

#### Scenario: RBAC 授权 action 不变
- **WHEN** 已认证用户访问 RBAC 保护路由
- **THEN** 授权判断 MUST 继续使用当前 HTTP method 作为 Casbin action
- **AND** 本次 allowlist 加固 MUST NOT 改变权限目录、route diff、policy loader、policy sync、超级管理员通配授权或授权失败响应语义
