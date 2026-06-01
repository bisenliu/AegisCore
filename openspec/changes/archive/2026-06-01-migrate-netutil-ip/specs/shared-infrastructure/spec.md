## ADDED Requirements

### Requirement: Provide reusable client IP extraction

系统 MUST 在 `common` 模块提供可复用的 Gin 客户端 IP 提取工具。该工具 MUST 按 `X-Forwarded-For`、`X-Real-IP`、`X-Client-IP`、Gin `ClientIP()` 的顺序解析客户端 IP，并且 MUST 忽略空白 header 值和逗号分隔列表中的空白候选值。

#### Scenario: Extract first forwarded IP
- **Given** HTTP 请求包含 `X-Forwarded-For: 203.0.113.10, 10.0.0.1`
- **When** 调用共享客户端 IP 提取工具
- **Then** 系统 MUST 返回 `203.0.113.10`

#### Scenario: Ignore blank forwarded candidates
- **Given** HTTP 请求包含 `X-Forwarded-For:  , 203.0.113.11`
- **When** 调用共享客户端 IP 提取工具
- **Then** 系统 MUST 返回 `203.0.113.11`

#### Scenario: Fall back to real IP header
- **Given** HTTP 请求未包含可用的 `X-Forwarded-For`
- **Given** HTTP 请求包含 `X-Real-IP: 203.0.113.12`
- **When** 调用共享客户端 IP 提取工具
- **Then** 系统 MUST 返回 `203.0.113.12`

#### Scenario: Fall back to client IP header
- **Given** HTTP 请求未包含可用的 `X-Forwarded-For` 或 `X-Real-IP`
- **Given** HTTP 请求包含 `X-Client-IP: 203.0.113.13`
- **When** 调用共享客户端 IP 提取工具
- **Then** 系统 MUST 返回 `203.0.113.13`

#### Scenario: Fall back to Gin client IP
- **Given** HTTP 请求未包含可用的代理 IP header
- **When** 调用共享客户端 IP 提取工具
- **Then** 系统 MUST 返回 Gin `ClientIP()` 的结果

#### Scenario: Request log uses shared client IP extraction
- **Given** HTTP 请求经过共享 request logging middleware
- **When** 请求包含可用的代理 IP header
- **Then** 请求日志中的 `client_ip` 字段 MUST 使用共享客户端 IP 提取工具的结果
