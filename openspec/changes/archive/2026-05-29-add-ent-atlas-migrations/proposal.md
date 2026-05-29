## Why

当前项目已有 Ent schema 和 PostgreSQL 基础设施，但缺少可审查、可部署、可回放的数据库迁移工作流。引入基于 Ent 与 Atlas 的 SQL 迁移方案，可以避免运行时 `client.Schema.Create(ctx)` 带来的不可控变更，并支持多服务演进时对迁移文件进行版本化审查与 CI/CD 部署。

## What Changes

- 新增数据库迁移能力，定义从 Ent schema 生成 Atlas SQL migration、人工 review、校验和部署执行的标准流程。
- 采用服务内独立维护迁移目录的方案，以 `user-services` 为首个落地点，保障微服务解耦、镜像打包和部署独立性。
- 增加 Atlas 配置、Ent schema loader、迁移生成脚本和部署前执行脚本的设计与实施任务。
- 增加 Docker/CI 集成约束，确保 `.sql` 迁移文件和 `atlas.sum` 一起进入镜像并在服务启动前可执行。
- 明确人工修改 SQL 后必须重新计算 Atlas 校验和，避免破坏 migration directory integrity。
- 不引入运行时自动建表，不修改现有 HTTP API、响应信封或用户查询语义。

## Capabilities

### New Capabilities
- `database-schema-migrations`: 覆盖服务内 Ent schema 到 Atlas SQL migration 的生成、审查、校验、打包与部署执行流程。

### Modified Capabilities
- `shared-infrastructure`: 扩展数据库配置与部署约定，使迁移执行可复用现有 PostgreSQL 连接配置或等价环境变量，不改变现有 Fx 基础设施运行时行为。

## Impact

- 影响代码与配置：`user-services/ent/schema/`、`user-services/atlas.hcl`、`user-services/ent/migrate/`、`user-services/migrations/`、`user-services/scripts/`、Dockerfile 或 entrypoint 相关文件。
- 影响开发流程：修改 Ent schema 后需运行迁移生成命令，提交生成的 `.sql` 文件和 `atlas.sum`，并在 review 阶段允许安全人工微调 SQL。
- 影响依赖：需要 Atlas CLI，并可能增加 `ariga.io/atlas-provider-ent` 作为开发期 schema source 集成依赖。
- 影响部署：镜像需包含迁移目录，CI/CD 或容器启动前执行 `atlas migrate apply`；迁移失败时服务不应继续启动。
- 兼容性：不改变现有 API、错误码和数据读取路径；数据库结构变更需通过 migration 文件显式管理。
