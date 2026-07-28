## ADDED Requirements

### Requirement: Gin 入站 HTTP 观测路径低基数契约

系统 MUST 对 Gin 入站 HTTP 观测路径使用低基数 route template。access log、认证失败日志、绑定或校验失败日志、HTTP metrics route label、trace span name、授权 object 和默认观测过滤判断 MUST 使用 Gin route template 或固定 `__unmatched__`，MUST NOT 发出或依赖原始 URL path。

#### Scenario: 匹配动态业务路由的观测路径
- **WHEN** Gin 入站请求匹配包含路径参数的业务 route template，例如 `/api/v1/users/:user_id`
- **THEN** 日志 `path` 字段、HTTP metrics route label、trace span name 和授权 object MUST 使用该 route template
- **AND** 系统 MUST NOT 在这些默认观测字段中写入真实用户 ID、角色 ID、权限 ID、session ID、tenant、cursor、UUID 或其他 raw path 参数值

#### Scenario: 未匹配路由的观测路径
- **WHEN** Gin 入站请求未匹配任何 route template
- **THEN** 日志 `path` 字段、HTTP metrics route label、trace span name 和授权 object MUST 使用固定值 `__unmatched__`
- **AND** 系统 MUST NOT 回退到 `c.Request.URL.Path`、`request.URL.Path` 或等价 raw URL path

#### Scenario: 请求绑定和校验失败日志
- **WHEN** Gin 入站请求在 binding 或 validation 阶段失败
- **THEN** 失败日志 MUST 使用匹配 route template 或 `__unmatched__` 作为 `path` 字段
- **AND** 失败日志 MUST NOT 因请求体绑定失败、参数校验失败或上下文尚未进入 feature controller 而记录 raw URL path

#### Scenario: runtime endpoint 观测跳过判断
- **WHEN** 系统判断 `/metrics`、`/livez`、`/readyz` 或 `/startupz` 等 runtime endpoint 是否跳过成功请求日志、请求计数或请求耗时
- **THEN** 判断输入 MUST 是 route template 或显式静态配置归一化结果
- **AND** Gin 入站观测跳过判断 MUST NOT 使用 raw URL path

#### Scenario: tracing 过滤与 span name
- **WHEN** OTel Gin instrumentation 处理入站 HTTP 请求
- **THEN** 应用内 Gin tracing 逻辑 MUST NOT 在 route match 前基于 `request.URL.Path` 过滤请求
- **AND** HTTP server span name MUST 使用 `METHOD <route template>` 或 `METHOD __unmatched__`
- **AND** 低噪声 tracing 过滤如仍需要，MUST 在应用外或基于稳定 route/span name 执行，MUST NOT 依赖 raw URL path

#### Scenario: HTTP metrics route fallback
- **WHEN** HTTP metrics middleware 记录未匹配 Gin 入站请求
- **THEN** route label fallback MUST 固定为 `__unmatched__`
- **AND** 公共 middleware 配置 MUST NOT 允许调用方提供可能包含 raw path 或高基数值的 route fallback
