# AegisCore Common Module

`common` 是跨服务共享 Go module，模块路径为 `github.com/aegiscore/common`。它只承载稳定、业务中立、可被多个服务复用的契约和 runtime primitive，不承载 user-service 特定业务语义。

## Package Areas

| Area | Purpose |
|---|---|
| `contract/errors` | 稳定应用错误码和可渲染错误类型 |
| `contract/pagination` | Cursor/Keyset 分页契约 |
| `contract/response` | HTTP 响应信封 DTO |
| `http/binding` | Gin 请求绑定和校验失败响应适配 |
| `http/response` | Gin 响应输出 helper |
| `http/middleware` | 无业务语义 Gin middleware 骨架 |
| `http/openapi` | Swagger/OpenAPI 转换、规范化、序列化和 Go embed 渲染 helper |
| `http/pprof` | Go runtime pprof 路由注册 helper |
| `runtime/config` | YAML 与 `AEGISCORE_` 环境覆盖配置加载 |
| `runtime/logger` | Zap logger、context trace/span 字段 helper 和 Fx provider |
| `runtime/datastore` | Named Redis/PostgreSQL provider |
| `runtime/id` | 跨服务默认 UUID 生成策略 |
| `runtime/localcache` | 进程内短 TTL cache primitive |
| `runtime/rediskey` | 通用 Redis key 构造规则 |
| `runtime/workerpool` | 受控后台任务池 primitive |
| `runtime/scheduler` | 定时任务、锁、续租和 metrics adapter primitive |
| `runtime/observability` | Prometheus metrics 和 OpenTelemetry tracing provider |
| `security` | JWT、Bearer、密码和 Casbin generic helper |
| `testing` | 跨模块测试基础设施和无业务语义 fixture |
| `validation` | 通用结构校验核心 |

## Rules

- 不放 user-service 业务 DTO、权限目录、policy loader、route diff、auth/session/role/permission 语义。
- 不放 feature Redis key schema、缓存策略、session 策略或外部系统业务编排。
- `workerpool` 不是 MQ、eventbus、outbox 或可靠投递框架。
- `scheduler` 不是 feature orchestration、outbox 或具体 Prometheus registry wiring。
- `openapi` helper 不拥有服务 API server、auth scheme、source scan range 或输出目录。

## Development

From repository root:

```bash
make common-test
make common-lint
```

Or inside `common/`:

```bash
go test ./...
golangci-lint run ./...
```
