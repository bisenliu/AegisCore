# Tasks

## Implementation

- [x] 新增根目录 `Makefile`。
- [x] 在 Makefile 中定义 `COMMON_DIR`、`USER_SERVICE_DIR`、可覆盖的 `USER_SERVICE_CONFIG` 和 `USER_SERVICE_BIN` 变量。
- [x] 新增 `help` target，展示命令和简短说明。
- [x] 新增 `build-user-service`，从根目录构建用户服务二进制。
- [x] 新增 `build`，顺序执行当前服务构建入口。
- [x] 新增 `test-common`，在 `common/` 执行 `go test ./...`。
- [x] 新增 `test-user-service`，在 `user-service/` 执行 `go test ./...`。
- [x] 新增 `test`，顺序执行 `test-common` 和 `test-user-service`。
- [x] 新增 `lint-common`，在 `common/` 执行 `golangci-lint run ./...`。
- [x] 新增 `lint-user-service`，在 `user-service/` 执行 `golangci-lint run ./...`。
- [x] 新增 `lint`，顺序执行 `lint-common` 和 `lint-user-service`。
- [x] 新增 `run-user-service`，从根目录运行 `go run ./user-service/cmd serve --config $(USER_SERVICE_CONFIG)`。
- [x] 新增 `generate`，在 `user-service/` 执行 `go generate ./ent`。
- [x] 新增 `migrate-diff`，要求 `name=<migration-name>` 并委托 `user-service/scripts/migrate-diff.sh`。
- [x] 新增 `migrate-validate`，委托 `user-service/scripts/migrate-validate.sh`。
- [x] 新增 `migrate-apply`，委托 `user-service/scripts/migrate-apply.sh`，不在 Makefile 中改写 `DATABASE_URL`。
- [x] 新增 `swagger-generate`，委托 `user-service/scripts/swagger-generate.sh`。
- [x] 更新 `docs/DEVELOPMENT.md` 的常用命令表，优先展示 Makefile 入口并保留底层执行语义。
- [x] 更新 `AGENTS.md` 的 Development Commands，保持与 `docs/DEVELOPMENT.md` 一致。

## Verification

- [x] 运行 `make help`，确认展示命令列表。
- [x] 运行 `make build`，确认用户服务二进制可构建。
- [x] 运行 `make test`，确认等价于分别测试 `common` 和 `user-service`。
- [x] 运行 `make test-common`。
- [x] 运行 `make test-user-service`。
- [x] 在本地安装 `golangci-lint` 时运行 `make lint`；如工具缺失，记录未运行原因。
- [x] 运行 `make migrate-diff`，确认缺少 `name` 时输出用法并失败。
- [x] 如本地 Atlas 可用，运行 `make migrate-validate`。
- [x] 如本地 Swagger 生成依赖可用，运行 `make swagger-generate`。
- [x] 检查 `git diff`，确认没有修改 `user-service/scripts/` 下已有脚本行为。

`make lint` 已运行，但当前仓库存在本变更未触及的存量 lint 问题：`common/` 中 gofmt/goimports/govet/revive/staticcheck 报错，`user-service/` 中 goimports/revive 报错。Makefile lint target 本身能正确进入模块并调用 `golangci-lint run ./...`。

## Review Notes

- [x] 确认没有新增 `openspec/` 或 `docs/opsx/`。
- [x] 确认 Makefile 只做命令调度，不复制迁移或 Swagger 脚本逻辑。
- [x] 确认 `make test` 的失败行为能反映任一模块测试失败。
- [x] 确认文档没有暗示底层脚本被移除或废弃。
