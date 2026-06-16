# Development

## 1. Prerequisites

- Go workspace 使用 `go 1.26` 和 `toolchain go1.26.3`，见 `go.work`。
- 工具链基线由 `go.work`、`common/go.mod`、`user-service/go.mod` 和本文档共同说明；修改 Go/toolchain 版本时需同步更新这些文件。
- 本地代码规范检查使用 `golangci-lint`，建议安装与 CI 一致的版本：`go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2`。
- 本地运行用户服务需要 PostgreSQL 和 Redis。
- 生成或执行数据库迁移需要 Atlas CLI，用户服务迁移目标通常指向 `postgres.user_db` 或部署环境提供的 `DATABASE_URL`。
- 用户服务配置示例位于 `user-service/configs/config.yaml`。

## 2. Workspace Layout

- `common/go.mod`：共享 Go 模块，模块路径 `github.com/aegiscore/common`。
- `user-service/go.mod`：用户服务 Go 模块，模块路径 `github.com/aegiscore/user-service`。
- `go.work`：将两个模块纳入同一个 workspace。

## 3. Common Commands

| 任务 | 命令 | 执行目录 |
|---|---|---|
| 查看统一入口 | `make help` | 仓库根目录 |
| 构建用户服务二进制 | `make build` 或 `make build-user-service` | 仓库根目录 |
| 运行全部测试 | `make test` | 仓库根目录 |
| 运行用户服务 | `make run-user-service` | 仓库根目录 |
| 初始化或更新 RBAC 系统数据 | `make seed-rbac` | 仓库根目录 |
| 构建用户服务 Docker 镜像 | `docker build -f deployments/docker/user-service.Dockerfile -t aegiscore-user-services .` | 仓库根目录 |
| 运行共享模块测试 | `make test-common` | 仓库根目录 |
| 运行用户服务测试 | `make test-user-service` | 仓库根目录 |
| 运行全部 lint | `make lint` | 仓库根目录 |
| 运行共享模块 lint | `make lint-common` | 仓库根目录 |
| 运行用户服务 lint | `make lint-user-service` | 仓库根目录 |
| 运行架构边界检查 | `make architecture-lint` | 仓库根目录 |
| 运行完整本地验证 | `make verify` | 仓库根目录 |
| 生成 Ent 代码 | `make generate` | 仓库根目录 |
| 生成用户服务数据库迁移 | `make migrate-diff name=<name>` | 仓库根目录 |
| 校验用户服务迁移目录 | `make migrate-validate` | 仓库根目录 |
| 执行用户服务数据库迁移 | `DATABASE_URL='<postgres-url>' make migrate-apply` | 仓库根目录 |
| 生成 OpenAPI 3 文档 | `make openapi-generate`；或 `cd user-service && ./scripts/openapi-generate.sh` | 仓库根目录；或 `user-service/` |
| 格式化 Go 文件 | `gofmt -w <files>` | 任意目录 |

Makefile 只是统一入口：测试和 lint 仍分别进入 `common/` 与 `user-service/` 执行，迁移和 OpenAPI 操作仍委托 `user-service/scripts/` 下的现有脚本。OpenAPI 生成脚本从 `user-service/` 模块目录运行，将解析范围限定为 `github.com/aegiscore/user-service` 包前缀，先从源码注解生成临时 Swagger 2 输入，再通过 `common/http/openapi` 的共享转换 helper 生成最终 OpenAPI 3 文档；服务侧脚本或薄 wrapper 继续拥有 API server、健康探针路径、认证方案和输出目录等服务语义。直接运行底层命令仍然可用，排错时可参考对应脚本和执行目录。

## 4. Local Runtime And Deployment Assets

- 本地直接运行用户服务：先准备 PostgreSQL 和 Redis，再执行 `make run-user-service`。
- RBAC 系统角色、系统权限和系统角色权限绑定需要显式初始化或更新：推荐顺序为先执行数据库 migration，再执行 `make seed-rbac`，最后启动 HTTP server。seed 不会在 `serve` 启动时自动执行，也不会发布运行期 policy refresh；不要把 `make seed-rbac` 当作在线授权变更入口。
- 生产环境若必须在已有 HTTP 副本运行中执行 `make seed-rbac` 或 `rbac assign-super-admin`，执行后需要滚动重启服务副本，或通过正式 RBAC 管理接口触发一次在线 policy refresh，确保所有副本的内存 Casbin policy 重新加载。日常在线权限、角色状态、用户角色绑定和角色权限绑定变更应使用 HTTP RBAC 管理接口，该路径会通过 Redis policy version、Pub/Sub 和版本补偿检查同步多副本。
- 新增或调整进入 RBAC 授权中间件的业务路由时，必须同步更新 `user-service/internal/shared/rbacbaseline` 中的系统权限 URL catalog，重新执行 `make seed-rbac`，并通过 `GET /api/v1/permissions/route-diff` 检查权限目录与已注册路由是否仍一致。
- 构建用户服务容器镜像：从仓库根目录执行 `docker build -f deployments/docker/user-service.Dockerfile -t aegiscore-user-services .`。该 Dockerfile 依赖仓库根目录作为 build context，以便复制 `go.work`、`common/` 和 `user-service/`。
- 本地 Compose 文件归属 `deployments/compose/`。当前没有可运行 Compose file；若本地没有 PostgreSQL/Redis，需要按 `user-service/configs/config.yaml` 中的配置自行准备依赖。
- Kubernetes YAML 归属 `deployments/k8s/`，Helm chart 归属 `deployments/helm/`。当前目录只声明边界，不提供可直接部署的生产资源。
- 用户服务探针路径为：`/livez` 用于 liveness，`/readyz` 用于 readiness，`/startupz` 用于 startup probe。`/readyz` 和 `/startupz` 会检查 PostgreSQL、Redis、Casbin policy 加载状态和 RBAC policy watcher 状态。

## 5. Configuration

配置加载逻辑位于 `common/runtime/config/loader.go`。

- 默认配置文件路径由服务命令传入，`serve` 默认使用 `./configs/config.yaml`。
- 从仓库根目录运行时应显式传入 `./user-service/configs/config.yaml`。
- 环境变量前缀为 `AEGISCORE`。
- 配置 key 中的 `.` 和 `-` 会映射为环境变量中的 `_`。
- Redis 使用 `redis.<name>` 命名实例，例如 `redis.cache_redis`、`redis.queue_redis`。
- PostgreSQL 使用 `postgres.<name>` 命名实例，例如 `postgres.user_db`、`postgres.pay_db`。
- 用户服务当前声明 `cache_redis` 和 `user_db`；其他命名实例可存在于配置中作为示例或其他服务配置，但不代表用户服务会自动连接对应资源或启用相关业务。
- Redis key 的通用构造规则使用 `common/runtime/rediskey`；具体 key schema 放在 owning feature 的 `infrastructure/redis` 或 owning runtime primitive 内，不放入 `common` 的通用 key 大表。
- 可观测性配置位于 `observability`：`observability.metrics.enabled` 控制未来 metrics wiring 是否消费该开关，`observability.metrics.path` 默认 `/metrics`，`observability.metrics.include_runtime` 控制未来是否包含 Go runtime/process 指标；`observability.tracing.enabled` 控制 tracing 配置入口，`observability.tracing.sample_ratio` 范围为 `0.0` 到 `1.0`，`observability.tracing.exporter` 当前支持 `none` 和 `otlp`，`observability.tracing.otlp_endpoint` 只在 `exporter: otlp` 时必填，`observability.tracing.insecure` 在生产类环境中不能与 `otlp` exporter 同时使用。
- 当前阶段 observability 只建立配置契约，不注册 `/metrics` 路由、不创建 metrics exporter、不创建 OpenTelemetry tracer provider、不接入 Gin tracing middleware。用户服务本地默认 `observability.tracing.exporter: none`，因此不强制部署 `otel-collector:4317`。
- OTLP endpoint 不应包含 token、Authorization header、账号密码、Cookie 或其他敏感凭据；未来 exporter 认证需要单独设计 Secret 注入方式。
- `common/runtime/config.Load` 会读取 YAML、应用 `AEGISCORE_` 覆盖、反序列化为配置对象，并在返回前执行结构化字段校验；缺失必填字段、非法端口、非正超时、无效 Redis/PostgreSQL named config 或生产环境不安全配置会在启动期被拒绝。

示例：`AEGISCORE_HTTP_PORT=8081` 可覆盖 `http.port`。

示例：`AEGISCORE_OBSERVABILITY_TRACING_EXPORTER=otlp` 可覆盖 `observability.tracing.exporter`，配套 `AEGISCORE_OBSERVABILITY_TRACING_OTLP_ENDPOINT=localhost:4317` 可设置本地 Collector endpoint。

## 6. Coding Conventions

- HTTP 层只处理请求解析、参数校验和响应输出。
- HTTP request/response DTO 和 OpenAPI 文档 model 放在对应 feature 的 `transport/http/request.go`、`response.go`，OpenAPI 生成使用稳定 Go struct/type 名称，避免生成文档暴露 `transport/http` 目录派生名称；不要新增 feature-level `api/` DTO 包。
- Application service/use case 层负责业务编排、command/query 处理和 DTO 映射；已有 feature 可在 `application/command`、`application/query`、`application/validators` 和稳定组件包下继续细分读写用例与 transport-neutral 输入辅助。Auth 的登录、刷新、强制改密和登出流程放在扁平 `application/command`；认证上下文 helper 放在 `application/authctx`；凭据校验和强制改密凭据更新放在 `application/credentials`；JWT 签发解析和 token result DTO 放在 `application/tokens`；refresh session 生命周期、token version fallback、session 上限和会话撤销放在 `application/sessions`；认证 application 输入辅助、token version 撤销校验和 refresh session 一致性校验放在 `application/validators`。
- Domain 层负责实体、值对象、枚举、领域错误和纯业务规则；只有存在真实跨实体/跨值对象领域规则或领域事件模型时，才在 feature 内新增 `domain/services` 或 `domain/events`，不得为了目录完整添加空业务代码。Domain services/events 不依赖 Gin、Ent、Redis、config、logger、application ports 或 infrastructure adapter。
- Infrastructure adapter 层负责 Ent/PostgreSQL、Redis 访问、存储模型转换和存储错误转换，具体放在对应 feature 的 `infrastructure/postgres` 或 `infrastructure/redis` 下。
- 服务级 Fx provider 放在 `user-service/internal/providers`，用于组装 Gin engine、HTTP route registration、JWT service、PostgreSQL/Redis named resources 和 Ent clients；`internal/bootstrap` 只保留顶层 AppModule 和 HTTP server lifecycle。
- 共享中间件、响应模型、配置和基础设施放在 `common/` 的对应能力分类目录中：响应 DTO 契约使用 `common/contract/response`，Gin 请求绑定和校验失败响应适配使用 `common/http/binding`，Gin 响应输出使用 `common/http/response`，OpenAPI 构建期转换和 Go embed 渲染 helper 使用 `common/http/openapi`，运行时基础能力使用 `common/runtime`，HTTP/Gin 适配使用 `common/http`，安全凭证原语使用 `common/security`，通用校验核心使用 `common/validation`。
- `common` 只承载跨服务稳定契约和基础能力；用户服务独有规则、DTO 映射、infrastructure adapter 行为或仅为未来可能复用的 helper 应保留在 `user-service` 内。
- Ent 生成代码不要手动编辑；修改 schema 后重新生成。生成代码边界、`go generate ./ent` 用法和新增 Entity Schema 流程见 `user-service/ent/README.md`。
- Go 文件提交前运行 `gofmt`。
- 提交前建议在受影响 Go module 中运行 `golangci-lint run ./...`；CI lint failure 会阻断合并。完整 lint 配置、CI/pre-commit 集成和问题治理方案见 `docs/GO_LINT_AUTOMATION.md`。
- GitHub Actions 当前还会在 PR 和主线 push 上运行测试、构建、Docker 镜像构建、race、coverage artifact、govulncheck、gosec、Trivy 扫描和 SBOM artifact；本地可优先用 `make test`、`make build` 和 Docker build 命令复现核心门禁。

### 6.1 Lint Troubleshooting

- `gofmt` 或 `goimports` 失败时，先格式化对应文件并整理 imports。
- `errcheck` 失败时，优先显式处理错误；确认可忽略时用 `_ = fn()` 表达有意忽略。
- `govet` 或 `staticcheck` 失败时，按潜在真实 bug 优先排查，不建议直接排除。
- CI 与本地结果不一致时，检查 Go 版本、`golangci-lint` 版本、执行目录和 `../.golangci.yml` 配置路径是否一致。
- 生成代码产生 lint 噪声时，确认排除规则只覆盖 Ent 生成代码，不覆盖 `user-service/ent/schema/`。

## 7. Database Migrations

用户服务采用服务内迁移目录方案：Ent schema、业务代码、Atlas 配置和 SQL migration 都由 `user-service/` 自己维护。这样可以保持服务发布、镜像打包和 CI/CD 迁移执行独立，不需要在仓库根目录集中维护所有服务的迁移文件。

### 7.1 Directory Layout

```text
user-service/
  ent/
    schema/
    migrate/main.go
  migrations/
    atlas.hcl
    atlas.sum
    *.sql
  scripts/
    migrate-diff.sh
    migrate-validate.sh
    migrate-apply.sh
    entrypoint.sh
```

### 7.2 Generate SQL Migrations

1. 修改 `user-service/ent/schema/` 下的 Ent schema。
2. 在 `user-service/` 执行 `go generate ./ent`，只生成 Ent 代码，不要手写 `user-service/ent/` 下的生成文件。
3. 在 `user-service/` 执行 `./scripts/migrate-diff.sh <migration-name>`。
4. 审查 `user-service/migrations/*.sql` 和 `user-service/migrations/atlas.sum`。

Ent 生成代码边界和新增 Entity Schema 的完整说明见 `user-service/ent/README.md`。

`migrate-diff.sh` 使用 Atlas 的 `ent://ent/schema` schema source 读取 Ent schema，并通过 PostgreSQL dev database 计算与现有 migration directory 的差异。默认 dev URL 为 `docker://postgres/15/dev?search_path=public`，可通过 `ATLAS_DEV_URL` 覆盖。

Atlas 配置位于 `user-service/migrations/atlas.hcl`。迁移脚本从 `user-service/` 目录执行，并显式使用该配置文件，因此 `ent://ent/schema` 和 `file://migrations` 仍以 `user-service/` 为解析基准。

### 7.3 Review And Manual SQL Edits

Atlas 生成的 SQL 必须提交前 review。允许手动调整 SQL 以满足 PostgreSQL 生产安全要求，例如将普通索引调整为并发索引：

```sql
-- atlas:txmode none
CREATE INDEX CONCURRENTLY "users_username_idx" ON "users" ("username");
```

`CREATE INDEX CONCURRENTLY` 不能在事务中执行，因此需要将该语句放在非事务 migration 中，或按 Atlas 支持的事务模式指令拆分 migration。任何手动修改 SQL 后，都必须在 `user-service/` 执行：

```bash
atlas migrate hash --dir file://migrations
./scripts/migrate-validate.sh
```

如果 SQL 文件与 `atlas.sum` 不一致，`atlas migrate validate --dir file://migrations` 会失败，CI/CD 不得继续部署。

### 7.4 Apply In CI/CD Or Entrypoint

推荐在 CI/CD release job 中执行迁移，再启动或滚动发布服务：

```bash
cd user-service
./scripts/migrate-validate.sh
DATABASE_URL='postgres://user:pass@host:5432/aegiscore_user?sslmode=require&search_path=public' ./scripts/migrate-apply.sh
```

如果发布平台无法提供独立 migration job，也可以使用容器 `entrypoint.sh` 在启动前执行迁移。容器启动前迁移会增加启动耗时，并且多副本并发启动时需要依赖 Atlas migration lock 和发布平台副本策略；生产环境优先使用单独 migration job。

`DATABASE_URL` 必须指向用户服务拥有的 `user_db`，不要因为配置中存在其他 PostgreSQL 命名实例而迁移非目标数据库。

## 8. API Conventions

- 成功响应使用 `common/http/response.OK` 或 `common/http/response.Created`。
- 失败响应使用 `common/http/response.Fail` 或便捷方法 `BadRequest`、`NotFound`。
- 响应信封字段为 `success`、`code`、`message`、`data`。
- API 错误码目前包括 `OK`、`BAD_REQUEST`、`NOT_FOUND`、`INTERNAL_ERROR`。

## 9. Logging And Trace ID

- 日志使用 `common/runtime/logger` 提供的 Zap 封装和 context API。
- HTTP trace header 是 `X-Trace-ID`，Gin context key 和日志字段统一为 `trace_id`。
- HTTP access log 标准字段为 `trace_id`、`user_id`、`client_ip`、`method`、`path`、`status`、`latency_ms`；认证失败安全事件日志额外记录 `user_agent`。
- 认证失败日志不得记录 password、token、Authorization header、Cookie 或原始请求体。
- trace-id 中间件会将 trace-id 写入 Gin context、Go `context.Context` 和响应头。
- 业务代码优先通过 `common/runtime/logger.Info(ctx, ...)`、`Warn(ctx, ...)`、`Error(ctx, ...)` 输出日志，避免绕过 context helper 导致 trace-id 丢失。
- Error 级别日志默认不自动添加 stacktrace；关键运行时错误需要显式传入 `logger.StackTrace(...)` 或 `zap.Stack("stacktrace")`。
- 文件日志按天写入带日期的分类文件，例如 `aegiscore-user-services.2026-06-02.info.log`；其中 `aegiscore-user-services` 是运行时 service name，不是目录名。

## 10. Adding Features

1. 先阅读 `docs/ARCHITECTURE.md`，确认新能力属于哪个模块、feature 和层。
2. 新增服务内业务能力时，优先放在 `user-service/internal/features/<feature>/`；已有 feature 内按 `application/`、`domain/`、`transport/http/`、`infrastructure/*/` 和 `fx.go` 分层扩展，HTTP DTO 放在 `transport/http/request.go` 和 `response.go`。用户资料 feature 的写侧用例放在 `application/command`，读侧用例放在 `application/query`，transport-neutral application 输入辅助放在 `application/validators`；认证 feature 的会话控制 use case 放在扁平 `application/command`，支撑组件按职责放在 `application/authctx`、`application/credentials`、`application/tokens`、`application/sessions`，认证输入辅助、token version 撤销校验和 session 一致性校验放在 `application/validators`。`domain/services` 和 `domain/events` 只在有真实纯领域规则或事件模型时创建；事件总线、broker、outbox、publisher、subscriber 或异步投递需要单独设计。
3. 新增跨服务稳定基础能力时，按 `common/contract`、`common/runtime`、`common/http`、`common/security` 或 `common/validation` 归类；OpenAPI 转换这类 HTTP API 构建期辅助能力归入 `common/http`，但服务路径、认证描述和源码扫描范围仍留在各服务脚本或 wrapper。
4. 跨 feature、跨模块、外部 API、配置、数据库 schema 或目录结构变更，应在 issue、PR 描述或开发记录中写清目标、影响范围和验证方式。
5. 增加或更新测试，并在受影响模块目录运行相关 `go test` 命令；跨模块变更时分别在 `common/` 和 `user-service/` 运行。

## 11. Adding Shared Code

1. 先确认共享代码是否属于跨服务稳定契约、运行时基础能力、HTTP/Gin 适配、安全凭证原语或通用校验核心。
2. 如能力只服务于用户服务请求清洗、状态规则、DTO 映射、infrastructure adapter 行为或业务编排，保留在 `user-service` 对应分层内。
3. 如确需进入 `common`，选择对应目录：`common/contract`、`common/runtime`、`common/http`、`common/security` 或 `common/validation`，并在 `docs/ARCHITECTURE.md` 更新边界说明。进入 `common/http/openapi` 的能力不得包含某个服务的 API server、认证方案描述、健康探针路径、源码扫描范围或生成输出目录。
4. 新增或迁移共享 API 时同步更新 Go imports、测试和文档。

## 12. Local Runtime Notes

用户服务启动时会 ping Redis 和 PostgreSQL。若本地没有外部依赖，启动会失败。开发纯业务逻辑时优先通过单元测试覆盖 application service 与 infrastructure adapter 边界，集成验证再连接真实依赖。
