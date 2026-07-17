## MODIFIED Requirements

### Requirement: 认证能力边界与私有配置

user-service auth feature MUST 私有拥有 token issuer、claims schema、subject 常量、TTL fallback 和认证策略配置；`common/security/auth` MUST 只提供通用验证原语，不得拥有 user-service token 签发入口或专属 claims。认证 application MUST 通过消费侧最小 port 和窄 settings 编排凭据、token、session 与版本行为，MUST NOT 依赖 HTTP transport DTO、完整运行时配置或 Fx/Dig 语义。auth PostgreSQL/Redis adapter 和 HTTP controller constructor MUST 使用普通参数或无 DI tag 的 feature-local Options，MUST NOT 嵌入 `fx.In`、`fx.Out`，也 MUST NOT 在 Options 或 params 字段上声明 DI named tag。

#### Scenario: 服务私有签发与配置

- **WHEN** user-service 签发 token 或装配认证流程
- **THEN** 系统 MUST 从服务私有配置读取 JWT TTL、password KDF 预算、refresh rotation、token version cache TTL 和每用户活跃 session 上限
- **AND** `common/runtime/config` MUST NOT 声明或校验这些业务策略
- **AND** production-like 环境中的 `auth.jwt.secret` MUST 至少为 32 bytes，校验错误 MUST 明确定位该配置项

#### Scenario: Application 最小依赖

- **WHEN** 构造登录、refresh、改密或退出 use case
- **THEN** constructor MUST 只接收该 use case 所需的 collaborator 和窄 settings
- **AND** application command 包 MUST NOT 导入 `go.uber.org/fx`，也 MUST NOT 通过跨 use case 公共依赖容器暴露无关依赖
- **AND** refresh use case MUST 只接收 rotation 所需窄 settings，MUST NOT 接收完整 `*config.Config`

#### Scenario: Adapter 与 Controller 构造边界

- **WHEN** 构造 auth PostgreSQL credential store、Redis session store 或 HTTP controller
- **THEN** constructor MUST 使用普通参数或无 DI tag 的 feature-local Options
- **AND** constructor 输入类型 MUST NOT 嵌入 `fx.In`、`fx.Out` 或依赖 `go.uber.org/dig`
- **AND** constructor 输入字段 MUST NOT 声明 `name`、`group` 或其他 DI tag
- **AND** Fx named resource 适配 MUST 留在服务 composition 边界，MUST NOT 泄漏到 auth infrastructure 或 HTTP transport constructor API

#### Scenario: SessionStore 窄依赖

- **WHEN** 构造 auth Redis session store
- **THEN** constructor MUST 只接收 Redis client、Redis key catalog、所需 auth 配置值、metrics、logger 和后台清理所需窄接口
- **AND** Redis session store MUST NOT 接收完整 `*config.Config` 或把 service config 当作 DI 容器
- **AND** token version cache TTL、refresh session TTL fallback、password-change session TTL fallback 和 Redis key 生成语义 MUST 保持不变

#### Scenario: 分层存储边界

- **WHEN** 新增凭据、token、session、token version 或撤销行为
- **THEN** 业务编排 MUST 位于 auth application 或 domain，Redis 和 PostgreSQL adapter MUST 只实现消费侧最小存储 port
- **AND** token version 持久化、Redis 投影、refresh session 和本地失效器 MUST 通过可独立依赖的接口表达

#### Scenario: 显式日志依赖

- **WHEN** auth application 或关键 Redis/PostgreSQL infrastructure 记录正式业务日志
- **THEN** logger MUST 由 constructor 显式注入或从 request context 获取，MUST NOT 依赖可变 package-level 默认 logger
- **AND** 撤销失败日志 MUST 保留可用的 `user_id`、错误分类、`request_id`、`trace_id` 和 `span_id`
- **AND** 日志 MUST NOT 暴露 token、jti、session ID、Redis key、SQL、密码或敏感原始错误
