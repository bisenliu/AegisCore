# Remove shared DB and complete CI gates

## What

移除用户服务中预留的 `common_db` 共享数据库路径，并补齐当前 CI/release 门禁。

包括：

- 删除用户服务 PostgreSQL provider 对 `postgres.common_db` 的启动依赖，只声明并连接 `postgres.user_db`。
- 删除用户服务 Ent provider 对 `common_db` 的 Ent client 输出，只保留 `name:"user_db"` 的 Ent client。
- 从配置样例、服务级 provider 测试、E2E 配置模板、架构文档和测试文档中移除“公共业务库”语义。
- 从 `common/runtime/resources` 移除 `NameCommonDB` 这类业务语义资源名；`common/runtime/datastore` 继续保留按名字创建 PostgreSQL pool 的无业务 primitive。
- 修复当前 `make lint-user-service` 的存量 P1 lint failure：`user-service/internal/bootstrap/validation_test.go` 中测试 SQL driver 的未使用 `dsn` 参数。
- 新增 GitHub Actions 门禁，覆盖测试、二进制构建、Docker 镜像构建、race、coverage、govulncheck、gosec、Trivy 和 SBOM artifact。

本变更不改变 HTTP API、数据库 schema、Atlas migration、Redis key、业务用例或 feature 分层。仓库已明确不再维护 OpenSpec/OPSX 工件，因此本 change 使用 `docs/changes/remove-shared-db-and-complete-ci-gates/`，不新增 `openspec/` 或 `docs/opsx/`。

## Why

当前用户服务同时声明 `user_db` 和 `common_db`：

- `user-service/internal/providers/postgres.go` 同时创建 `user_db` 与 `common_db` SQL pool。
- `user-service/internal/providers/ent.go` 同时把 `user_db` 与 `common_db` 包装成 Ent client。
- `user-service/configs/config.yaml` 将 `common_db` 描述为“公共业务库”。
- 当前真实业务 adapter 只注入 `name:"user_db"`，例如用户 profile 和 auth credential PostgreSQL adapter。

这说明 `common_db` 目前不是被真实用例消费的资源，而是预留的共享数据库通道。共享数据库会绕过服务契约和 feature 边界，后续拆分服务时容易形成跨服务表耦合、迁移 ownership 不清、事务边界混乱和数据权限治理困难。

CI 当前也只覆盖 lint 和 migration validation。`Makefile` 已有 `make test`、`make build`，Dockerfile 也有明确构建命令，但 GitHub Actions 尚未阻断测试失败、构建失败、镜像构建失败或常见安全扫描失败。当前本地 `make lint-user-service` 还会因 `validation_test.go` 的 unused parameter 失败，说明现有 lint 门禁会被一个小问题阻断。

## Scope

包括：

- 修改 `user-service/internal/providers/postgres.go`，让 `NamedPostgresPools` 只输出 `` UserDB *sql.DB `name:"user_db"` ``。
- 修改 `user-service/internal/providers/ent.go`，让 `NamedEntClientParams` 和 `NamedEntClients` 只处理 `user_db` Ent client，并更新关闭错误上下文。
- 更新 `user-service/internal/providers/postgres_test.go` 和 `ent_test.go` 中关于 `common_db` 的断言，改为证明 user-service 不再提供 `common_db` 和 `pay_db`。
- 更新 `user-service/internal/bootstrap/validation_test.go` 的测试配置和 SQL driver unused parameter。
- 更新 `user-service/tests/e2e/harness_test.go`，E2E 配置模板只写入 `postgres.user_db`。
- 更新 `user-service/configs/config.yaml`，删除 `common_db` 示例；可保留 `pay_db` 作为“不因配置存在自动连接”的示例，但不得把它描述为用户服务依赖。
- 更新 `AGENTS.md`、`docs/ARCHITECTURE.md`、`docs/DEVELOPMENT.md`、`docs/TESTING.md` 中关于用户服务声明数据库资源的说明。
- 从 `common/runtime/resources/resource_names.go` 删除 `NameCommonDB`。
- 更新 `common/runtime/datastore` 测试，避免依赖 `NameCommonDB` 常量；如需测试多 pool primitive，可使用测试内局部名字，例如 `"audit_db"`。
- 新增或调整 GitHub Actions workflow：
  - `test`: 运行 `make test`。
  - `build`: 运行 `make build`。
  - `image`: 运行 `docker build -f deployments/docker/user-service.Dockerfile -t aegiscore-user-services .`。
  - `race`: 分别在 `common/` 和 `user-service/` 执行 `go test -race ./...`。
  - `coverage`: 生成分模块 coverage profile，并上传 artifact；覆盖率阈值可先只记录，后续单独变更再设硬阈值。
  - `govulncheck`: 对 `common` 和 `user-service` 分别执行。
  - `gosec`: 输出 SARIF 或可下载报告。
  - `trivy`: 扫描 filesystem，并在 image 构建后扫描镜像。
  - `sbom`: 使用 Syft 或 Trivy 生成 CycloneDX/SPDX SBOM artifact。

不包括：

- 不新增公共数据库、共享 Ent client、跨服务 transaction wrapper、outbox 或 eventbus。
- 不把 `common/runtime/datastore` 改成用户服务专用 provider；它仍是按名字创建连接池的无业务 primitive。
- 不新增真实 HTTP/gRPC 外部服务契约或事件契约。
- 不修改 Ent schema、生成代码、Atlas SQL migration 或 Swagger 产物。
- 不降低现有 `.golangci.yml` 规则，也不用无说明的 `nolint` 绕过架构门禁。
- 不新增 OpenSpec/OPSX 工件。

## Acceptance Criteria

- 用户服务启动图只要求 `postgres.user_db` 和 `redis.cache_redis`；缺失 `postgres.common_db` 不再导致启动失败。
- `user-service/internal/providers` 不再输出或注入 `name:"common_db"` 的 SQL pool 或 Ent client。
- `common/runtime/resources` 不再定义 `NameCommonDB`。
- `user-service/configs/config.yaml` 不再包含“公共业务库”或 `common_db` 配置样例。
- `docs/ARCHITECTURE.md`、`docs/DEVELOPMENT.md`、`docs/TESTING.md` 和 `AGENTS.md` 均说明用户服务当前只声明 `user_db` PostgreSQL。
- `make lint-user-service` 不再因 `validation_test.go` unused parameter 失败。
- GitHub Actions 至少包含 lint、migration validation、test、build、Docker image build、race、coverage artifact、govulncheck、gosec、Trivy 和 SBOM artifact。
- `make lint`、`make test`、`make build` 通过。
- `docker build -f deployments/docker/user-service.Dockerfile -t aegiscore-user-services .` 通过，或在本地 Docker 不可用时由 CI workflow 验证。
- 没有新增 `openspec/`、`docs/opsx/`、Ent generated code、migration 或 Swagger 的无关改动。
