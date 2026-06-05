## Context

`user-services/internal/bootstrap/ent.go` 在用户服务 bootstrap 边界基于具名 `user_db` 与 `common_db` PostgreSQL 连接池创建两个 Ent clients，并在 Fx `OnStop` lifecycle 中关闭它们。当前停止逻辑会先关闭 `user_db` Ent client，再关闭 `common_db` Ent client；如果 `user_db` close 返回错误，函数会直接返回，导致 `common_db` close 错误不可见。

该变更属于 `shared-infrastructure` 能力中服务侧 Ent runtime dependency wiring 的停止错误处理细化，不涉及 controller/service/repository 分层、HTTP 响应契约、配置加载、Ent schema 或数据库 migration。

## Goals / Non-Goals

**Goals:**

- 确保用户服务停止时两个 Ent clients 都会被尝试关闭。
- 当任一 Ent client close 失败时，返回的 Fx 停止错误保留失败 client 的具名上下文。
- 当两个 Ent clients close 都失败时，返回错误同时保留两者的底层错误。
- 保持 `user_db` 与 `common_db` 的 Fx named injection、PostgreSQL 配置路径和 repository 注入行为不变。

**Non-Goals:**

- 不新增或修改 HTTP API、错误码、配置项、数据模型或 migration。
- 不移动 Ent client provider 的 bootstrap 边界。
- 不修改 Ent schema 或手写 `user-services/ent/` 生成代码。
- 不引入新的共享基础设施抽象或外部依赖。

## Decisions

- 使用 Go 标准库错误聚合方式合并 close 错误，并用具名错误包装标识 `user_db` 与 `common_db`。
  - 原因：`errors.Join` 能在单个返回错误中保留多个底层错误，避免第一个错误覆盖第二个错误；`fmt.Errorf("close user_db ent client: %w", err)` 这类包装能保留具名上下文。
  - 备选：只记录第二个错误到日志并返回第一个错误。该方案能提高可观察性，但 Fx 停止调用方仍无法从返回错误中获取完整失败集合。
  - 备选：自定义聚合错误类型。该方案增加不必要的类型和维护成本，当前需求可由标准库覆盖。
- 保持两个 close 调用顺序和 Ent client provider 结构不变。
  - 原因：需求只涉及停止错误保留，不需要改变资源生命周期或依赖图。
  - 备选：抽取通用 named close helper。当前只有两个 Ent clients，抽象会增加命名和测试负担，收益有限。
- 在用户服务 bootstrap 包内验证停止错误聚合行为。
  - 原因：该行为是服务侧 Ent client provider 的 lifecycle 细节，应靠近 `user-services/internal/bootstrap` 测试，避免扩大 common 边界。

## Risks / Trade-offs

- [Risk] `errors.Join` 返回的字符串格式由标准库决定，测试若断言完整字符串可能脆弱。→ Mitigation：测试优先断言错误非空、包含具名上下文，必要时断言 `errors.Is` 能识别底层错误。
- [Risk] Ent client close 通常依赖真实 driver，直接制造双 close 错误可能不便。→ Mitigation：实现时优先抽出最小的 close 错误合并函数或使用可控 closer seam 测试错误聚合，不改变生产依赖图。
- [Risk] 过度抽象会把服务特定 Ent wiring 推入 common。→ Mitigation：仅在 `user-services/internal/bootstrap` 内处理该 provider 的停止错误，保持共享基础设施边界不变。
