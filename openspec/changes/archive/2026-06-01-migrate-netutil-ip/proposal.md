## Why

当前项目的共享 HTTP 中间件已经记录 `client_ip`，但缺少一个统一、可测试的客户端真实 IP 提取工具。将既有 `go-micro-scaffold/common/pkg/netutil/ip.go` 迁移到 AegisCore 的 `common` 模块，可以避免各服务重复解析代理头，并为后续中间件和业务审计场景提供一致行为。

## What Changes

- 在 `common` 模块中新增共享网络工具包，用于从 Gin 请求上下文中提取客户端 IP。
- 迁移并优化 `GetClientIP` 行为：按可信顺序处理 `X-Forwarded-For`、`X-Real-IP`、`X-Client-IP`，忽略空白候选值，并最终回退到 Gin 的 `ClientIP()`。
- 为代理头解析与回退逻辑补充单元测试，覆盖多 IP、空白值和 fallback 场景。
- 不改变现有 HTTP API、响应信封、配置结构、数据库模型或运行时依赖装配。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `shared-infrastructure`: 扩展共享基础设施能力，增加可复用的 Gin 客户端 IP 提取工具。

## Impact

- 影响代码位置：`common/netutil/`，以及可能复用该工具的共享 HTTP 中间件测试或实现。
- 外部兼容性：无破坏性 API 变更；不新增 HTTP 路由、错误码、配置项或数据模型。
- 依赖影响：复用 `common` 模块已有的 `github.com/gin-gonic/gin` 依赖，不引入新的第三方依赖。
