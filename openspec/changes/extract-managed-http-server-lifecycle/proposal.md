## Why

业务 HTTP server 与 pprof server 目前在 user-service bootstrap 中分别维护监听、异步 `Serve`、异常退出通知、优雅关闭、强制关闭和 handler drain 逻辑，行为不一致且难以证明并发停止与超时后的资源回收语义。需要把这套业务中立的 `net/http` 生命周期收敛到 `common/runtime/httpserver`，让服务 composition 只负责配置、Fx hook 和日志策略，并为后续服务复用提供稳定契约。

## What Changes

- 在 `common/runtime/httpserver` 新增不依赖 Gin、Fx、user-service 配置或业务日志字段的 `Managed` HTTP server，提供严格 options 校验、同步监听、异步 `Serve`、异常退出回调、状态机和可重复并发停止能力。
- 为已进入 handler 的请求提供内部 drain tracker；优雅关闭超时后强制关闭连接，并等待 handler 与 `Serve` goroutine 退出，最终错误通过 `errors.Join` 完整保留。
- 一次性把 user-service 业务 HTTP server 和独立 pprof server 迁移为两个独立的 `Managed` 实例；启用策略、地址来源、默认 timeout、结构化日志和 Fx 非零 shutdown signal 继续由服务 composition 拥有。
- **BREAKING** 删除 bootstrap 中旧的 listener、`Serve`、`Shutdown`、`Close`、drain helper 及其测试入口，不提供 type alias、deprecated wrapper、feature flag 或新旧双实现。
- 更新架构说明、能力地图和 OpenSpec 规格，使共享 runtime primitive 与服务侧观测生命周期的归属保持一致。

## Capabilities

### New Capabilities

无。

### Modified Capabilities

- `shared-platform-primitives`: 增加业务中立的 managed `net/http` server 生命周期、状态转换、异常分类、关闭与 handler drain 稳定契约。
- `runtime-observability`: 业务 HTTP 与 pprof listener 改为复用共享生命周期 primitive，并明确 Fx 启停、故障信号、独立实例和关闭预算行为。

## Impact

- 代码：新增 `common/runtime/httpserver/`；重写 `user-service/internal/bootstrap/server.go` 与 `user-service/internal/bootstrap/pprof.go`，并迁移对应 bootstrap 测试。
- 共享契约：新增 `httpserver.Options`、`httpserver.Managed`、构造与生命周期错误；核心包保持无 Gin、Fx 和服务私有依赖。
- HTTP API 与 OpenAPI：业务路由、响应和文档契约不变，不需要更新 OpenAPI 生成物。
- 数据库与安全：不修改 Ent schema、Atlas migration、认证、授权或 RBAC 行为。
- 部署与观测：不修改部署端口和观测资产；pprof 仍由既有配置决定是否启用及监听地址，异常退出继续记录服务日志并触发 exit code 1。
- 文档与验证：更新 `docs/ARCHITECTURE.md`、`docs/opsx/CAPABILITY_MAP.md` 和两个既有 capability 规格；增加 common race 测试与 user-service bootstrap lifecycle 测试。
