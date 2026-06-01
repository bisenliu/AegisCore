## Why

当前 `common/netutil` 只为 HTTP 日志中间件封装客户端 IP 解析，但 Gin 已提供 `Context.ClientIP()` 并支持受信代理配置。保留自定义解析会重复框架能力，并可能与 Gin 的代理解析策略产生不一致。

## What Changes

- **BREAKING**: 移除共享包 `common/netutil`，不再向仓库内部或潜在外部调用方提供 `netutil.GetClientIP`、`XForwardedFor`、`XRealIP`、`XClientIP`。
- 将现有客户端 IP 获取逻辑改为直接调用 Gin 的 `c.ClientIP()`。
- 删除 `common/netutil` 对应单元测试，因为 IP 解析行为由 Gin 负责。
- 保持 HTTP 日志字段 `client_ip` 不变，仅替换其取值来源。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `http-service-runtime`: HTTP 请求日志中的客户端 IP 来源改为 Gin 原生 `Context.ClientIP()`。
- `shared-infrastructure`: 移除共享基础设施中的 `common/netutil` IP 解析工具，减少重复实现。

## Impact

- 受影响代码：`common/middleware/logging.go`、`common/netutil/`。
- API 兼容性：HTTP 路由、响应信封、错误码和数据模型不变。
- 日志兼容性：日志字段名 `client_ip` 不变；字段值遵循 Gin 的 `ClientIP()` 解析规则。
- 依赖影响：不新增第三方依赖；减少仓库内共享工具包。
