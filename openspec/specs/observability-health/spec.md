# 可观测性与健康检查规格

## 需求

### 需求：指标运行时
指标配置必须位于 `observability.metrics`。启用模式必须使用独立 Prometheus registry，并由服务显式挂载配置化 endpoint。禁用模式必须零副作用。

#### 场景：启用指标
Given `observability.metrics.enabled` 为 true
When user-service 启动
Then 必须在 `/api/v1` 外暴露配置化指标路径，默认 `/metrics`，并且不经过 RBAC 授权。

#### 场景：指标标签
Given 记录指标
When 选择标签
Then 标签必须保持低基数，不得包含用户 ID、角色 ID、权限 ID、会话 ID、token ID、trace/span ID、原始路径、IP、邮箱、用户名、SQL、Redis key 或原始错误。

### 需求：链路追踪与日志
链路追踪必须使用 W3C `traceparent` / `tracestate`。日志 context helper 必须从有效 OTel span context 派生 `trace_id` 和 `span_id`，不得伪造。

#### 场景：无有效 span
Given 当前 context 没有有效 OTel span context
When 通过 context helper 写日志
Then 必须省略 `trace_id` 和 `span_id` 字段。

#### 场景：敏感日志
Given 记录认证失败或请求错误
When 选择日志字段
Then 不得记录 password、token、Authorization header、Cookie、原始请求体、DSN、SQL、Redis key 或敏感原始错误。

#### 场景：日志语言和字段
Given 正式代码写结构化日志
When 选择日志消息和字段
Then 日志消息必须使用英文，字段名必须使用稳定英文 `snake_case`；不得记录 password、token、Authorization header、Cookie 或原始请求体。

#### 场景：日志等级
Given 发生预期业务拒绝
When 写日志
Then 不得使用 `Error` 级别。

#### 场景：系统异常日志等级
Given 发生系统异常、外部依赖失败、后台任务失败或 panic recover
When 写日志
Then 不得降级为 `Info`。

#### 场景：认证失败安全日志
Given 认证失败或安全拒绝
When 记录客户端审计上下文
Then 可以记录 `user_agent`、client IP 和低敏上下文字段，但不得记录凭据、token、Authorization header、Cookie 或原始请求体。

### 需求：健康探针
user-service 必须暴露 `/livez`、`/readyz` 和 `/startupz`。存活检查只表示进程可响应。就绪检查和启动检查必须检查 PostgreSQL `user_db`、Redis `cache_redis`、Casbin 策略状态和 RBAC 策略 watcher 状态。

#### 场景：Redis 不可用
Given Redis `cache_redis` 不可用
When 调用 `/readyz` 或 `/startupz`
Then endpoint 必须返回 HTTP 503，且不得暴露 secret、stacktrace、DSN、SQL、token 或 Cookie。

#### 场景：依赖不可用
Given PostgreSQL 或 RBAC 策略状态不健康
When 调用 `/livez`
Then 存活检查可以继续成功，因为它只证明进程可响应。
