## MODIFIED Requirements

### Requirement: 跨服务 HTTP 契约与 helper

系统 MUST 在 `common/contract` 中维护业务中立的应用错误、响应 envelope 和分页契约，由 `common/http/response` 统一完成 HTTP 渲染，并 MUST 在 `common/http` 和 `common/validation` 中提供行为一致、业务中立的入站绑定、字段校验、认证授权 middleware、CORS、metrics、logging、recovery、OpenAPI 和轻量出站请求能力。应用错误 MUST 使用低基数 `Kind`、稳定 `Reason`、响应 `Code`、公开 `Message` 和可选内部 `Cause` 表达语义，MUST NOT 保存或接收 HTTP status；HTTP status MUST 只根据 `Kind` 推导。

#### Scenario: 响应、错误归一化与 HTTP 映射

- **WHEN** 服务返回成功、分页或错误响应
- **THEN** 系统 MUST 使用共享 envelope 表达 `success`、`code`、`message`、`data`、`pagination` 或结构化错误详情，新服务 MUST 优先复用 `common/contract` 和 `common/http/response`，不得定义不兼容的 envelope
- **WHEN** 系统创建、包装或通过 `FromError` 归一化错误
- **THEN** wrapped application error MUST 保留原始 `Kind`、`Reason`、`Code` 和公开 `Message`
- **AND** nil 或未知错误 MUST 归一化为使用非敏感公开消息的内部错误，原始错误只能作为内部 `Cause` 保留
- **AND** `errors.As` MUST 能解析应用错误，`errors.Is` MUST 按稳定 `Kind`、`Reason` 或内部 `Cause` 语义匹配
- **WHEN** `common/http/response` 写入应用错误响应
- **THEN** 请求格式或字段校验错误 MUST 渲染为 `400 Bad Request`
- **AND** 未认证或 token 无效、过期、撤销 MUST 渲染为 `401 Unauthorized`
- **AND** 权限不足、未找到、冲突和服务不可用 MUST 分别渲染为 `403 Forbidden`、`404 Not Found`、`409 Conflict` 和 `503 Service Unavailable`
- **AND** nil、未知或内部错误 MUST 渲染为 `500 Internal Server Error`

#### Scenario: HTTP 请求处理与 helper

- **WHEN** HTTP 请求被 Gin 路由处理
- **THEN** 服务 MUST 能复用共享 middleware 完成认证上下文、授权检查、日志字段、metrics、panic recovery 和 span error 记录
- **WHEN** HTTP handler 绑定请求或校验字段
- **THEN** `common/validation`、`common/http/binding` 和 response helper MUST 生成或传播语义应用错误，并返回一致的字段名、公开消息和结构化字段错误明细
- **AND** validation tag 的字段名解析顺序 MUST 保持稳定
- **WHEN** 调用方使用 `Created` 或 `NoContent` 写入响应
- **THEN** `Created` MUST 返回包含统一成功 envelope 和调用方 `data` 的 `201 Created`，`NoContent` MUST 返回 body 为空的 `204 No Content`

#### Scenario: 业务中立的出站 HTTP 请求

- **WHEN** 调用方使用共享 HTTP client helper 发送出站请求
- **THEN** helper MUST 基于 Resty 支持 query、header、JSON 或 form body、context、逐请求 timeout 和显式 HTTP(S) proxy URL，并 MUST 允许注入调用方拥有的 `*resty.Client`
- **AND** form 与 JSON 同时存在时 MUST 使用 form，并由 Resty 设置 `application/x-www-form-urlencoded`
- **AND** 零值 timeout MUST 使用 60 秒默认值，负值、空 URL、空 method 或非法 proxy MUST 在网络请求前失败
- **AND** helper MUST 使用调用方 context 表达逐请求 timeout，MUST NOT 为设置 timeout 修改共享或注入 client
- **AND** 默认 Resty client MUST 长期复用、MUST NOT 保存 cookie、启用 retry、记录请求或响应 body、注入业务认证信息；调用方注入 client 时 MUST 保留其已有 middleware、retry、transport、TLS 和 response body limit 行为
- **AND** `ProxyURL` 与注入 client 同时存在时 MUST 在网络请求前失败；固定或高频代理 MUST 由调用方预先配置在注入 client 上
- **WHEN** 调用方未注入自定义 TLS transport
- **THEN** helper MUST 保持 Go 默认 TLS 证书校验，MUST NOT 默认或通过隐式选项跳过证书校验
- **WHEN** 上游返回 HTTP 响应
- **THEN** 全部 2xx 状态 MUST 返回成功和完整 response body，其他状态 MUST 返回可检查的状态错误和 response body，错误文本 MUST NOT 包含 response body
- **AND** Resty 构造、middleware、body limit、context、TLS 或 transport 错误 MUST 返回失败、nil body 和可包装的原始错误
- **AND** 具体外部系统的 DTO、认证、重试、业务错误映射和防腐逻辑 MUST 留在消费服务的 `internal/integration/http` 或所属 feature 边界

#### Scenario: CORS 默认策略与预检

- **WHEN** 请求经过 `CORS()` middleware
- **THEN** 响应 MUST 默认包含 `Access-Control-Allow-Origin=*`、`Access-Control-Allow-Methods=GET,POST,PUT,PATCH,DELETE,OPTIONS` 和 `Access-Control-Allow-Headers=Authorization,Content-Type`
- **AND** 默认配置 MUST NOT 启用 credentials、max age、exposed headers 或 `Vary: Origin`
- **AND** middleware MUST 复制默认或调用方传入的 slice，调用方后续修改 MUST NOT 改变已创建 middleware 的行为
- **WHEN** `OPTIONS` 预检请求经过共享 CORS middleware
- **THEN** middleware MUST 返回带默认 CORS header 的 `204 No Content` 并停止调用后续 handler
- **AND** 非 `OPTIONS` 请求 MUST 调用后续 handler，并保持其 status 和 body 可见

#### Scenario: 业务语义与 OpenAPI 留在所属边界

- **WHEN** auth、permission、role 或 `internal/shared/identity` 定义稳定业务错误
- **THEN** owning domain MUST 为错误提供共享契约要求的 `Kind`、`Reason`、`Code` 和公开 `Message`，保持 `errors.Is` 匹配语义，并使 `common/http/response.Fail` 能直接渲染该错误
- **AND** 系统 MUST NOT 在 `common`、跨 feature 全局包或 HTTP transport 中维护重复的业务错误映射表
- **AND** `CodePasswordChangeRequired` 的数值 MUST 保持为 `20006`，但 `common` MUST NOT 承载 user-service 的状态判断、token 签发或登录编排
- **WHEN** 授权依赖 user-service 的 subject schema、角色、权限目录、route diff 或超级管理员基线
- **THEN** 行为 MUST 留在 user-service permission 或 shared 边界，不得进入通用 HTTP middleware 或 `common/security/casbin`
- **WHEN** 服务生成、转换或嵌入 OpenAPI 文档
- **THEN** 系统 MUST 复用 `common/http/openapi` 的规范化、序列化和 Go embed 渲染能力，API server、认证方案、扫描范围、健康路径和输出目录等服务元数据 MUST 留在服务脚本或薄 wrapper
