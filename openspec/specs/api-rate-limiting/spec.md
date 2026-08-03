# api-rate-limiting Specification

## Purpose
TBD - created by archiving change add-api-rate-limiting. Update Purpose after archive.
## Requirements
### Requirement: 匿名接口按 IP 限流

系统 MUST 对未认证即可访问的认证入口执行按客户端 IP 的限流。限流 MUST 在 controller 和业务用例执行前完成，超限请求 MUST 被拒绝并返回统一限流错误响应。

#### Scenario: 登录接口按 IP 超限

- **WHEN** 同一客户端 IP 在匿名认证限流窗口内超过允许请求数量访问 `POST /api/v1/auth/login`
- **THEN** 系统 MUST 在执行登录 controller 前拒绝请求
- **AND** 响应 MUST 为 `429 Too Many Requests`、限流错误 code 和 `success=false`

#### Scenario: 匿名 key 使用客户端 IP

- **WHEN** 未认证请求访问登录、refresh 或强制改密入口
- **THEN** 系统 MUST 使用 Gin 解析出的客户端 IP 作为限流身份 key
- **AND** 系统 MUST NOT 使用用户名、密码、refresh token、password-change token 或请求 body 字段作为匿名限流 key

### Requirement: 已认证业务接口按 User ID 限流

系统 MUST 对已认证接口执行按 User ID 的限流。User ID 限流 MUST 发生在 JWT 和 token version 校验通过之后、RBAC 授权和 controller 执行之前。

#### Scenario: 已认证请求按 User ID 超限

- **WHEN** 同一已认证 User ID 在业务限流窗口内超过允许请求数量访问 `/api/v1` 下受保护接口
- **THEN** 系统 MUST 在 RBAC 授权和 controller 执行前拒绝请求
- **AND** 响应 MUST 为 `429 Too Many Requests`、限流错误 code 和 `success=false`

#### Scenario: 认证失败不进入 User ID 限流

- **WHEN** 请求缺少 bearer token、token 无效、token 过期或 token version 不匹配
- **THEN** 系统 MUST 返回现有认证错误响应
- **AND** 系统 MUST NOT 为该请求创建或消费 User ID 限流 token

### Requirement: 本地限流器资源生命周期与清理

系统 MUST 使用 `golang.org/x/time/rate` 作为服务内本地 token bucket 限流实现，并 MUST 为每个限流 key 维护独立 limiter。本地 limiter store MUST 使用分片 map 降低锁竞争，并 MUST 通过后台 janitor 定期清理超过 TTL 未访问的 key。

#### Scenario: 后台清理过期 key

- **WHEN** 某个 IP 或 User ID 对应的限流 key 超过配置 TTL 未访问
- **THEN** 后台 janitor MUST 在后续清理周期删除该 key 对应的 limiter
- **AND** 该 key 后续再次访问时 MUST 创建新的 token bucket

#### Scenario: lifecycle 停止清理资源

- **WHEN** user-service 停止或 Fx lifecycle 取消限流资源
- **THEN** 系统 MUST 停止后台 janitor goroutine
- **AND** 停止过程 MUST NOT 关闭 Redis、PostgreSQL、Ent、Casbin 或其他共享资源

#### Scenario: 禁用限流

- **WHEN** user-service 配置禁用某类限流
- **THEN** 对应路由组 MUST 不消费 token bucket 并允许请求进入后续 middleware
- **AND** 禁用状态 MUST NOT 创建该类限流 janitor 或后台资源

### Requirement: 限流配置与默认值

user-service MUST 私有声明和校验 API 限流配置。配置 MUST 分别表达匿名 IP 限流和已认证 User ID 限流的启用状态、速率、突发容量、key TTL、清理间隔和分片数量。

#### Scenario: 默认限流配置

- **WHEN** 配置未显式声明 API 限流字段
- **THEN** user-service MUST 使用启用的匿名 IP 限流和已认证 User ID 限流默认值
- **AND** 默认值 MUST 包含正数速率、正数 burst、正数 key TTL、正数清理间隔和正数 shard 数

#### Scenario: 非法限流配置

- **WHEN** 已启用限流的速率、burst、key TTL、清理间隔或 shard 数为零值或非法值
- **THEN** 服务配置校验 MUST 在启动前失败
- **AND** 错误 MUST 包含对应配置字段路径

### Requirement: 单实例兜底边界

服务内限流 MUST 明确作为单实例兜底能力运行。系统 MUST NOT 将本地 limiter 描述为跨实例全局精确配额，也 MUST NOT 在本次能力中引入 Redis、数据库或消息队列作为每请求限流计数依赖。

#### Scenario: 多副本部署语义

- **WHEN** user-service 以多个副本运行
- **THEN** 每个副本 MUST 独立维护本地 limiter store
- **AND** 系统 MUST NOT 保证本次能力提供跨副本全局精确限流

