## 1. 初始化 OPSX 与 OpenSpec 结构

- [x] 1.1 确认 `openspec/config.yaml` 存在；如不存在，执行 `openspec init --tools none --force` 创建 OpenSpec 根目录。
- [x] 1.2 创建 `docs/opsx/` 和 `openspec/specs/` 目录，保持 README 中声明的入口路径可访问。
- [x] 1.3 确认本 change 不修改 Go 业务代码、数据库 migration、OpenAPI 生成物或部署清单运行时行为。

## 2. 配置仓库级 OpenSpec 规则

- [x] 2.1 更新 `openspec/config.yaml`，声明 `schema: spec-driven`、项目上下文、输出语言、Go workspace 技术栈和核心目录职责。
- [x] 2.2 在 `openspec/config.yaml` 中补充 proposal、specs、design、tasks 规则，要求中文正文、kebab-case capability、场景格式、影响分析和验证步骤。
- [x] 2.3 检查配置中没有保留默认英文模板说明，技术标识符、路径、命令和 Go symbol 保留原文即可。

## 3. 创建代理与项目基础文档

- [x] 3.1 创建 `AGENTS.md`，包含仓库导航、核心文档地图、常用命令、OPSX 入口、语言规则、架构边界和高风险区域提示。
- [x] 3.2 创建 `docs/PRODUCT.md`，说明 AegisCore 的项目目标、用户与角色、核心场景、关键约束和当前能力范围。
- [x] 3.3 创建 `docs/ARCHITECTURE.md`，说明 `common/`、`user-service/`、`deployments/` 的职责，列出 HTTP、CLI、RBAC、认证、观测和数据迁移关键流程。
- [x] 3.4 创建 `docs/DEVELOPMENT.md`，整理构建、运行、测试、lint、生成 Ent、生成 OpenAPI、migration 和本地部署命令。
- [x] 3.5 创建 `docs/TESTING.md`，说明 Go 单元测试、集成测试、e2e、Testcontainers、架构 lint、OpenAPI drift 和 dashboard drift 的验证入口。

## 4. 创建 OPSX 工作流文档

- [x] 4.1 创建 `docs/opsx/CAPABILITY_MAP.md`，用表格连接 capability、业务说明、主要代码位置、主规格路径和状态。
- [x] 4.2 在能力地图中列出 7 个基线能力：`opsx-foundation`、`shared-platform-primitives`、`user-identity-management`、`auth-session-management`、`rbac-access-control`、`runtime-observability`、`delivery-operations`。
- [x] 4.3 在能力地图中补充关键入口点、交叉依赖、待补能力和后续拆分建议。
- [x] 4.4 创建 `docs/opsx/CHANGE_WORKFLOW.md`，结合本仓库说明 `/opsx:explore`、`/opsx:propose`、`/opsx:apply`、`/opsx:verify`、`/opsx:archive` 的使用时机和产物要求。
- [x] 4.5 在工作流文档中补充新 change 命名规则、artifact 中文规则、实现前检查、归档后主规格维护规则。

## 5. 创建 OpenSpec 主规格基线

- [x] 5.1 创建 `openspec/specs/opsx-foundation/spec.md`，覆盖 OPSX 基础目录结构、仓库级配置、代理导航、能力地图和变更工作流。
- [x] 5.2 创建 `openspec/specs/shared-platform-primitives/spec.md`，覆盖共享契约、HTTP 与安全中间件、runtime primitive 和测试基础设施。
- [x] 5.3 创建 `openspec/specs/user-identity-management/spec.md`，覆盖用户创建、查询、列表、状态约束和用户 HTTP 边界。
- [x] 5.4 创建 `openspec/specs/auth-session-management/spec.md`，覆盖登录、令牌签发、刷新、退出、改密和认证 HTTP 边界。
- [x] 5.5 创建 `openspec/specs/rbac-access-control/spec.md`，覆盖权限目录、角色权限绑定、用户角色绑定、Casbin 授权和 RBAC 系统数据引导。
- [x] 5.6 创建 `openspec/specs/runtime-observability/spec.md`，覆盖健康检查、OpenAPI、metrics、tracing、日志、错误可观测性和部署观测资产。
- [x] 5.7 创建 `openspec/specs/delivery-operations/spec.md`，覆盖构建运行、测试 lint、架构边界检查、代码生成、数据库迁移和发布部署资产。
- [x] 5.8 检查每个主规格都使用 `## Requirements`、`### Requirement:` 和 `#### Scenario:` 结构，并包含主流程、异常流程或边界场景。

## 6. 提供基础模板内容

- [x] 6.1 在 `docs/opsx/CHANGE_WORKFLOW.md` 中加入 proposal、design、tasks 和 spec delta 的最小中文模板，供后续协作者参考。
- [x] 6.2 在 `docs/opsx/CHANGE_WORKFLOW.md` 中说明哪些内容属于 OpenSpec `context` 与 `rules` 约束，不能直接复制进 artifact 正文。
- [x] 6.3 在 `AGENTS.md` 或 `docs/DEVELOPMENT.md` 中补充 OPSX 初始化说明，说明新仓库或缺失 `openspec/` 时使用 `openspec init --tools none --force`。

## 7. 验证与收尾

- [x] 7.1 执行文件存在性检查：`test -f AGENTS.md`、`test -f docs/opsx/CAPABILITY_MAP.md`、`test -f docs/opsx/CHANGE_WORKFLOW.md`、`test -f openspec/config.yaml`。
- [x] 7.2 执行规格数量和格式检查：`find openspec/specs -name spec.md | wc -l`，并确认每个 spec 包含 `Requirement` 与 `Scenario`。
- [x] 7.3 执行 OpenSpec 检查：`openspec list --specs`、`openspec validate --specs` 和 `openspec status --change create-opsx-foundation`。
- [x] 7.4 执行仓库文档约束检查：`make user-service-architecture-lint`。
- [x] 7.5 如时间允许，执行 `make verify` 确认 lint、测试、OpenAPI 生成和 git diff 检查均通过。
- [x] 7.6 汇总新建文件、主规格能力基线、验证结果和后续 `/opsx:archive create-opsx-foundation` 归档提醒。
