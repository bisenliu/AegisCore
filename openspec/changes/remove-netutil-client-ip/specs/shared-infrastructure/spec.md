## REMOVED Requirements

### Requirement: Provide reusable client IP extraction

**Reason**: Gin 已提供 `Context.ClientIP()` 作为 HTTP runtime 的客户端 IP 解析入口，继续在 `common` 中维护自定义 header 优先级会重复框架能力，并可能与 Gin 的代理配置策略不一致。

**Migration**: 删除 `common/netutil` 后，仓库内 request logging middleware 直接使用 `c.ClientIP()`；其他调用方应改为在 Gin handler 或 middleware 中使用 `gin.Context.ClientIP()`。
