## Context

user-service 当前通过 `bootstrap.AppModule` 聚合 `common` runtime module、validation、auth、permission、role、user feature module 和 `providers.Module`，各 module 内继续使用 `fx.Provide` 与 `fx.Invoke` 注册 provider 和生命周期依赖。现有代码可以通过 Fx 在运行时发现依赖缺失，但缺少可提交、可 review 的静态依赖图产物。

本变更横跨 `common` 与 `user-service`。`common` 只承载与服务无关的 Fx 图构建、DOT 渲染、稳定排序和文件写入能力；user-service 负责选择本服务 module、定义生成命令、确定生成物位置和接入 Makefile。该能力不进入 `user-service/internal/shared`，也不进入任一 feature 包。

## Goals / Non-Goals

**Goals:**

- 为 user-service 生成稳定的 Fx 依赖图文件，便于 review provider/module 变更。
- 在 `common/` 提供业务中立的 Fx 图构建与渲染方法，后续服务可以复用。
- 提供带服务前缀的 Makefile 目标生成依赖图，并提供 drift 检查路径。
- 用测试覆盖图输出的稳定排序、文件输出和 user-service 生成入口。

**Non-Goals:**

- 不改变 user-service 启动时的 provider、invoke 或生命周期执行顺序。
- 不新增 HTTP API、OpenAPI 注解、Ent schema、Atlas migration、部署清单或 RBAC policy。
- 不把 user-service feature 语义、业务 DTO、route diff、权限策略或配置专用逻辑放入 `common`。
- 不引入 Graphviz 二进制作为必须运行时依赖；第一阶段输出 DOT 文本即可。

## Decisions

1. 公共能力放入 `common/runtime/fxgraph` 或等价 runtime primitive 包。

   该包依赖 `go.uber.org/fx` 和 Fx 可用的图导出能力，负责接收 `fx.Option` 或等价输入，生成 DOT 文本并写入文件。选择 `common/runtime` 是因为它属于跨服务启动和依赖组装 primitive；不选择 `common/http`、`common/security` 或 `user-service/internal/shared`，因为依赖图与 HTTP、安全和服务内业务内核无关。

2. user-service 使用薄生成入口组装 `bootstrap.AppModule`。

   生成入口应位于 `user-service` 自有边界，例如 `user-service/cmd/fxgraph`、`user-service/scripts/fxgraph-generate.sh` 或专用 `go run` 入口。入口只负责提供 `config.ConfigPath`、公共配置和 logger provider 以及 `bootstrap.AppModule`，再调用 `common` helper 输出文件。

3. 生成物采用稳定 DOT 文件。

   DOT 文本适合 code review，可在需要时由外部 Graphviz 渲染图片。helper 必须保证节点、边或原始 DOT 的输出稳定，避免相同代码多次生成产生无意义 diff。若 Fx 原生输出已稳定，仍需用测试或检查约束生成命令的幂等性。

4. Makefile 目标使用服务前缀。

   根 `Makefile` 新增 `user-service-fxgraph-generate` 或等价目标，委托 `user-service/Makefile` 中的服务内目标。不得新增没有服务上下文的根目标，例如 `fxgraph-generate`。

5. drift 检查优先复用生成后 `git diff --exit-code`。

   可以新增专用 check 模式，也可以先将生成目标纳入 `verify` 或 tasks 验证说明。若纳入 `verify`，必须确保本地无额外外部依赖且生成速度可接受。

## Risks / Trade-offs

- [Risk] Fx 图生成可能触发 provider 构造或读取真实配置，导致需要数据库、Redis 或外部环境。→ 生成入口必须使用 Fx 图验证/可视化路径，避免启动 lifecycle 或连接真实依赖；必要时使用只构图的 `fx.New` 选项并在测试中覆盖无外部依赖执行。
- [Risk] DOT 输出排序不稳定造成噪声 diff。→ helper 或生成脚本必须稳定化输出，并以重复生成无 diff 作为验证项。
- [Risk] 公共 helper 过度抽象并泄漏 user-service 语义。→ `common` API 只接受通用 Fx option、输出路径和渲染选项，不包含 user、auth、RBAC、HTTP route 或配置名称语义。
- [Risk] 把图生成入口加入 `verify` 可能增加开发成本。→ 实施时评估耗时；如成本较高，先提供显式生成目标和文档化 drift 检查，后续再纳入完整 verify。

## Migration Plan

1. 新增 `common` helper 和单元测试，不影响现有服务运行。
2. 新增 user-service 生成入口和 DOT 生成物，运行一次生成并提交结果。
3. 新增 Makefile 目标，并执行相关测试和 `make user-service-architecture-lint`。
4. 回滚时删除新增 helper、入口、Makefile 目标和 DOT 生成物；不会涉及数据库、部署或外部 API 回滚。

## Open Questions

- 生成物最终路径在实现时确定，优先选择 `user-service/docs/fx-dependency-graph.dot` 或 `user-service/docs/fx/` 下的稳定位置。
- 是否把生成目标纳入 `make verify` 由实现时的耗时和外部依赖情况决定；若不纳入，必须提供明确的 check 命令或 tasks 验证说明。
