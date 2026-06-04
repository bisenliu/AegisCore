## Context

`user-services/internal/bootstrap/server.go` 当前在 Fx `OnStart` 中启动 goroutine 调用 `server.ListenAndServe()`，随后立即返回 `nil`。这种写法无法把端口占用、地址不可用或监听 socket 创建失败等错误同步反馈给 Fx；错误只会在后台 goroutine 中写入日志。

该问题位于 `http-service-runtime` capability 的 HTTP server lifecycle 边界，影响用户服务启动命令对外呈现的可用性。变更应限制在 bootstrap HTTP server 生命周期实现和相关测试，不改变 controller/service/repository 分层、路由、响应信封、Redis/PostgreSQL/Ent 依赖或配置加载策略。

## Goals / Non-Goals

**Goals:**

- 在 Fx `OnStart` 阶段同步创建监听器，使地址绑定或监听失败直接返回给 Fx app start。
- 成功绑定后继续异步处理 HTTP 请求，保持服务运行模型不变。
- 保持 `OnStop` graceful shutdown 逻辑、默认 shutdown timeout 和 `http.ErrServerClosed` 处理语义不变。
- 用测试覆盖监听失败会导致启动失败，避免回归为“启动成功但服务不可用”。

**Non-Goals:**

- 不新增健康检查聚合、依赖级 readiness probe 或外部监控接口。
- 不修改 HTTP API、错误码、响应信封、中间件顺序或认证路由分组。
- 不新增配置项，不修改 Redis、PostgreSQL、Ent 或 Atlas migration 行为。

## Decisions

- 使用 `net.Listen("tcp", addr)` 替代在 goroutine 中直接 `server.ListenAndServe()` 完成绑定。
  备选方案是在 goroutine 启动后通过 channel 回传早期错误并等待短暂窗口，但该方案需要人为选择等待时间，仍可能遗漏慢速失败。同步 `net.Listen` 能在 `OnStart` 内确定端口是否已绑定，失败路径更直接。

- 成功监听后使用 `server.Serve(listener)` 在 goroutine 中处理请求。
  备选方案是继续使用 `ListenAndServe` 并预先探测端口可用性，但预探测与实际绑定之间存在竞态。把已绑定的 listener 交给 `Serve` 可避免探测竞态。

- `OnStart` 返回监听创建错误，并在错误中保留 HTTP server 地址上下文。
  备选方案是只记录日志并返回通用错误，但 Fx/Cobra 层更需要明确的启动失败原因，便于部署诊断端口占用或绑定配置错误。

- 保持正常关闭时 `http.ErrServerClosed` 不记为失败。
  备选方案是统一记录所有 `Serve` 返回错误，但会把 graceful shutdown 记录为误报，不符合现有 `Shutdown gracefully` 规格。

## Risks / Trade-offs

- [Risk] `net.Listen` 成功后、goroutine 调用 `Serve` 前出现极短暂窗口。→ Mitigation: listener 已经完成端口绑定，Fx 启动可用性风险的核心失败路径已同步处理；后续 serving 异常仍按现有日志路径记录。
- [Risk] 测试如果使用固定端口可能不稳定。→ Mitigation: 测试应使用本地临时端口或先占用系统分配端口，再配置 HTTP server 绑定同一地址验证失败。
- [Risk] 改动 HTTP lifecycle 可能影响 graceful shutdown。→ Mitigation: 保留 `server.Shutdown` 和 `http.ErrServerClosed` 处理逻辑，并运行 `user-services` 相关测试验证。
