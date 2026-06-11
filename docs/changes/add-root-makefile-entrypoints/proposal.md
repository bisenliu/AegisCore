# Add root Makefile entrypoints

## What

新增仓库根目录 `Makefile`，提供统一的开发入口，覆盖常用 build、test、lint、generate、migrate 和 Swagger 操作。

包括：

- 新增根目录 `Makefile`，通过 `make help` 展示可用命令。
- 支持 `common` 与 `user-service` 两个 Go module 的测试与 lint。
- 支持用户服务构建、运行、生成 Ent 代码、生成/校验/执行 Atlas migration、生成 Swagger 文档。
- 更新 `docs/DEVELOPMENT.md` 与 `AGENTS.md` 中的开发命令，优先展示根目录 Makefile 入口，并保留底层命令语义说明。

本变更只新增统一入口和文档，不改变现有脚本、模块结构、业务逻辑、HTTP API、数据库 schema 或迁移行为。

## Why

当前开发命令分散在 `common/`、`user-service/` 和 `user-service/scripts/` 下。开发者需要记住不同执行目录和参数，尤其是跨模块测试、lint、Ent 生成、迁移和 Swagger 生成时容易出现目录错误。

根目录 Makefile 可以把常用操作收敛到稳定入口，让新协作者先从仓库根目录工作，同时仍然委托给现有 Go 命令与用户服务脚本，避免复制或改变底层行为。

## Scope

包括：

- 新增根目录 `Makefile`。
- `make help` 展示命令和简短说明。
- `make build` 构建当前服务二进制，默认输出到 `bin/user-service`。
- `make test` 等价于分别在 `common/` 和 `user-service/` 执行 `go test ./...`。
- 提供单模块测试入口，例如 `test-common` 与 `test-user-service`。
- `make lint` 分别在 `common/` 和 `user-service/` 执行 `golangci-lint run ./...`。
- 提供单模块 lint 入口，例如 `lint-common` 与 `lint-user-service`。
- `make run-user-service` 从仓库根目录运行用户服务，默认使用 `./user-service/configs/config.yaml`。
- `make generate` 触发用户服务 Ent 代码生成。
- `make migrate-diff name=<migration-name>` 委托 `user-service/scripts/migrate-diff.sh`。
- `make migrate-validate` 委托 `user-service/scripts/migrate-validate.sh`。
- `make migrate-apply` 委托 `user-service/scripts/migrate-apply.sh`，继续依赖调用方设置 `DATABASE_URL`。
- `make swagger-generate` 委托 `user-service/scripts/swagger-generate.sh`。
- 更新开发文档和代理入口规则中的命令说明。

不包括：

- 不改变 `user-service/scripts/` 下任何脚本的行为。
- 不删除或重命名已有脚本。
- 不新增 OpenSpec/OPSX 工件。
- 不改变 Go module path、feature 分层、运行时配置、HTTP API、Ent schema 或 Atlas migration 内容。
- 不把 Makefile 作为 CI 唯一入口；CI 可继续使用现有底层命令。

## Acceptance Criteria

- 根目录存在 `Makefile`。
- `make help` 能展示可用命令和简短说明。
- `make build` 能构建用户服务二进制。
- `make test` 等价于分别在 `common/` 和 `user-service/` 执行 `go test ./...`。
- `make lint` 分别在 `common/` 和 `user-service/` 执行 `golangci-lint run ./...`。
- `make generate` 在 `user-service/` 执行 `go generate ./ent`。
- `make migrate-diff name=<migration-name>` 调用现有 `user-service/scripts/migrate-diff.sh <migration-name>`。
- `make migrate-validate` 调用现有 `user-service/scripts/migrate-validate.sh`。
- `make migrate-apply` 调用现有 `user-service/scripts/migrate-apply.sh`，且 `DATABASE_URL` 缺失时仍由脚本失败。
- `make swagger-generate` 调用现有 `user-service/scripts/swagger-generate.sh`。
- `docs/DEVELOPMENT.md` 和 `AGENTS.md` 中的开发命令已同步为根目录 Makefile 优先入口。
