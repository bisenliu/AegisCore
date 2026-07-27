## Why

仓库当前为入站 HTTP 处理提供了 binding、middleware 和 response helper，但跨服务发起出站 HTTP 请求时仍需重复处理 query、header、JSON/form body、timeout、代理和非成功状态。待迁移代码已经使用 Resty，但每次 `Send` 都新建 client、默认跳过 TLS 证书校验，并用无类型 map 拼接代理，无法安全复用连接池或承载消费侧统一的 retry、middleware、证书和 response limit 配置。

需要在 `common/http` 中提供一个业务中立、基于长期复用 Resty client 且默认安全的请求 helper，统一基础发送语义，同时让具体外部系统的端口、DTO、防腐映射和 retry policy 继续由消费服务拥有。

## What Changes

- 新增 `common/http/client`，提供 `SendRequest`、`NewRequest`、`Send` 和支持取消/截止时间的 `SendContext`。
- 引入 `github.com/go-resty/resty/v2`，支持 query、header、JSON body、form body、逐请求 timeout、显式代理 URL和调用方注入的 `*resty.Client`。
- 默认复用无 cookie jar、无 retry、无 debug logging 的 Resty client；调用方注入 client 时保留其 retry、middleware、transport、TLS 和 response body limit 行为。
- 默认 timeout 为 60 秒并通过 request context 实现，不修改共享或注入 client；非法 timeout、URL、method 或代理在网络请求前失败。
- 默认保持 Go TLS 证书校验，不提供隐式跳过校验的开关；特殊 TLS transport 必须由调用方通过 `*resty.Client` 显式注入。
- 全部 2xx 状态视为成功；其他状态返回可检查的状态错误和响应体，Resty 执行错误返回 nil body。
- 不在默认 client 中配置业务重试、认证、熔断、DTO 或外部系统适配。

## Capabilities

### New Capabilities

无。

### Modified Capabilities

- `shared-platform-primitives`: 新增基于 Resty 的业务中立出站 HTTP 请求 helper 及其 client 所有权、安全、timeout、代理和错误语义。

## Impact

- 代码影响：新增 `common/http/client` 包及单元测试。
- 依赖影响：`common` 新增 `github.com/go-resty/resty/v2 v2.17.2`。
- 安全影响：移除待迁移实现中的默认 `InsecureSkipVerify` 行为，代理必须是可解析的绝对 HTTP(S) URL。
- 架构影响：具体外部 HTTP 集成仍属于 `user-service/internal/integration/http`；其专用 Resty client 可配置 retry、middleware、认证和证书后注入共享 helper。
- 不影响 HTTP API、OpenAPI 生成物、数据库 schema、migration、部署资产或现有服务运行时行为。
