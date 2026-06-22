## Context

`common/runtime/localcache` 属于 `shared-platform-primitives` 的 runtime primitive。当前 `cache.go` 同时包含错误变量、公开类型、`Cache` 结构和全部方法实现，代码行为集中但文件职责不够清晰。

本次变更只调整同一个 Go package 内的文件布局。调用方仍通过 `github.com/aegiscore/common/runtime/localcache` 使用相同的导出符号；Go 编译单元不因文件拆分改变包级可见性、泛型类型签名或初始化行为。

## Goals / Non-Goals

**Goals:**

- 将错误变量集中到 `errors.go`，便于识别包级错误契约。
- 将公开类型集中到 `types.go`，便于识别 `localcache` 对外 API。
- 让 `cache.go` 聚焦 `Cache` 核心实现、构造函数和方法。
- 保持导出 API、错误变量、Ristretto 配置、TTL、singleflight 合并、stats 计数和 `Close` 行为完全不变。
- 通过 `common/runtime/localcache` 包测试确认拆分未改变行为。

**Non-Goals:**

- 不修改 `localcache` 的导出类型、函数、方法或错误变量名称。
- 不调整缓存容量、TTL、`LoadTimeout`、`singleflight`、clone、防击穿、stats、eviction 或 close 语义。
- 不修改调用方代码、HTTP API、数据库 migration、OpenAPI 生成物、部署清单、观测资产或安全边界。
- 不把 user-service 业务语义、feature key schema 或缓存策略迁移到 `common/runtime/localcache`。

## Decisions

1. 在同一 package 内按职责拆分文件。

   - 决策：新增 `errors.go` 和 `types.go`，保留 `cache.go` 作为核心实现文件，三者均使用 `package localcache`。
   - 理由：Go 同包文件共享包级作用域，能够在不改变任何导出符号路径和调用方式的前提下改善可读性。
   - 备选方案：新增子包如 `localcache/errors` 或 `localcache/types`。该方案会改变导入路径并破坏 API，不采用。

2. 只移动声明，不重写逻辑。

   - 决策：错误变量、公开类型和实现代码按原声明内容迁移，除了必要的 import 调整和 gofmt 外不改变表达式。
   - 理由：本次目标是结构重组，行为风险应控制在最小范围。
   - 备选方案：顺手整理构造函数、stats 或 close 逻辑。该方案扩大行为面，不符合范围。

3. 保持验证聚焦在 localcache 包。

   - 决策：优先运行 `go test ./runtime/localcache`（在 `common/` 模块内）验证。
   - 理由：变更只影响该包源码布局，包级测试可以覆盖编译、导出符号和现有行为。
   - 备选方案：直接运行 `make verify`。该方案可作为合并前补充，但对本次窄范围变更成本较高。

## Risks / Trade-offs

- [Risk] 移动声明时遗漏 import 或注释，导致 lint、doc 或编译异常。→ Mitigation：拆分后运行 `gofmt` 和 `go test ./runtime/localcache`。
- [Risk] 误改错误变量或类型定义，造成 API 或错误匹配变化。→ Mitigation：只移动原声明内容，不改变量名、类型名、字段名、签名和错误字符串。
- [Risk] `cache.go` import 列表未收敛，残留未使用依赖。→ Mitigation：使用 `gofmt`/Go 编译反馈清理 import。

## Migration Plan

实现时在单次代码变更中完成文件拆分，部署层无需迁移。若验证失败，回滚方式是将移动出的声明恢复到 `cache.go` 并删除新增文件；由于未改变 API 或持久化数据，不需要数据库、配置、OpenAPI 或部署回滚。

## Open Questions

- 无。
