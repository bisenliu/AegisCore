## ADDED Requirements

### Requirement: Trusted proxy 配置交付

Nacos、Compose、Kubernetes、Helm、README 和部署说明 MUST 使用 `server.http.trusted_proxies` 表达 user-service 受信任入口代理 IP 或 CIDR。生产和 production-like 环境 MUST 按实际 Ingress、gateway、ALB、Envoy、Nginx 或 service mesh 拓扑配置该列表；系统 MUST NOT 提供旧 `http.trusted_proxies` 示例或兼容说明。

#### Scenario: 部署配置声明可信入口

- **WHEN** user-service 部署在反向代理、Ingress、gateway、load balancer 或 service mesh 后方
- **THEN** 运行配置 MUST 在 `server.http.trusted_proxies` 中声明真实入口代理 IP 或 CIDR
- **AND** 配置示例 MUST 明确该列表需要按环境拓扑填写，MUST NOT 默认信任所有私有网段或所有来源

#### Scenario: 入口层清洗 forwarded headers

- **WHEN** 外部请求进入 Ingress、gateway、ALB、Envoy、Nginx 或 service mesh 边界
- **THEN** 入口层 MUST 覆盖或重建 `X-Forwarded-For` 和 `X-Real-IP` 等 forwarded headers
- **AND** 入口层 MUST NOT 将客户端提供的未清洗 forwarded headers 直接透传给 user-service

#### Scenario: 发布验证客户端地址

- **WHEN** 发布启用 trusted proxy 配置的 user-service
- **THEN** 发布验证 MUST 覆盖登录请求在受信任代理后方记录真实客户端地址
- **AND** 未配置或错误配置 trusted proxy 时，验证 MUST 能暴露 `client_ip` 仍为代理地址的 drift
