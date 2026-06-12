# Tasks

## Implementation

- [x] 阅读 `docs/ARCHITECTURE.md`、`docs/DEVELOPMENT.md`、`docs/TESTING.md` 和本 change 的 `proposal.md`、`design.md`，确认目标是不再声明共享数据库，并补齐 CI/release 门禁。
- [x] 从 `common/runtime/resources/resource_names.go` 删除 `NameCommonDB`。
- [x] 更新 `common/runtime/datastore/datastore_fx_test.go`，使用测试局部 PostgreSQL resource name 覆盖多 pool primitive，不再依赖 `resources.NameCommonDB`。
- [x] 修改 `user-service/internal/providers/postgres.go`，`NamedPostgresPools` 只输出 `` UserDB *sql.DB `name:"user_db"` ``。
- [x] 修改 `ProvidePostgresPools`，只请求 `resources.NameUserDB`，缺失 `postgres.common_db` 不再报错。
- [x] 修改 `user-service/internal/providers/ent.go`，`NamedEntClientParams` 只注入 `` UserDB *sql.DB `name:"user_db"` ``。
- [x] 修改 `NamedEntClients`，只输出 `` UserClient *ent.Client `name:"user_db"` ``。
- [x] 简化 Ent client lifecycle close helper，关闭错误上下文只包含 `user_db`，不再硬编码 `common_db`。
- [x] 更新 `user-service/internal/providers/postgres_test.go`，断言 provider 只打开、ping、关闭 `aegiscore_user`。
- [x] 更新 provider 测试，新增或调整缺失 `common_db` 配置不报错的覆盖。
- [x] 更新 provider 测试，保留“不提供 `pay_db`”覆盖，并新增“不提供 `common_db`”覆盖。
- [x] 更新 `user-service/internal/providers/ent_test.go`，删除 `common_db` close error 断言。
- [x] 更新组合 lifecycle 测试，确认 PostgreSQL pool 和 Ent client 组合后只关闭 user SQL pool 一次。
- [x] 修复 `user-service/internal/bootstrap/validation_test.go` 中 SQL driver `Open(dsn string)` 的 unused parameter，改为 `Open(_ string)` 或真实断言 DSN。
- [x] 更新 `user-service/internal/bootstrap/validation_test.go` 的测试配置，删除 `resources.NameCommonDB`。
- [x] 更新 `user-service/tests/e2e/harness_test.go`，配置模板只输出 `postgres.user_db`，并同步删除多余格式化参数。
- [x] 更新 `user-service/configs/config.yaml`，删除 `postgres.common_db` 配置和“公共业务库”注释。
- [x] 检查 `user-service/internal/features/*/infrastructure/postgres`，确认所有业务 adapter 仍只注入 `name:"user_db"`。
- [x] 运行 `gofmt -w` 格式化改动的 Go 文件。

## CI

- [x] 新增或调整 GitHub Actions workflow，加入 `make test` job。
- [x] 新增或调整 GitHub Actions workflow，加入 `make build` job。
- [x] 新增或调整 GitHub Actions workflow，加入 Docker image build job：`docker build -f deployments/docker/user-service.Dockerfile -t aegiscore-user-services .`。
- [x] 新增或调整 GitHub Actions workflow，加入分模块 `go test -race ./...` job。
- [x] 新增或调整 GitHub Actions workflow，加入分模块 coverage profile 生成，并上传 coverage artifact。
- [x] 新增或调整 GitHub Actions workflow，加入 `govulncheck`，分别扫描 `common` 和 `user-service`。
- [x] 新增或调整 GitHub Actions workflow，加入 `gosec`，输出 SARIF 或报告 artifact，并明确失败阈值。
- [x] 新增或调整 GitHub Actions workflow，加入 Trivy filesystem scan。
- [x] 新增或调整 GitHub Actions workflow，加入 Docker image Trivy scan。
- [x] 新增或调整 GitHub Actions workflow，生成 repository 和 image SBOM artifact，使用 CycloneDX 或 SPDX 格式。
- [x] 确认新增 workflow 使用 `actions/setup-go@v5`，Go version 与 `go.work`/`go.mod` 工具链一致。
- [x] 确认 workflow permissions 最小化；如上传 SARIF，只在对应 job 声明 `security-events: write`。

## Documentation

- [x] 更新 `AGENTS.md` Repository Shape 和 Current Feature Areas，说明用户服务当前只声明 `user_db` PostgreSQL。
- [x] 更新 `AGENTS.md` Development Commands 或 Repository Rules，如新增 CI 命令说明，保持与 Makefile 一致。
- [x] 更新 `docs/ARCHITECTURE.md` Infrastructure，删除用户服务连接 `postgres.common_db` 的描述。
- [x] 更新 `docs/ARCHITECTURE.md` Database Migrations，强调迁移只面向用户服务拥有的 `user_db`。
- [x] 更新 `docs/DEVELOPMENT.md` 配置说明，删除“用户服务当前声明 `common_db`”。
- [x] 更新 `docs/DEVELOPMENT.md` migration 说明，删除“不要因为配置中存在 `common_db` 而迁移非目标数据库”的当前配置暗示，可保留泛化的“其他命名实例”说明。
- [x] 更新 `docs/TESTING.md` Runtime/Infrastructure 说明，删除 `common_db` 测试期望。
- [x] 如新增 CI 文档或变更 `docs/GO_LINT_AUTOMATION.md`，确认不把 lint 从硬门禁降级。
- [x] 确认文档没有恢复 OpenSpec/OPSX 工作流。

## Verification

- [x] 运行 `rg -n "NameCommonDB|name:\"common_db\"|CommonDB" common user-service`，确认当前源码不再声明或注入 common DB。
- [x] 运行 `rg -n "postgres\\.common_db|公共业务库|common_db" user-service/configs AGENTS.md docs/ARCHITECTURE.md docs/DEVELOPMENT.md docs/TESTING.md`，确认当前配置和主文档不再描述 common DB。
- [x] 运行 `make lint-user-service`，确认 `validation_test.go` unused parameter 已修复。
- [x] 运行 `make lint`。
- [x] 运行 `make test-common`。
- [x] 运行 `make test-user-service`。
- [x] 运行 `make test`。
- [x] 运行 `make build`。
- [x] 在 Docker 可用时运行 `docker build -f deployments/docker/user-service.Dockerfile -t aegiscore-user-services .`。
- [x] 如本地工具可用，运行 `govulncheck`、`gosec`、`trivy` 或记录由 CI 验证。
- [x] 检查 `.github/workflows`，确认 lint 和 migrations 仍存在，新门禁覆盖 test/build/image/race/coverage/security/SBOM。
- [x] 检查 `git diff`，确认没有 Ent generated code、migration、Swagger、`openspec/` 或 `docs/opsx/` 的无关改动。

## Review Notes

- [x] 确认删除的是用户服务预留共享 DB 通道，不是删除 `common/runtime/datastore` 的 named pool primitive。
- [x] 确认没有为了公共能力新增共享 DB、共享 Ent client、outbox、eventbus、broker 或跨服务 transaction wrapper。
- [x] 确认所有 feature PostgreSQL adapter 仍通过消费侧 application port 访问 `user_db` Ent client。
- [x] 确认配置样例中保留的任何非 user DB 命名实例都不会被用户服务 provider 自动连接。
- [x] 确认新增 CI job 的失败语义清晰：测试、构建、镜像、安全扫描失败应阻断；coverage 第一阶段只上传 artifact，不因覆盖率低阻断。
- [x] 确认日志、错误字符串、配置 key、数据库字段和 Redis key 没有因文档修正被无关改动。
