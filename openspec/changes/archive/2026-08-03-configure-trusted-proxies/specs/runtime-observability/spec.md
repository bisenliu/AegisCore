## ADDED Requirements

### Requirement: HTTP 日志客户端地址语义

系统 MUST 在 HTTP access log 和认证失败日志中使用 Gin trusted proxy 校验后的 `c.ClientIP()` 作为 `client_ip` 字段。日志 middleware MUST NOT 自行解析 `X-Forwarded-For`、`X-Real-IP` 或其他 forwarded headers；IP 是否可信 MUST 由 Gin engine 的 `server.http.trusted_proxies` 配置统一决定。

#### Scenario: Access log 记录真实客户端地址

- **WHEN** 请求来自已配置的 trusted proxy，且 forwarded headers 已由入口层清洗
- **THEN** HTTP access log 的 `client_ip` 字段 MUST 记录 Gin 解析后的真实客户端地址
- **AND** 日志字段 MUST NOT 额外记录完整 forwarded header 链路或未清洗原始 header

#### Scenario: 未信任代理时忽略 forwarded headers

- **WHEN** 请求来自未受信任 TCP peer
- **THEN** HTTP access log 和认证失败日志的 `client_ip` 字段 MUST 记录 TCP peer 地址
- **AND** 请求携带的 `X-Forwarded-For` 或 `X-Real-IP` MUST NOT 改变该字段
