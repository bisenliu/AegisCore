## ADDED Requirements

### Requirement: HTTP trusted proxy 配置契约

系统 MUST 在共享 runtime HTTP server 配置中提供 `server.http.trusted_proxies`，用于声明 Gin 可信任的上游代理 IP 或 CIDR 列表。该配置 MUST 由 `common/runtime/config` 严格解码、默认保持空值，并由服务级 Gin engine 初始化直接传入 `SetTrustedProxies`；系统 MUST NOT 读取、迁移、兼容或双写 `http.trusted_proxies` 或其他旧配置位置。

#### Scenario: 默认不信任代理

- **WHEN** `server.http.trusted_proxies` 未配置或为空
- **THEN** Gin engine MUST 不信任任何代理
- **AND** `c.ClientIP()` MUST 忽略 `X-Forwarded-For` 和 `X-Real-IP`，只返回请求 TCP peer 地址

#### Scenario: 显式信任代理 CIDR

- **WHEN** `server.http.trusted_proxies` 包含请求 TCP peer 所属的 IP 或 CIDR
- **THEN** Gin engine MUST 使用 Gin trusted proxy 机制解析 `X-Forwarded-For` 或 `X-Real-IP`
- **AND** `c.ClientIP()` MUST 返回可信代理链解析后的客户端地址

#### Scenario: 拒绝旧配置位置

- **WHEN** 配置文档包含 `http.trusted_proxies` 或其他未声明 trusted proxy 键
- **THEN** 严格配置解码 MUST 失败并报告完整配置路径
- **AND** 系统 MUST NOT 通过 normalize、alias 或 fallback 接受该旧配置
