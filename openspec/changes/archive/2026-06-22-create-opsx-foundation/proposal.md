## Why

AegisCore 的 README 已将 `docs/opsx/` 与 `openspec/` 定义为协作入口，但当前仓库尚缺少可执行的 OPSX 基础框架、仓库级 OpenSpec 规则和主规格基线。现在补齐这些底座，可以让后续跨模块、契约、部署或行为变更先经过一致的 `/opsx:*` 流程，减少每次变更前的重复侦查和隐性约定。

## What Changes

- 创建完整 OPSX 文档框架，覆盖代理导航、架构、开发、产品、测试、能力地图和变更工作流。
- 建立仓库级 `openspec/config.yaml`，明确简体中文输出、Go workspace 技术栈、分层约束、design 规则和 tasks 规则。
- 在 `openspec/specs/` 建立主规格基线，先覆盖当前仓库中长期稳定且会被后续 change 反复引用的核心能力。
- 将 README 已声明的 `docs/opsx/`、`openspec/` 与真实文件结构对齐，提供初始化说明、使用说明和基础模板内容。
- 不修改业务代码、HTTP API、数据库 schema、部署行为或运行时依赖。

## Capabilities

### New Capabilities

- `opsx-foundation`: 仓库级 OPSX/OpenSpec 基础框架，覆盖目录结构、配置规则、文档入口、能力地图、变更工作流和验证约束。
- `shared-platform-primitives`: `common/` 提供的跨服务契约、HTTP helper、配置加载、安全原语、runtime primitive、测试基础设施和校验能力。
- `user-identity-management`: `user-service` 的用户资料创建、查询、列表、状态校验和用户存储边界。
- `auth-session-management`: 用户登录、令牌签发、刷新、改密、当前会话退出、全部会话退出和 token version 校验。
- `rbac-access-control`: 权限目录、角色管理、用户角色绑定、Casbin 授权、默认 RBAC seed 和超级管理员引导能力。
- `runtime-observability`: 健康检查、OpenAPI、metrics、tracing、logging、pprof、Prometheus/Grafana 资产和运行时可观测性约束。
- `delivery-operations`: 构建、测试、lint、OpenAPI 生成、Ent/Atlas migration、Docker、Compose、Kubernetes、Helm 和发布顺序相关的交付运维能力。

### Modified Capabilities

- 无。当前仓库不存在可修改的 OpenSpec 主规格，本次只新增 OPSX 基线和现有能力的规格化描述。

## Impact

- 受影响路径：`AGENTS.md`、`docs/ARCHITECTURE.md`、`docs/DEVELOPMENT.md`、`docs/PRODUCT.md`、`docs/TESTING.md`、`docs/opsx/`、`openspec/config.yaml`、`openspec/specs/`。
- 受影响协作流程：后续跨 feature、跨模块、外部契约、schema、部署或行为变更应先使用 `/opsx:explore`、`/opsx:propose`、`/opsx:apply`、`/opsx:verify`、`/opsx:archive`。
- 验证重点：OpenSpec 状态与校验、Markdown 中文约束、主规格 Given/When/Then 场景完整性、能力地图中的代码路径与规格路径一致性。
- 运行时影响：无。该 change 只创建文档、配置和规格基础设施，不改变 Go 代码、接口响应、数据库迁移或部署清单。
