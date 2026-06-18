## Context

AegisCore 是 Go 1.26 workspace 后端项目底座，当前包含 `common/` 共享模块、`user-service/` 用户服务模块和 `deployments/` 部署观测资产。README 已声明 `AGENTS.md`、`docs/ARCHITECTURE.md`、`docs/DEVELOPMENT.md`、`docs/PRODUCT.md`、`docs/TESTING.md`、`docs/opsx/CAPABILITY_MAP.md`、`docs/opsx/CHANGE_WORKFLOW.md` 与 `openspec/specs/` 是协作入口，但这些基础文件尚未落地。

当前代码事实：

- `Makefile` 汇总 `build`、`test`、`lint`、`verify`、`user-service-*`、migration、OpenAPI 和 dashboard 命令。
- `common/` 承载契约响应、错误、分页、HTTP 中间件、OpenAPI helper、配置、Postgres/Redis、logger、metrics、scheduler、workerpool、安全和测试基础设施。
- `user-service/cmd/main.go` 提供 `serve` 和 `rbac` CLI，`rbac` 包含 seed、assign-super-admin、create-super-admin。
- `user-service/internal/router/router.go` 挂载健康检查、OpenAPI、metrics、pprof、认证、权限、角色和用户 API。
- `user-service/internal/features/` 使用 feature 分层组织 auth、permission、role、user，并区分 domain、application、infrastructure、transport。
- `deployments/` 提供 Docker、Compose、Kubernetes、Helm、Prometheus 和 Grafana 资产。

约束：

- OPSX/OpenSpec 文档、change artifacts 和主规格正文必须使用简体中文；技术标识符、路径、命令、Go symbol、OpenSpec 关键字可保留英文。
- 本 change 只创建文档、配置和规格基础设施，不写业务代码、不改运行时行为。
- 后续实现必须尊重现有 Go workspace、feature 分层和 Makefile 验证入口。

## Goals / Non-Goals

**Goals:**

- 创建完整 OPSX 基础目录和文档入口，使代理读完后能理解项目目标、架构、开发方式、测试方式和变更流程。
- 建立仓库级 OpenSpec 配置，固化中文输出、技术栈、分层、规格、设计和任务规则。
- 创建能力地图，将 capability、主要代码位置和主规格路径串起来。
- 创建 7 个主规格基线：`opsx-foundation`、`shared-platform-primitives`、`user-identity-management`、`auth-session-management`、`rbac-access-control`、`runtime-observability`、`delivery-operations`。
- 提供可直接用于项目启动的基础模板内容，包括 change 模板、spec 模板、文档维护规则和初始化说明。

**Non-Goals:**

- 不新增、删除或修改任何 Go 业务代码、测试逻辑、数据库 migration、OpenAPI 生成代码或部署清单行为。
- 不引入新的外部依赖、CI 系统或代码生成工具。
- 不替代现有 README，而是补齐 README 已指向的详细入口。
- 不在本 change 中完成业务能力重构；后续业务变更仍通过独立 `/opsx:propose` 与 `/opsx:apply` 执行。

## Decisions

### Decision: 以现有 README 声明的路径作为 OPSX 基础目录

采用以下最终目录结构：

```text
.
├── AGENTS.md
├── docs/
│   ├── ARCHITECTURE.md
│   ├── DEVELOPMENT.md
│   ├── PRODUCT.md
│   ├── TESTING.md
│   └── opsx/
│       ├── CAPABILITY_MAP.md
│       └── CHANGE_WORKFLOW.md
└── openspec/
    ├── config.yaml
    ├── specs/
    │   ├── auth-session-management/spec.md
    │   ├── delivery-operations/spec.md
    │   ├── opsx-foundation/spec.md
    │   ├── rbac-access-control/spec.md
    │   ├── runtime-observability/spec.md
    │   ├── shared-platform-primitives/spec.md
    │   └── user-identity-management/spec.md
    └── changes/
        └── <change-name>/
            ├── proposal.md
            ├── design.md
            ├── specs/<capability>/spec.md
            └── tasks.md
```

理由：README 已将这些路径作为入口，直接补齐能减少迁移成本。备选方案是创建新的 `opsx/` 根目录，但会与 README 和 OpenSpec CLI 默认结构脱节。

### Decision: `AGENTS.md` 作为代理导航图，详细内容下沉到 `docs/`

`AGENTS.md` 控制在约 80-140 行，提供仓库地图、常用命令、变更入口、语言规则、分层提醒和高风险区域提示。架构、产品、测试和工作流细节写入 `docs/`。

理由：代理入口需要短而可扫读，避免把所有细节堆到单文件。备选方案是只写 README，但 README 面向人类概览，不适合承载所有代理操作规则。

### Decision: 主规格以稳定能力而不是目录名命名

新增主规格使用业务语义命名，例如 `auth-session-management` 和 `rbac-access-control`，而不是 `auth-feature` 或 `role-package`。

理由：OPSX change 需要围绕长期能力演进，目录名可能随实现调整而变化。备选方案是按代码目录一比一建 spec，但会把实现结构误认为产品能力。

### Decision: `openspec/config.yaml` 固化仓库级中文和工程约束

核心配置内容应覆盖：

```yaml
schema: spec-driven
context: |
  项目：AegisCore
  输出语言：简体中文
  技术栈：Go 1.26 workspace，Gin，Fx，Ent，Atlas，PostgreSQL，Redis，Casbin，Prometheus，OpenTelemetry，Docker，Kubernetes，Helm
  代码结构：common 为跨服务共享模块，user-service 为用户服务模块，deployments 为部署和观测资产
rules:
  proposal:
    - 说明 why、what、capabilities、impact
    - capability 使用 kebab-case，并优先对应稳定业务能力
  specs:
    - 使用 Given/When/Then 或 WHEN/THEN 场景
    - 每个 Requirement 至少包含一个主流程和一个异常或边界场景
    - 正文使用简体中文，技术标识符保留原文
  design:
    - 说明受影响路径、关键决策、备选方案、风险和验证方式
    - 明确是否影响 API、数据库 schema、部署、观测或安全边界
  tasks:
    - 任务按 1-2 小时粒度拆分
    - 每组任务包含文件产物和验证命令
```

理由：OpenSpec instruction 会把 config context 和 rules 注入后续 artifacts，仓库规则越具体，后续变更越稳定。备选方案是保留默认 config，但它缺少本仓库的语言、技术栈和分层约束。

### Decision: 能力地图必须连接 capability、代码位置、主规格和状态

`docs/opsx/CAPABILITY_MAP.md` 使用表格列出 capability、业务说明、主要代码位置、主规格路径和状态，并补充入口点、交叉依赖和待补能力。

理由：后续代理需要先定位能力，再决定是否修改主规格或创建 change delta。备选方案是只列目录树，但无法回答“某个变更属于哪个规格”。

### Decision: 验证分为文档结构、OpenSpec 状态和仓库既有验证

基础验证包括：

- 文件存在性：`test -f AGENTS.md`、`test -f docs/opsx/CAPABILITY_MAP.md`、`test -f openspec/config.yaml`。
- 规格格式：`find openspec/specs -name spec.md`，并检查 `Requirement` 与 `Scenario`。
- OpenSpec：`openspec list --specs`、`openspec validate --specs`，以及当前 change 的 `openspec status --change create-opsx-foundation`。
- 仓库验证：`make user-service-architecture-lint`；若耗时允许，再运行 `make verify`。

理由：这次变更不改业务代码，但文档语言和规格格式仍需要自动化约束。备选方案是只人工检查 Markdown，会放过模板残留和格式漂移。

## Risks / Trade-offs

- [Risk] 主规格一次覆盖 7 个能力，内容可能偏基线而不够深入 → Mitigation：每个 spec 先覆盖稳定约束、主流程、异常流程和边界条件，后续业务 change 再通过 delta 细化。
- [Risk] 文档描述与代码事实漂移 → Mitigation：能力地图引用真实路径，并把维护规则写入 `AGENTS.md` 与 `CHANGE_WORKFLOW.md`。
- [Risk] OpenSpec 默认模板英文混入中文文档 → Mitigation：配置中固化中文规则，并通过 architecture-lint 检查 OpenSpec/OPSX Markdown。
- [Risk] `openspec validate --specs` 对主规格格式要求与当前 CLI 版本存在差异 → Mitigation：先运行本地 CLI 校验，必要时调整 header 和场景格式，保留 `#### Scenario`。
- [Risk] 后续实现者把本 change 扩展到业务代码 → Mitigation：tasks 中明确非目标和完成边界，只创建文档、配置、主规格和模板。

## Migration Plan

1. 创建 `AGENTS.md`、`docs/`、`docs/opsx/`、`openspec/specs/` 目录和文件。
2. 用仓库事实填充文档内容，确保没有默认英文模板段落残留。
3. 更新 `openspec/config.yaml` 为仓库级规则。
4. 创建 7 个主规格基线并保证 `Requirement` 与 `#### Scenario` 格式正确。
5. 运行文件存在性检查、OpenSpec 状态检查、OpenSpec 校验和 `make user-service-architecture-lint`。
6. 如需回滚，删除本 change 创建的文档和主规格文件，并恢复 `openspec/config.yaml` 到变更前内容；因为不涉及运行时文件，无需数据迁移或服务回滚。

## Open Questions

- 是否在后续独立 change 中把 `make verify` 纳入 CI 或 pre-commit 入口，需要结合仓库实际 CI 决策。
- 是否需要为 `common/` 中更细的能力继续拆分主规格，例如 `contract-response-envelope`、`runtime-scheduler`、`validation-binding`，可在本次基础框架落地后按变更频率追加。
