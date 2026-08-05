## 1. 扩展仓库级质量与安全门禁

- [x] 1.1 为 `tools/openapi-convert` 和 `tools/nacos-config-seed` 增加 module-local `lint` 目标，并在根 Makefile 增加带工具上下文的 lint 目标后纳入 `make lint`。
- [x] 1.2 将 CI `govulncheck` 和 `gosec` 改为包含四个 module 的 name/path matrix，确保 job、SARIF 与 artifact 名称稳定且报告统一写入仓库根 `security/`。
- [x] 1.3 更新根 golangci 配置说明和 `docs/GO_LINT_AUTOMATION.md`，明确四个 module 共享配置、逐 module 执行方式和本地工具 lint 命令。

## 2. 修复工具输出错误传播

- [x] 2.1 修改两个工具的成功输出写入，stdout writer 失败时输出上下文诊断并返回 `exitError`；失败诊断 writer 失败时显式保持原非零状态。
- [x] 2.2 为两个工具增加失败 writer 测试，断言业务操作完成但成功消息不可写时退出码为非零，并保持原有正常输出和错误路径测试通过。

## 3. 收敛 Go module metadata

- [x] 3.1 在 `tools/openapi-convert/go.mod` 增加到仓库 `common` 的相对 `replace`，明确完整仓库 checkout 内的 `GOWORK=off` module 维护边界。
- [x] 3.2 分别在 `common`、`user-service`、`tools/openapi-convert` 和 `tools/nacos-config-seed` 执行 `GOWORK=off go mod tidy` 并提交所需 manifest 与 checksum 变化。
- [x] 3.3 对四个 module 分别执行 `GOWORK=off go mod tidy -diff`，确认不再尝试解析不可用的远端 `common@v0.0.0` 且没有 metadata drift。

## 4. 定向验证与规格校验

- [x] 4.1 对修改的 Go 文件执行 `gofmt`，运行两个工具 module 的单测与 lint，确认四个 `errcheck` finding 消失且 stdout 写失败回归测试通过。
- [x] 4.2 运行 `openspec validate harden-repository-tool-quality` 和 `make user-service-architecture-lint`，确认 `delivery-operations` delta、中文文档和仓库边界有效。
- [x] 4.3 检查 workflow diff 与 GitHub Actions 表达式，确认 `govulncheck`、`gosec` 恰好覆盖四个 module，且不修改 race、coverage、API、数据库、OpenAPI、部署或观测资产。

## 5. 合并前门禁

- [x] 5.1 使用显式路径暂存本 change 的 OpenSpec artifacts、Makefile、workflow、工具代码与测试、文档和 module metadata，检查 `git status --short`，确认暂存范围只包含预期变更。
- [x] 5.2 在全部预期变更已暂存后运行 `make lint`；仅在命令通过后将本任务标记完成。
- [x] 5.3 在全部预期变更已暂存后运行 `make verify`；仅在测试、生成检查和最终 `git diff --exit-code` 全部通过后将本任务及 change 标记完成。
