## Why

当前 user-service 已通过多个 `fx.Module` 组装 runtime provider、HTTP server、router 和各 feature，但缺少可审查的依赖图产物，协作者很难在修改 provider 或 module 边界前快速判断依赖方向、缺失 provider、循环依赖或不合理共享位置。

需要为 user-service 增加 Fx 依赖图生成入口，并把与具体服务无关的 Fx 图构建、DOT 渲染和文件输出方法沉淀到 `common/`，避免后续服务重复实现同类工具。

## What Changes

- 新增跨服务共享的 Fx 依赖图生成 helper，用于从 Fx app/module 构建依赖图并输出稳定、可审查的图文件。
- 新增 user-service 专用生成入口或脚本，复用 `common/` helper 生成 user-service 当前 Fx 依赖图。
- 新增 Makefile 目标，支持协作者通过服务前缀命令生成或检查 user-service Fx 依赖图。
- 将生成物纳入交付校验或明确 drift 检查方式，使 provider/module 变化后依赖图可以同步更新。
- 不改变 user-service HTTP API、数据库 schema、认证/RBAC 行为或运行时服务启动语义。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `shared-platform-primitives`: 扩展 `common/` 中 runtime primitive 的范围，新增业务中立的 Fx 依赖图构建与渲染 helper。
- `delivery-operations`: 扩展 user-service 交付命令，新增 Fx 依赖图生成或 drift 检查入口。

## Impact

- 影响 `common/`：新增用于 Fx 依赖图构建、DOT 渲染或文件输出的业务中立包或 helper，不引入 user-service feature 语义。
- 影响 `user-service/`：新增服务级依赖图生成入口，引用现有 `bootstrap.AppModule` 或等价服务 module 组装图。
- 影响 `Makefile` 与 `user-service/Makefile`：新增带 `user-service-` 前缀的根目标和服务内目标。
- 可能新增生成物，例如 `user-service/docs/fx-dependency-graph.dot` 或等价位置；生成物必须稳定排序，便于 code review 检查 diff。
- 不影响外部 HTTP API、OpenAPI、Ent schema、Atlas migration、部署清单、安全契约或 RBAC policy。
