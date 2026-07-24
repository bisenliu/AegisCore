# AegisCore User Service

`user-service` 是 AegisCore 当前用户服务 Go module，模块路径为 `github.com/aegiscore/user-service`。服务通过 Cobra 暴露 CLI，通过 Uber Fx 组装运行时，通过 Gin 暴露 HTTP API，通过 Ent/PostgreSQL 持久化，通过 Redis 管理 refresh session 和 RBAC policy sync。

## Entry Points

| Path | Purpose |
|---|---|
| `cmd/main.go` | CLI 入口，注册 `serve` 和 `rbac` 子命令 |
| `cmd/rbac.go` | 离线 RBAC seed 和一次性 super-admin bootstrap |
| `internal/bootstrap/` | Fx app、HTTP server lifecycle 和独立 pprof 诊断监听 |
| `internal/providers/` | 服务级 Gin、routes、JWT、PostgreSQL、Redis、Ent、metrics/tracing provider |
| `internal/router/` | `/api/v1` route graph、health、metrics、OpenAPI routes |
| `internal/features/user` | 用户资料 feature |
| `internal/features/auth` | 认证会话 feature |
| `internal/features/role` | 角色管理 feature |
| `internal/features/permission` | 权限目录、RBAC 授权和 policy sync feature |
| `internal/shared` | 服务内稳定业务内核：`identity`、`rbacbaseline` |
| `internal/integration` | 未来真实外部系统防腐层边界 |
| `ent/schema` | Ent schema source |
| `migrations` | Atlas SQL migration directory |
| `scripts` | migration 和 OpenAPI generation scripts |

## HTTP Runtime

System endpoints:

- `GET /livez`
- `GET /readyz`
- `GET /startupz`
- `GET /metrics` when metrics are enabled
- OpenAPI UI/JSON/routes when enabled

Business APIs are mounted under `/api/v1` and include auth, users, roles, permissions, effective permissions, and route diff. Protected business routes require JWT authentication and RBAC authorization.

## Runtime Config

共享核心配置只包含 `app/runtime/server/log/observability`。user-service 在 `resources.redis.cache_redis` 和 `resources.postgres.primary_db` 声明外部资源，在 `auth.token_version_cache` 和 `rbac.user_role_cache` 声明服务私有配置。配置只从显式 `--config` 指定的一份完整 YAML 文件读取；未知 YAML 字段会在启动前失败。

日志只写 stdout/stderr，tracing 启用后固定使用 OTLP，进程时区使用 `runtime.timezone`。Gin 不信任代理，代理信任由 Ingress、gateway 或 service mesh 入口策略负责。pprof 默认关闭；临时诊断需修改当前环境的完整配置文件并使用受控端口转发。

## Feature Rules

- Business code is feature-first under `internal/features/<feature>`.
- `transport/http` owns Gin controller, routes, request/response DTOs, OpenAPI DTOs, and input preparers.
- `application` owns command/query/use case and consumer-side ports.
- `domain` owns pure entities, value objects, errors, and rules.
- `infrastructure/postgres` owns Ent/PostgreSQL adapters.
- `infrastructure/redis` owns Redis adapters and feature key schema.
- `fx.go` owns wiring only.

## 本地命令

从仓库根目录执行：

```bash
make user-service-run
make user-service-test
make user-service-lint
make user-service-generate
make user-service-migrate-diff name=<name>
make user-service-migrate-validate
make user-service-openapi-generate
ADMIN_BOOTSTRAP_PASSWORD='<temporary-password>' make user-service-bootstrap-super-admin ADMIN_USERNAME='initial-admin'
```

在 `user-service/` 目录内执行：

```bash
make test
make generate
make migrate-validate
make openapi-generate
ADMIN_BOOTSTRAP_PASSWORD='<temporary-password>' make bootstrap-super-admin ADMIN_USERNAME='initial-admin'
```

## Migration 与 RBAC Seed

推荐发布顺序：

1. 按 Ent schema -> Atlas diff 生成 SQL -> Atlas validate/hash 校验 SQL 目录 -> SQL 进 Git -> DBA 工单或受控发布平台执行的流程，确认服务拥有的 `primary_db` 已完成本 release 对应 SQL migration。
2. 执行 `rbac seed`、`make user-service-seed-rbac` 或在服务目录执行 `make seed-rbac` 初始化系统 RBAC 数据。
3. 在全新数据库上通过 `ADMIN_BOOTSTRAP_PASSWORD='<temporary-password>' make user-service-bootstrap-super-admin ADMIN_USERNAME='initial-admin' ADMIN_NICKNAME='Initial Administrator'`，或在服务目录执行对应 `make bootstrap-super-admin`，一次性创建初始超级管理员账号。密码只从 `ADMIN_BOOTSTRAP_PASSWORD` 环境变量读取。
4. 启动或滚动更新 HTTP server 副本。
5. 初始管理员使用临时密码登录并完成强制改密。

如果 seed 或 `bootstrap-super-admin` 在副本运行中执行，必须滚动重启副本或触发在线 policy refresh。这些命令是离线运维工具，不是运行期 policy sync。

普通 user-service 运行时镜像不包含 Atlas，也不会因 `RUN_MIGRATIONS=true` 执行 migration。容器化发布必须先确认 SQL migration 已通过 DBA 工单或受控发布平台执行完成，再启动或滚动 HTTP 副本。若 SQL 包含 `CREATE EXTENSION IF NOT EXISTS pg_trgm;`，生产库可能需要 DBA 权限或前置动作。
