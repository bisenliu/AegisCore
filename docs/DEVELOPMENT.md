# Development

## Prerequisites

- Go workspace 使用 `go 1.26` 和 `toolchain go1.26.3`，见 `go.work`。
- 本地 lint 使用 `golangci-lint` v2.12.2，根 `.golangci.yml` 同时约束 `common` 与 `user-service`。
- 本地运行用户服务需要 PostgreSQL 和 Redis。
- 生成或执行 migration 需要 Atlas CLI。
- 用户服务配置示例位于 `user-service/configs/config.yaml`。

## Workspace Layout

- `common/go.mod`：共享 Go 模块，模块路径 `github.com/aegiscore/common`。
- `user-service/go.mod`：用户服务 Go 模块，模块路径 `github.com/aegiscore/user-service`，通过 `replace github.com/aegiscore/common => ../common` 使用本地 common。
- `go.work`：将两个模块纳入同一个 workspace；仓库根目录不是单一 Go module。

## Common Commands

根 `Makefile` 提供跨模块统一入口；`common/Makefile` 和 `user-service/Makefile` 承载模块级命令，协作者可按当前工作目录选择入口。

| 任务 | 命令 |
|---|---|
| 查看统一入口 | `make help` |
| 构建用户服务 | `make user-service-build` |
| 运行全部测试 | `make test` |
| 运行用户服务 | `make user-service-run` |
| 初始化或更新 RBAC 系统数据 | `make user-service-seed-rbac` |
| 创建或复用超级管理员账号 | `ADMIN_PASSWORD='<password>' make user-service-create-super-admin` |
| 重置已有超级管理员账号密码 | `ADMIN_PASSWORD='<password>' make user-service-create-super-admin ADMIN_RESET_PASSWORD=true` |
| 生成 Compose Grafana dashboard | `make compose-dashboard-generate` |
| 检查 Compose dashboard drift | `make compose-dashboard-check` |
| 构建 Docker 镜像 | `docker build -f deployments/docker/user-service.Dockerfile -t aegiscore-user-services .` |
| 运行共享模块测试 | `make common-test` |
| 运行用户服务测试 | `make user-service-test` |
| 运行全部 lint | `make lint` |
| 运行架构边界检查 | `make user-service-architecture-lint` |
| 运行完整验证 | `make verify` |
| 生成 Ent 代码 | `make user-service-generate` |
| 生成 migration | `make user-service-migrate-diff name=<name>` |
| 校验 migration | `make user-service-migrate-validate` |
| 执行 migration | `DATABASE_URL='<postgres-url>' make user-service-migrate-apply` |
| 生成 OpenAPI 3 文档 | `make user-service-openapi-generate` |

根 `Makefile` 中仓库级聚合命令可以无前缀；只作用于单个模块或服务的根目标必须使用 `common-*` 或 `user-service-*` 前缀，避免后续多服务场景下产生歧义。`make verify` 会运行 lint、`user-service-architecture-lint`、test、OpenAPI generate，并检查生成物是否有未提交 drift。

## Local Runtime

本地直接运行用户服务时，先准备 PostgreSQL 和 Redis，再执行 `make user-service-run`。RBAC seed 推荐顺序为 migration、`make user-service-seed-rbac`、按需 `ADMIN_PASSWORD='<password>' make user-service-create-super-admin`、启动或滚动更新 HTTP server。`make user-service-seed-rbac` 只初始化系统角色、系统权限和默认系统绑定，不会创建真实业务用户或分配超级管理员角色；`make user-service-create-super-admin` 是独立显式运维入口，默认不重置已有管理员密码，只有设置 `ADMIN_RESET_PASSWORD=true` 才会重置。Seed 和超级管理员创建都不是在线授权变更入口，也不会替代运行期 policy refresh。

新增或调整进入 RBAC 授权的业务路由时，必须同步更新 `user-service/internal/shared/rbacbaseline` 的系统权限 catalog，重新执行 seed，并通过 `GET /api/v1/permissions/route-diff` 检查目录与已注册路由一致性。

## Configuration

配置加载逻辑位于 `common/runtime/config/loader.go`，支持 YAML 与 `AEGISCORE_` 环境变量覆盖。Redis 使用 `redis.<name>`，PostgreSQL 使用 `postgres.<name>`。用户服务当前连接 `redis.cache_redis` 与 `postgres.user_db`；配置中存在其他 named instance 不代表服务会自动连接或迁移。

Metrics 配置位于 `observability.metrics`，tracing 配置位于 `observability.tracing`。本地默认可使用 `tracing.exporter: none` 生成 trace/span context 但不导出 span；需要导出时配置 OTLP Collector endpoint。

## Coding Conventions

- HTTP 层只处理请求解析、边界校验、DTO 到 command/query 映射和响应输出。
- HTTP request/response DTO 与 OpenAPI 文档 model 放在对应 feature 的 `transport/http/request.go`、`response.go`。
- Application 层负责业务编排、command/query 和消费侧 ports。
- Domain 层负责实体、值对象、领域错误和纯规则；不要为了目录完整创建空 `domain/services` 或 `domain/events`。
- Infrastructure adapter 负责 Ent/PostgreSQL、Redis 访问和存储错误转换。
- 服务级 Fx provider 放在 `user-service/internal/providers`；`internal/bootstrap` 只保留顶层 AppModule 和 HTTP server lifecycle。
- 共享基础能力进入 `common` 前必须跨服务稳定且无业务语义。
- Ent 生成代码不要手写；修改 schema 后运行 `make user-service-generate`。
- Go 文件提交前运行 `gofmt`。
- 注释使用中文，日志 message 使用英文，日志字段使用 snake_case。

## Migrations

1. 修改 `user-service/ent/schema/`。
2. 运行 `make user-service-generate`。
3. 运行 `make user-service-migrate-diff name=<name>`。
4. Review `user-service/migrations/*.sql` 和 `atlas.sum`。
5. 运行 `make user-service-migrate-validate`。

手动修改 SQL 后必须重新 hash 并 validate。生产迁移应在 release job 或独立 migration Job 中执行，并在 HTTP rollout 前完成。

## OPSX Workflow

跨 feature、跨模块、外部契约、目录结构或稳定行为变更应先使用 `/opsx:propose <change-name>` 生成 change artifacts，再通过 `/opsx:apply <change-name>` 实施。主规格位于 `openspec/specs/`；实现完成后如果行为边界改变，必须同步更新对应主规格并归档 change。

生成或更新 `openspec/specs/`、`openspec/changes/`、`docs/opsx/` 和 OpenSpec 相关说明时，正文、标题、需求、场景、任务和面向协作者的说明必须使用简体中文，不得保留英文模板内容；技术标识符、路径、命令和 Go symbol 可保留英文原文。
