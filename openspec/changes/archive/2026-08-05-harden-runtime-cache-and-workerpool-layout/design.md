## Context

`common/runtime/workerpool/pool.go` 当前同时包含公开类型、构造、提交、任务执行、统计、停止状态机和大段使用说明；`common/runtime/localcache/cache.go` 同时包含构造、读取、singleflight、revision 失效、统计和使用说明。两个 package 的导出面已经由 `shared-platform-primitives` 主规格约束，不能通过新增子包或新公开入口规避现有边界。

localcache 使用 `singleflight.Group.DoChan` 的 `any` 结果通道。泛型 `V` 为接口类型且 loader 返回 nil 时，返回值进入 `any` 后没有动态类型，当前 `loaded.Val.(V)` 会 panic。localcache 同时使用 `ttlcache.OnEviction` 区分容量驱逐；该依赖为每个 eviction callback 启动 goroutine，显式 `InvalidateAll` 也会为每个 item 触发无需计数的回调。

## Goals / Non-Goals

**Goals:**

- 保持 `workerpool` 与 `localcache` 的 package、导出类型、函数、方法、错误和调用方式不变。
- 按单一职责拆分生产文件，让准入、执行、停止、加载、失效和统计边界可独立审查。
- 保证 `LoadingCache[V]` 在 `V` 为接口且成功值为 nil 时不 panic，并继续按正常成功加载缓存该值。
- 让容量驱逐计数在导致驱逐的公开 `Get` 返回前同步可见，同时不统计 TTL、`Invalidate` 或 `InvalidateAll`。
- 保持 fixed TTL、singleflight、caller cancellation、cache-wide revision、一次透明重试和 workerpool drain 语义不变。

**Non-Goals:**

- 不新增 queue capacity、非阻塞提交、可取消 admission、`Set`、`Close`、clone、per-key revision 或后台清理能力。
- 不更换 ants、ttlcache 或 singleflight 依赖，不重写 workerpool 并发模型。
- 不迁移 auth、permission 或其他 feature 业务语义到 common。
- 不修改 HTTP API、数据库、OpenAPI、部署或观测资产。

## Decisions

### Decision: 只做同 package 文件拆分

workerpool 按 types、pool/admission、execution、lifecycle、stats 和 package docs 拆分；localcache 按 types、cache、loading、invalidation、stats 和 package docs 拆分。所有文件继续使用原 package，公开符号路径和包级可见性不变。

备选方案是创建 `workerpool/lifecycle`、`localcache/loading` 等子包。该方案会拆散同一个并发状态机、改变导入路径或迫使暴露内部接口，因此不采用。

### Decision: 使用泛型 flight result 保存 value

singleflight 的成功结果统一包装为不可导出的 `flightResult[V]`，调用方从 wrapper 读取字段，不再直接把可能为 nil 的接口 value 放入 `any` 后执行 `.(V)`。loader error 与失效 sentinel 仍通过 error 通道传播，既有错误匹配不变。

备选方案是对 `loaded.Val == nil` 特判并返回 `zero V`。该方案无法区分合法 nil 和实现错误，且把泛型语义分散在结果消费侧，不采用。

### Decision: 在发布锁内同步计数容量驱逐

成功 loader 在 revision 校验通过后继续持有 `publishMu`，先执行 `DeleteExpired`，再根据当前未过期 item 数和配置容量判断本次新 key 写入是否必然触发容量驱逐，随后同步增加计数并调用 `Set`。所有写入只来自这一受锁路径，因此判定与发布之间不存在其他写入竞态。

备选方案是保留 `OnEviction` 并过滤 reason。该方案仍会为 TTL 和显式删除启动异步 callback goroutine，且 `Stats` 只能最终一致，不采用。直接导出 `ttlcache.Metrics` 无法区分容量、TTL 和显式删除，也违反现有规格。

### Decision: package 文档承载完整使用契约

构造函数注释保留简洁的 API 摘要，配置、取消、背压、失效、统计和值所有权等完整说明迁入 `doc.go`。可执行用法使用 `example_test.go` 表达，避免不可编译的伪代码淹没核心实现。

## Risks / Trade-offs

- [Risk] 文件移动遗漏 import、注释或 helper，导致编译或文档回退。-> Mitigation：保持同 package，使用 `gofmt`、包测试和 `go vet` 验证。
- [Risk] 同步容量判定与 ttlcache 实际 eviction 漂移。-> Mitigation：所有 Set 在 `publishMu` 下串行，写入前先清理过期项，并新增容量边界与非容量删除测试。
- [Risk] package 文档迁移时丢失重要取消或生命周期约束。-> Mitigation：按原说明逐项迁移，并保留构造器上的简洁指向。
- [Risk] 当前未归档的 scheduler change 也包含同 capability delta。-> Mitigation：本 change 使用独立 ADDED requirement，实施和验证不修改该 change，也不代为归档。

## Migration Plan

1. 先提交 localcache 泛型结果和同步容量计数测试，再完成实现及文件拆分。
2. 拆分 workerpool 文件并保持现有测试全部通过，修正文档中的 ants 预分配描述。
3. 运行两个包及 metrics adapter 的普通测试、race 测试和 vet，再运行 OpenSpec、架构、lint 与 verify 门禁。
4. 本变更无调用方迁移和部署顺序要求。若验证失败，回滚新增文件并恢复原同包声明；不涉及持久化数据、配置或外部契约回滚。
