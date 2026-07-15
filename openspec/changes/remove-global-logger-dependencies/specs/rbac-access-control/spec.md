## ADDED Requirements

### Requirement: RBAC feature 日志依赖显式化

permission 和 role feature 的 application、policy sync、Casbin authorization、RBAC seed、Redis watcher、PostgreSQL adapter 和其他关键 infrastructure 在正式业务主路径记录日志时 MUST 使用 constructor 注入的 `*zap.Logger`、由注入 logger 派生的组件 logger，或 request context 中明确携带的 logger。RBAC 授权、policy reload、用户角色缓存失效和跨副本同步路径 MUST NOT 依赖可变进程级默认 logger 作为正式日志依赖。

#### Scenario: permission application 构造声明日志依赖
- **WHEN** 权限目录 command/query、授权服务、route diff、policy reload coordinator 或 policy sync 组件需要记录日志
- **THEN** 对应 constructor MUST 显式接收 `*zap.Logger` 或包含该 logger 的最小依赖结构
- **AND** logger 迁移 MUST NOT 改变权限目录、授权判断、policy reload、用户角色缓存失效或 Redis policy version 发布语义

#### Scenario: role application 构造声明日志依赖
- **WHEN** 角色 command/query、用户角色绑定、角色权限绑定、RBAC seed 或超级管理员绑定流程需要记录日志
- **THEN** 对应 constructor MUST 显式接收 `*zap.Logger` 或包含该 logger 的最小依赖结构
- **AND** logger 迁移 MUST NOT 改变角色、权限、绑定、seed 或超级管理员业务结果

#### Scenario: RBAC infrastructure 不依赖全局默认 logger
- **WHEN** Casbin engine、policy loader、Redis watcher、PostgreSQL adapter、本地用户角色缓存或 policy change notifier 记录错误、reload 结果或后台同步状态
- **THEN** logger MUST 从 constructor 显式注入或由调用方 context 提供
- **AND** 生产文件 MUST NOT 通过 package-level `logger.Info`、`logger.Warn`、`logger.Error`、`logger.Debug` 或 `logger.NamedComponent(nil, ...)` 作为正式主路径日志来源

#### Scenario: RBAC policy sync 日志契约保持不变
- **WHEN** 在线 RBAC 写操作触发本实例 policy reload、用户角色缓存失效或 Redis policy version 发布
- **THEN** 日志依赖来源迁移 MUST NOT 改变 fail-closed 授权、同步失败向调用方传播、周期性版本补偿或 Pub/Sub payload 语义
- **AND** 日志字段 MUST 继续保持低基数并不得包含用户 ID 以外的高敏感明文、Redis key、SQL、token 或原始策略数据

#### Scenario: 架构检查覆盖 role 与 permission feature
- **WHEN** 执行 `make user-service-architecture-lint`
- **THEN** 检查 MUST 能发现 role 或 permission feature application、policy sync 或关键 infrastructure 重新依赖 package-level 默认 logger 的生产代码
- **AND** 生成 mock、测试 fixture 或显式局部 logger 使用不得被要求引入生产冗余接口
