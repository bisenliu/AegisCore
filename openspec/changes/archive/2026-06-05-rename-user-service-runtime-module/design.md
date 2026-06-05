## Context

`user-services/internal/bootstrap/app.go` 是用户服务进程的 Fx 组合根。当前 `NewApp` 通过 `UserServiceModule` 装配 timezone、validation、PostgreSQL、Redis、JWT、Ent、repository、service、controller、Gin engine、HTTP server 和路由注册，因此该符号表达的是 HTTP 服务运行时装配边界，而不是 `internal/service` 下的用户业务 service。

本变更属于 `http-service-runtime` capability 的内部命名一致性修正。它不改变 controller/service/repository 分层，不新增 common 共享基础能力，不修改 Ent schema、Atlas migration、HTTP API、响应契约或运行时配置。

## Goals / Non-Goals

**Goals:**

- 让用户服务 Fx 模块符号名称明确表达进程级运行时装配职责。
- 保持 `NewApp(configPath)` 创建 Fx app 的外部调用方式不变。
- 保持现有 Fx provider、invoke、路由注册、中间件顺序、HTTP server lifecycle、Redis/PostgreSQL/Ent runtime 依赖初始化行为等价。
- 使用测试或静态验证覆盖新符号命名和旧符号移除，避免概念层级混淆回归。

**Non-Goals:**

- 不重命名目录、Go module path、CLI 命令、服务标识 `aegiscore-user-services` 或配置键。
- 不调整 controller/service/repository 职责边界或移动业务逻辑。
- 不新增、删除或重排 HTTP 路由、中间件、认证边界或响应格式。
- 不修改数据库 schema、生成 Ent 代码或 Atlas migration。

## Decisions

- 将 `UserServiceModule` 重命名为表达应用组合根语义的符号 `AppModule`。选择应用组合根语义而非业务 service 语义，是因为该模块装配的是整个用户服务 HTTP runtime surface，而不是单个 `service.NewUserService` provider。
- 保留 Fx module 字符串名 `aegiscore-user-services`。该名称是服务身份和日志/配置语义的一部分，重命名它会扩大影响面，且当前问题只涉及 Go 内部符号职责表达。
- 保留 `NewApp` 对模块的单点引用并只更新直接引用。这样可以最小化变更，避免为命名修正引入新的 Fx 组合模式或拆分策略。
- 通过 Go 测试或仓库搜索验证旧符号不再存在，并运行用户服务模块测试确认 Fx 组装变更未破坏编译。相比新增运行时行为测试，命名回归更适合用静态断言覆盖。

## Risks / Trade-offs

- 旧符号若被测试或内部代码直接引用，重命名会导致编译失败。缓解方式是在同一变更内更新所有直接引用，并用 `go test ./...` 在 `user-services` 模块验证。
- 只重命名 Go 符号可能无法解决所有历史文档中的宽泛命名。缓解方式是只更新与本 capability 相关的 OpenSpec requirement 和必要测试，避免将一次命名修正扩大为文档全面整理。
- 不提供旧符号别名会让未来未同步的引用立即编译失败。该取舍有助于消除歧义，不保留与目标相反的兼容名称。

## Migration Plan

- 更新 `user-services/internal/bootstrap/app.go` 中 Fx 模块符号名称，并同步 `NewApp` 引用。
- 更新或新增测试，断言运行时模块命名使用进程级语义且旧 `UserServiceModule` 不再出现。
- 在 `user-services/` 中运行 `go test ./...` 验证编译和测试通过。
- 如需回滚，仅需恢复内部符号名称和对应测试；外部 API、配置和数据不涉及迁移。

## Open Questions

- 无。
