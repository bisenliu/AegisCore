# AegisCore Common Module

`common` 是跨服务共享 Go module，模块路径为 `github.com/aegiscore/common`。它只承载稳定、业务中立、可被多个服务复用的契约和 runtime primitive，不承载 user-service 特定业务语义。

## 包区域

| 区域 | 作用 |
|---|---|
| `contract/errors` | 稳定应用错误码和可渲染错误类型 |
| `contract/pagination` | Cursor/Keyset 分页契约 |
| `contract/response` | HTTP 响应信封 DTO |
| `http/binding` | Gin 请求绑定和校验失败响应适配 |
| `http/response` | Gin 响应输出 helper |
| `http/middleware` | 无业务语义 Gin middleware 骨架 |
| `http/openapi` | Swagger/OpenAPI 转换、规范化、序列化和 Go embed 渲染 helper |
| `http/pprof` | Go runtime pprof 路由注册 helper |
| `runtime/config` | 仅含 app/runtime/server/log/observability 的核心配置、严格 YAML 与显式配置文件加载 |
| `runtime/logger` | stdout/stderr Zap logger、context trace/span 字段 helper 和 Fx provider |
| `runtime/datastore` | Named Redis/PostgreSQL provider |
| `runtime/id` | 跨服务默认 UUID 生成策略 |
| `runtime/localcache` | 有容量上限的进程内 TTL loading cache primitive |
| `runtime/rediskey` | 通用 Redis key 构造规则 |
| `runtime/resources` | 无业务语义的具名 Redis/PostgreSQL 配置、默认值、校验和稳定资源名 |
| `runtime/workerpool` | 受控后台任务池 primitive |
| `runtime/scheduler` | 定时任务、锁、续租和 metrics adapter primitive |
| `runtime/observability` | Prometheus metrics 和 OpenTelemetry tracing provider |
| `runtime/timezone` | 基于已校验配置时区的时间位置 helper |
| `security/auth` | JWT、Bearer、认证上下文和 token version helper |
| `security/casbin` | 通用请求三元组和 Casbin authorizer wrapper |
| `security/password` | 密码哈希和校验 helper |
| `testing` | 跨模块测试基础设施和无业务语义 fixture |
| `validation` | 通用结构校验核心 |

## 规则

- 不放 user-service 业务 DTO、权限目录、policy loader、route diff、auth/session/role/permission 语义。
- 不放 feature Redis key schema、缓存策略、session 策略或外部系统业务编排。
- `workerpool` 不是 MQ、eventbus、outbox 或可靠投递框架。
- `scheduler` 不是 feature orchestration、outbox 或具体 Prometheus registry wiring。
- `openapi` helper 不拥有服务 API server、auth scheme、source scan range 或输出目录。

`runtime/scheduler` 使用固定 job key 提供 `Add`、`Remove`、`Start` 和 `Stop`。nil `LockPolicy` 表示不加分布式锁，正数 `WaitTimeout` 表示在总上限内重试等待，nil `RenewPolicy` 表示不续租；全局并发、Redis retry、owner token 锁和续租能力均保留。内部执行顺序固定为 triggered、本地 overlap、全局并发、锁、任务 context、续租、started/result 和 task，各 stage 只释放自身资源。

`AllowOverlap=true` 与全局或锁 wait 组合会让高频触发形成等待 goroutine，scheduler 不提供持久队列或 pending 上限。Redis 锁是需要任务协作响应 context 的 lease，不提供 exactly-once、fencing 或 goroutine 强杀保证；长任务必须配置足够 TTL 和续租，并保证副作用幂等。

## 开发

从仓库根目录执行：

```bash
make common-test
make common-lint
```

在 `common/` 目录内执行：

```bash
go test ./...
golangci-lint run ./...
```
