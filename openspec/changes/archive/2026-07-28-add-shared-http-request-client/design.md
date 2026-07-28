## Context

待迁移代码使用 Resty 在每次 `Send` 时创建 client，通过字符串 method 分支支持 GET、POST、PUT 和 DELETE，并把请求参数集中在一个结构体中。实现默认设置 `tls.Config{InsecureSkipVerify: true}`，代理由 `map[string]string` 拼接，非成功响应会丢弃 body，也无法接收调用方 context。

本仓库的 `common/http` 适合承载业务中立 HTTP primitive，但 `common` 不应拥有外部系统 DTO、认证协议或具体 retry policy。本 change 使用 Resty 统一请求构造与发送，同时允许消费侧通过注入长期存活的 Resty client 扩展 middleware、transport、证书、response limit 和经过业务评审的 retry。

## Goals / Non-Goals

**Goals:**

- 保留集中声明 URL、method、query、JSON/form、header、proxy 和 timeout 的简单调用方式。
- 默认复用长期存活的 Resty client，并允许调用方注入自己拥有的 `*resty.Client`。
- 支持 context 取消、默认 timeout、安全 TLS、显式代理和可检查的 HTTP 状态错误。
- 保留注入 client 上已有的 retry、middleware、transport、TLS 和 response body limit 配置。
- 覆盖请求编码、错误响应、timeout、TLS、代理和 Resty client 注入测试。

**Non-Goals:**

- 不在默认 Resty client 上启用 retry、debug logging、认证、cookie jar 或 tracing instrumentation。
- 不定义外部服务 DTO、认证 header、签名、重试条件、业务错误映射或日志策略。
- 不替换 Nacos source、healthcheck 或其他已有专用 HTTP client。
- 不允许 helper 隐式关闭 TLS 证书校验。

## Decisions

### Decision: 使用长期复用并可注入的 Resty client

`SendRequest.RestyClient` 接收调用方拥有的 `*resty.Client`；缺省时使用包内长期存活的默认 client。默认 client 通过 `resty.NewWithClient(&http.Client{})` 创建，不安装 cookie jar，不启用 retry 或 debug logging，并复用 Go 默认 transport 连接池。

发送过程只创建 request-level `*resty.Request`，不调用会修改 client-level 配置的 setter。调用方注入 client 上的 middleware、retry、transport、TLS、redirect、rate limit 和 response body limit 继续生效。

备选方案是每次 `Send` 调用 `resty.New()`。该方案无法复用无代理请求的 client lifecycle，也无法稳定承载消费侧统一配置。

### Decision: timeout 只通过 request context 实现

Resty v2 的 timeout setter 属于 client level。为避免并发请求互相修改共享 client，`SendContext` 使用 `context.WithTimeout` 为本次请求设置预算：零值回退到 60 秒，负值在发送前失败，调用方更早的 deadline 仍优先生效。`Send` 仅作为使用 `context.Background()` 的便捷入口。

### Decision: form 优先于 JSON，并委托 Resty 编码

当 `FormData` 非空时调用 Resty `SetFormData`，由 Resty 编码并设置 `application/x-www-form-urlencoded`；否则当 `JSONData` 非 nil 时调用 `SetBody`，由 Resty 根据 body 和 header 编码。method 使用 Resty `Execute`，不维护容易遗漏 PATCH、HEAD 等方法的本地分支。

### Decision: 默认安全 TLS 与代理所有权

helper 不调用 `SetTLSClientConfig`，默认保持 Go TLS 证书校验。需要自定义 CA、mTLS 或其他 TLS 行为时，调用方配置长期存活的 Resty client 后注入。

`ProxyURL` 必须是带 host 的绝对 `http` 或 `https` URL。便捷 `ProxyURL` 只允许在未注入 client 时使用，并为该次发送创建、关闭专用 Resty client；固定或高频代理必须预先配置在注入 client 上，避免 helper 修改共享 client 或使用并发不安全的 `Client.Clone()`。

### Decision: Resty 执行错误与 HTTP 状态错误分离

全部 2xx 状态返回 `success=true`。实际收到非 2xx 响应时返回 `success=false`、response body 和 `*StatusError`，错误文本只包含状态码。Resty 构造、middleware、body limit、context、TLS 或 transport 错误返回 `success=false`、nil body 和 wrapped error。

默认 client 不启用 retry；调用方注入 client 后，其重试策略可以生效，但重试条件、幂等性和副作用风险由消费侧外部集成负责。

## Risks / Trade-offs

- [风险] 注入 client 的重试策略可能重复执行非幂等请求。 -> 缓解：common 默认不启用 retry；消费侧必须按 method、状态和幂等键定义重试条件。
- [风险] 便捷 `ProxyURL` 每次创建专用 client，无法跨请求复用代理连接池。 -> 缓解：高频代理调用方应注入预配置并可复用的 Resty client，不再设置 `ProxyURL`。
- [风险] 默认 Resty client 未限制响应体大小。 -> 缓解：保持待迁移 API 返回完整 `[]byte` 的契约；不可信或大响应集成应注入设置 `SetResponseBodyLimit` 的 client，流式场景使用专用 adapter。
- [风险] Resty 升级可能改变编码或 middleware 行为。 -> 缓解：锁定 v2 minor 版本并通过 form、JSON、状态、timeout、TLS、proxy 和 retry 测试约束本包契约。
- [风险] form 和 JSON 同时设置时存在歧义。 -> 缓解：明确 form 优先，保持待迁移实现语义并通过测试约束。

## Migration Plan

1. 新增 OpenSpec delta，明确 Resty client 所有权、出站 helper 范围和安全语义。
2. 在 `common` 引入 `github.com/go-resty/resty/v2`，新增 `common/http/client` 实现和单元测试。
3. 更新 `docs/ARCHITECTURE.md` 的 `common/http` 路径说明。
4. 运行 common 包测试、OpenSpec validation、architecture lint、lint 和 verify。

回滚方式：删除新增包和 Resty 依赖，并回滚文档入口与本 change artifacts；现有生产代码未切换到该 helper，因此不需要数据或部署回滚。

## Open Questions

无。
