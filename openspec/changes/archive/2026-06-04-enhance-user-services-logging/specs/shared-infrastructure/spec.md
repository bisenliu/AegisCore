## ADDED Requirements

### Requirement: Provide reusable trace-aware logger API
共享基础设施 MUST 提供可复用的 trace-aware logger API，供服务侧通过 context 输出结构化日志。该 API MUST 复用 Zap logger 配置、时间戳、level、caller 和 writer 策略，并 MUST 使用统一字段名 `trace-id` 关联请求链路。服务侧 MUST NOT 为业务日志复制 logger 初始化、writer 或 trace-id 中间件实现。

#### Scenario: Service logs through context logger
- **Given** HTTP 请求经过 trace-id 中间件并进入业务 service
- **When** service 通过共享 context logger 输出日志
- **Then** 日志 MUST 包含 `trace-id` 字段
- **Then** 日志 MUST 保持共享 Zap encoder 的 `time`、`level`、`msg` 和 caller 格式

#### Scenario: User service does not duplicate logger infrastructure
- **Given** 用户服务需要增强业务日志字段
- **When** 实现日志增强
- **Then** 用户服务 MUST 复用 `common/logger` 和共享 trace-id 中间件
- **Then** 用户服务 MUST NOT 新增独立 logger 初始化、日志 writer、日志文件切分或 trace-id 生成实现

#### Scenario: Error logs can include stack trace helper
- **Given** service 记录系统错误级别日志
- **When** 调用共享 logger 堆栈 helper
- **Then** 日志 MUST 使用统一堆栈字段名记录错误堆栈
- **Then** warn 级业务拒绝日志 MUST NOT 默认携带堆栈

### Requirement: HTTP request logger classifies levels and user identity
共享 HTTP 请求日志中间件 MUST 根据响应状态码区分日志级别，并 MUST 在请求完成日志中包含 `user_id` 字段。已认证请求 MUST 记录认证上下文中的用户 UUID；无法从上下文获取用户标识时 MUST 记录 `user_id=anonymous`。请求日志 MUST 继续包含 `trace-id`、method、path、status、latency 和 client_ip。

#### Scenario: Successful request logs info with anonymous user
- **Given** 请求未经过认证或上下文中没有认证用户标识
- **When** `RequestLogger` 记录 2xx 或 3xx 请求完成日志
- **Then** 日志 MUST 使用 info 级别
- **Then** 日志 MUST 包含 `user_id=anonymous`
- **Then** 日志 MUST 包含 `trace-id`、method、path、status、latency 和 client_ip

#### Scenario: Client error request logs warn
- **Given** 请求完成后的响应状态码为 4xx
- **When** `RequestLogger` 记录请求完成日志
- **Then** 日志 MUST 使用 warn 级别
- **Then** 日志 MUST 包含认证上下文中的 `user_id`，无法获取时为 `anonymous`

#### Scenario: Server error request logs error
- **Given** 请求完成后的响应状态码为 5xx
- **When** `RequestLogger` 记录请求完成日志
- **Then** 日志 MUST 使用 error 级别
- **Then** 日志 MUST 包含认证上下文中的 `user_id`，无法获取时为 `anonymous`
