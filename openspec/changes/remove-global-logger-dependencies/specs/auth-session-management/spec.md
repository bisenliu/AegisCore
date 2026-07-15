## ADDED Requirements

### Requirement: 认证 feature 日志依赖显式化

auth feature 的 command、credentials、sessions、tokens、validators application 以及 Redis/PostgreSQL 等关键 infrastructure 在正式业务主路径记录日志时 MUST 使用 constructor 注入的 `*zap.Logger`、由注入 logger 派生的组件 logger，或 request context 中明确携带的 logger。认证会话、安全撤销、token version 和强制改密路径 MUST NOT 依赖可变进程级默认 logger 作为正式日志依赖。

#### Scenario: auth application 构造声明日志依赖
- **WHEN** 登录、refresh、改密、退出当前会话、退出全部会话、session lifecycle 或 token version validator 需要记录应用日志
- **THEN** 对应 constructor MUST 显式接收 `*zap.Logger` 或包含该 logger 的最小依赖结构
- **AND** constructor deps MUST 继续只暴露该组件真实需要的 collaborator，不得为日志迁移重新引入跨 use case 的公共依赖容器

#### Scenario: 安全撤销日志保留关联字段
- **WHEN** token version 本地缓存失效、Redis 投影刷新、refresh session 删除或强制改密撤销补偿失败需要记录日志
- **THEN** 日志 MUST 使用注入 logger 或 request context logger
- **AND** 既有 `user_id`、固定错误分类、`request_id`、`trace_id`、`span_id` 等可用关联信息 MUST 保持可用
- **AND** 日志 MUST NOT 暴露 token、jti、session ID、Redis key、SQL、password 或敏感原始错误

#### Scenario: auth infrastructure 不依赖全局默认 logger
- **WHEN** auth Redis/PostgreSQL adapter、password-change session store、token version cache adapter 或 metrics/provider 相关 infrastructure 记录运行时错误
- **THEN** logger MUST 从 constructor 显式注入或由调用方 context 提供
- **AND** 生产文件 MUST NOT 通过 package-level 默认 logger fallback 作为正式主路径日志来源

#### Scenario: auth 测试 App 日志互不覆盖
- **WHEN** 同一测试进程构造多个 auth use case fixture、provider fixture 或 user-service 测试 App
- **THEN** 每个 fixture MUST 使用自身注入的局部 logger 或 context logger
- **AND** 任一 fixture 调用 logger 构造函数 MUST NOT 改变其他 fixture 观察到的默认 logger

#### Scenario: 架构检查覆盖 auth feature
- **WHEN** 执行 `make user-service-architecture-lint`
- **THEN** 检查 MUST 能发现 auth feature application 或关键 infrastructure 重新依赖 package-level 默认 logger 的生产代码
- **AND** 必须覆盖默认 fallback 的 logger 包测试不属于 auth 正式主路径例外
