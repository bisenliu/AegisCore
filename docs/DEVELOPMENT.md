# Development

## 1. Prerequisites

- Go workspace 使用 `go 1.26` 和 `toolchain go1.26.3`，见 `go.work`。
- 工具链基线由 `openspec/specs/go-toolchain-baseline/spec.md` 约束；修改 `go.work` 或任一 `go.mod` 的 Go/toolchain 版本时需同步更新该规格和文档。
- 本地运行用户服务需要 PostgreSQL 和 Redis。
- 生成或执行数据库迁移需要 Atlas CLI，用户服务迁移目标通常指向 `postgres.user_db` 或部署环境提供的 `DATABASE_URL`。
- 用户服务配置示例位于 `user-services/configs/config.yaml`。

## 2. Workspace Layout

- `common/go.mod`：共享 Go 模块，模块路径 `github.com/aegiscore/common`。
- `user-services/go.mod`：用户服务 Go 模块，模块路径 `github.com/aegiscore/user-services`。
- `go.work`：将两个模块纳入同一个 workspace。

## 3. Common Commands

| 任务 | 命令 | 执行目录 |
|---|---|---|
| 运行全部测试 | 分别执行 `go test ./...` | `common/` 和 `user-services/` |
| 运行用户服务 | `go run ./user-services/cmd serve --config ./user-services/configs/config.yaml` | 仓库根目录 |
| 运行单模块测试 | `go test ./...` | `common/` 或 `user-services/` |
| 生成 Ent 代码 | `go generate ./ent` | `user-services/` |
| 生成用户服务数据库迁移 | `./scripts/migrate-diff.sh <name>` | `user-services/` |
| 校验用户服务迁移目录 | `./scripts/migrate-validate.sh` | `user-services/` |
| 执行用户服务数据库迁移 | `DATABASE_URL='<postgres-url>' ./scripts/migrate-apply.sh` | `user-services/` |
| 格式化 Go 文件 | `gofmt -w <files>` | 任意目录 |

## 4. Configuration

配置加载逻辑位于 `common/config/loader.go`。

- 默认配置文件路径由服务命令传入，`serve` 默认使用 `./configs/config.yaml`。
- 从仓库根目录运行时应显式传入 `./user-services/configs/config.yaml`。
- 环境变量前缀为 `AEGISCORE`。
- 配置 key 中的 `.` 和 `-` 会映射为环境变量中的 `_`。
- Redis 使用 `redis.<name>` 命名实例，例如 `redis.cache_redis`、`redis.queue_redis`。
- PostgreSQL 使用 `postgres.<name>` 命名实例，例如 `postgres.user_db`、`postgres.common_db`、`postgres.pay_db`。
- 用户服务当前声明 `cache_redis`、`user_db` 和 `common_db`；`pay_db` 可存在于配置中，但不代表支付连接池或支付业务已启用。
- `common/config.Load` 只负责读取 YAML、应用 `AEGISCORE_` 覆盖并反序列化为配置对象；缺失字段、零值端口或无效范围不在加载阶段被字段校验拒绝，后续初始化或依赖库会暴露运行时失败。

示例：`AEGISCORE_HTTP_PORT=8081` 可覆盖 `http.port`。

## 5. Coding Conventions

- HTTP 层只处理请求解析、参数校验和响应输出。
- Service 层负责业务编排和 DTO 映射。
- Repository 层负责 Ent/数据库访问和存储错误转换。
- 共享中间件、响应模型、配置和基础设施放在 `common/`。
- Ent 生成代码不要手动编辑；修改 schema 后重新生成。
- Go 文件提交前运行 `gofmt`。

## 6. Database Migrations

用户服务采用服务内迁移目录方案：Ent schema、业务代码、Atlas 配置和 SQL migration 都由 `user-services/` 自己维护。这样可以保持服务发布、镜像打包和 CI/CD 迁移执行独立，不需要在仓库根目录集中维护所有服务的迁移文件。

### 6.1 Directory Layout

```text
user-services/
  atlas.hcl
  ent/
    schema/
    migrate/main.go
  migrations/
    atlas.sum
    *.sql
  scripts/
    migrate-diff.sh
    migrate-validate.sh
    migrate-apply.sh
    entrypoint.sh
```

### 6.2 Generate SQL Migrations

1. 修改 `user-services/ent/schema/` 下的 Ent schema。
2. 在 `user-services/` 执行 `go generate ./ent`，只生成 Ent 代码，不要手写 `user-services/ent/` 下的生成文件。
3. 在 `user-services/` 执行 `./scripts/migrate-diff.sh <migration-name>`。
4. 审查 `user-services/migrations/*.sql` 和 `user-services/migrations/atlas.sum`。

`migrate-diff.sh` 使用 Atlas 的 `ent://ent/schema` schema source 读取 Ent schema，并通过 PostgreSQL dev database 计算与现有 migration directory 的差异。默认 dev URL 为 `docker://postgres/15/dev?search_path=public`，可通过 `ATLAS_DEV_URL` 覆盖。

### 6.3 Review And Manual SQL Edits

Atlas 生成的 SQL 必须提交前 review。允许手动调整 SQL 以满足 PostgreSQL 生产安全要求，例如将普通索引调整为并发索引：

```sql
-- atlas:txmode none
CREATE INDEX CONCURRENTLY "users_email_idx" ON "users" ("email");
```

`CREATE INDEX CONCURRENTLY` 不能在事务中执行，因此需要将该语句放在非事务 migration 中，或按 Atlas 支持的事务模式指令拆分 migration。任何手动修改 SQL 后，都必须在 `user-services/` 执行：

```bash
atlas migrate hash --dir file://migrations
./scripts/migrate-validate.sh
```

如果 SQL 文件与 `atlas.sum` 不一致，`atlas migrate validate --dir file://migrations` 会失败，CI/CD 不得继续部署。

### 6.4 Apply In CI/CD Or Entrypoint

推荐在 CI/CD release job 中执行迁移，再启动或滚动发布服务：

```bash
cd user-services
./scripts/migrate-validate.sh
DATABASE_URL='postgres://user:pass@host:5432/aegiscore_user?sslmode=require&search_path=public' ./scripts/migrate-apply.sh
```

如果发布平台无法提供独立 migration job，也可以使用容器 `entrypoint.sh` 在启动前执行迁移。容器启动前迁移会增加启动耗时，并且多副本并发启动时需要依赖 Atlas migration lock 和发布平台副本策略；生产环境优先使用单独 migration job。

`DATABASE_URL` 必须指向用户服务拥有的 `user_db`，不要因为配置中存在 `pay_db` 或 `common_db` 而迁移非目标数据库。

## 7. API Conventions

- 成功响应使用 `common/response.OK` 或 `common/response.Created`。
- 失败响应使用 `common/response.Fail` 或便捷方法 `BadRequest`、`NotFound`。
- 响应信封字段为 `success`、`code`、`message`、`data`。
- API 错误码目前包括 `OK`、`BAD_REQUEST`、`NOT_FOUND`、`INTERNAL_ERROR`。

## 8. Logging And Trace ID

- 日志使用 `common/logger` 提供的 Zap 封装和 context API。
- HTTP trace header 是 `X-Trace-ID`，日志字段统一为 `trace-id`。
- trace-id 中间件会将 trace-id 写入 Gin context、Go `context.Context` 和响应头。
- 业务代码优先通过 `common/logger.Info(ctx, ...)`、`Warn(ctx, ...)`、`Error(ctx, ...)` 输出日志，避免绕过 context helper 导致 trace-id 丢失。
- Error 级别日志默认不自动添加 stacktrace；关键运行时错误需要显式传入 `logger.StackTrace(...)` 或 `zap.Stack("stacktrace")`。
- 文件日志当天活动文件使用 `aegiscore-user-services.<level>.log`，跨天后归档为 `aegiscore-user-services-yyyy-mm-dd.<level>.log`。

## 9. Adding Features

1. 在 `docs/opsx/CAPABILITY_MAP.md` 中定位或新增 capability。
2. 如新增长期能力，先添加 `openspec/specs/<capability>/spec.md`。
3. 使用 `/opsx:propose <change-name>` 生成 change artifacts。
4. 使用 `/opsx:apply <change-name>` 实现。
5. 增加或更新测试，并在受影响模块目录运行相关 `go test` 命令；跨模块变更时分别在 `common/` 和 `user-services/` 运行。

## 10. Local Runtime Notes

用户服务启动时会 ping Redis 和 PostgreSQL。若本地没有外部依赖，启动会失败。开发纯业务逻辑时优先通过单元测试覆盖 service/repository 边界，集成验证再连接真实依赖。
