## Why

仓库当前固定 Go 1.26.5，并使用 `github.com/google/uuid` 表达跨模块 UUID。Go 1.27 已提供标准库 `uuid`；升级到当前最新的 1.27 补丁版并迁移 UUID，可统一工具链、减少项目直接依赖，同时保持现有 ID 契约。

## What Changes

- 将仓库当前涉及 Go 版本的全部有效配置升级到 Go 1.27 当前最新稳定补丁版 1.27.3，包括四个 Go module、`go.work`、CI、Docker 构建镜像和版本检查 fixture，并同步 README、开发及架构文档。
- 将项目直接使用的 `github.com/google/uuid` 替换为 Go 标准库 `uuid`；调整生成、解析、零值判断及版本断言，保持用户、角色、权限、会话等 ID 的现有格式与 UUID v7 默认生成策略。
- 适配 Ent schema 的 UUID 值读写，重新生成 Ent 代码并验证 PostgreSQL 列类型及迁移结果不变；清理不再需要的直接依赖。
- **BREAKING**：构建本仓库需要 Go 1.27.3；`common` 导出函数与服务内接口使用的 UUID Go 类型从第三方包类型变为标准库类型，源码调用方需要同步迁移。

## Capabilities

### New Capabilities

无。

### Modified Capabilities

- `delivery-operations`：交付工具链统一使用 Go 1.27.3，并确保生成与数据库结构无漂移。
- `shared-platform-primitives`：跨服务 UUID 原语使用标准库类型，维持默认 UUID v7、解析和对外字符串语义。

## Impact

- 代码与依赖：`common/runtime/id`、user-service 的 user/auth/role/permission、Ent schema 与生成物、四个 module metadata 和 `go.work`；`github.com/google/uuid` 若仍作为第三方传递依赖，不强行从依赖图移除。
- 构建与部署：GitHub Actions、两个 Go builder Dockerfile、架构 lint fixture 和本地 Go 工具链需要同步；运行时镜像及发布顺序保持现有契约。
- 外部契约：HTTP API、OpenAPI、JWT/Redis 中的 UUID 字符串格式及 PostgreSQL `uuid` 列保持不变；不预期新增 SQL migration、观测资产或安全策略变更，实施时以生成 diff 和集成测试确认。
- 文档与规格：同步 `README.md`、`docs/ARCHITECTURE.md`、`docs/DEVELOPMENT.md`、`openspec/config.yaml` 及对应 capability delta。
