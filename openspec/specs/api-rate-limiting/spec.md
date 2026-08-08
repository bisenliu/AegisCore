# api-rate-limiting Specification

## Purpose

定义 user-service 的本地 API 限流能力，约束匿名与已认证请求的限流身份、中间件顺序、错误响应、配置和单实例运行边界。

## Requirements

### Requirement: API 限流身份与中间件门禁

系统 MUST 对公开认证入口按可信客户端 IP 限流，对已认证接口按 User ID 限流。匿名限流 MUST 在 auth controller 前执行；User ID 限流 MUST 在 JWT 与 token version 校验通过后、RBAC 授权和 controller 前执行。超限响应 MUST 使用统一错误 envelope 返回 `429 Too Many Requests`。

#### Scenario: 公开认证入口按客户端 IP 限流

- **WHEN** 同一客户端 IP 在限流窗口内超过阈值访问登录、refresh 或强制改密入口
- **THEN** 系统 MUST 在 auth controller 前拒绝请求，并返回 `429 Too Many Requests`、限流错误 code 和 `success=false`
- **AND** 限流 key MUST 来自 Gin trusted proxy 规则解析后的客户端 IP，MUST NOT 来自用户名、密码、token 或请求体字段

#### Scenario: 已认证接口按 User ID 限流

- **WHEN** bearer token 与 token version 校验通过，且同一 User ID 在限流窗口内超过阈值访问受保护接口
- **THEN** 系统 MUST 在 RBAC 授权和 controller 前拒绝请求，并返回统一限流错误响应
- **WHEN** bearer token 缺失、无效、过期或 token version 不匹配
- **THEN** 系统 MUST 返回既有认证错误，MUST NOT 创建或消费 User ID 限流 token

#### Scenario: 未超限请求继续既有安全链路

- **WHEN** 请求未超过对应限流阈值
- **THEN** 公开认证请求 MUST 继续进入 auth controller
- **AND** 已认证业务请求 MUST 继续执行 RBAC 授权和 controller，限流逻辑 MUST NOT 改写后续认证或授权错误

### Requirement: 本地限流配置、生命周期与部署边界

user-service MUST 私有声明匿名与已认证限流策略，并使用共享业务中立的本地 token bucket primitive。每类策略 MUST 支持启停、速率、burst、key TTL、清理间隔、分片数量、最大 Key 容量和容量耗尽策略；服务 MUST 清理过期 key、停止自有后台资源，并明确该能力只提供单实例配额。

#### Scenario: 默认值与配置校验

- **WHEN** 未显式配置 API 限流
- **THEN** 两类策略 MUST 使用启用状态和正数速率、burst、key TTL、清理间隔、分片数量及最大 Key 容量的完整默认值
- **WHEN** 已启用策略的任一数值为零或非法值，或容量耗尽策略不是支持值
- **THEN** 配置校验 MUST 在启动前失败，并报告对应配置字段路径

#### Scenario: 清理、禁用与停止

- **WHEN** 某个限流 key 超过 TTL 未访问
- **THEN** janitor MUST 在后续清理周期删除该 key，后续访问 MUST 在容量允许时创建新的 token bucket
- **WHEN** 某类限流被禁用
- **THEN** 对应请求 MUST 直接进入后续 middleware，系统 MUST NOT 为该类策略创建 limiter janitor
- **WHEN** user-service 停止
- **THEN** 系统 MUST 幂等停止限流后台资源，MUST NOT 关闭 Redis、PostgreSQL、Ent、Casbin 或其他共享资源

#### Scenario: limiter 内部错误保持可观察的 fail-open

- **WHEN** limiter 因 key 缺失、资源关闭或内部错误无法完成判定
- **THEN** user-service MUST 记录固定 scope、event 与 reason 的低基数观测事件，并允许请求继续后续安全 middleware
- **AND** 系统 MUST NOT 将 limiter 内部错误伪装成调用方超限；后续认证、授权和 controller 错误语义 MUST 保持不变

#### Scenario: 多副本配额语义

- **WHEN** user-service 以多个副本运行
- **THEN** 每个副本 MUST 独立维护本地 limiter store
- **AND** 系统 MUST NOT 将该能力描述为跨副本全局精确配额，也 MUST NOT 为每请求计数引入 Redis、数据库或消息队列依赖

#### Scenario: 唯一 Key 压力下条目数有界

- **WHEN** 超过最大 Key 容量数量的不同客户端 IP 或 User ID 在 TTL 窗口内持续访问对应限流入口
- **THEN** 本地限流器持有的 Key 条目数 MUST NOT 超过该策略配置的最大 Key 容量
- **AND** 已存在 Key 的 token bucket 状态 MUST 继续生效，MUST NOT 因新 Key 到达而被重置绕过

#### Scenario: 容量耗尽时使用 overflow bucket

- **WHEN** 策略配置为 overflow，且新 Key 到达时所属容量预算已耗尽
- **THEN** 系统 MUST NOT 为该新 Key 创建独立 limiter 条目
- **AND** 系统 MUST 使用共享 overflow token bucket 判定该请求是否允许通过
- **AND** user-service MUST 记录固定 scope、event 与 reason 的低基数观测事件

#### Scenario: 容量耗尽时拒绝新 Key

- **WHEN** 策略配置为 reject，且新 Key 到达时所属容量预算已耗尽
- **THEN** 系统 MUST NOT 为该新 Key 创建独立 limiter 条目
- **AND** 系统 MUST 拒绝该请求并返回统一限流错误响应
- **AND** user-service MUST 记录固定 scope、event 与 reason 的低基数观测事件

#### Scenario: 容量观测保持低基数

- **WHEN** 系统记录当前 Key 数、容量耗尽、overflow、拒绝或驱逐相关日志与 metrics
- **THEN** 观测标签 MUST 只使用固定 scope、event、reason 或 key_present 等低基数字段
- **AND** 观测数据 MUST NOT 使用原始客户端 IP、User ID、token、用户名或请求体字段作为标签值
