## 1. 工具模块迁移

- [x] 1.1 创建 `tools/openapi-convert/` Go module，并在 `go.work` 中加入 `./tools/openapi-convert`。
- [x] 1.2 将 `user-service/internal/tools/openapi-convert/main.go` 迁移到 `tools/openapi-convert/main.go`，保持 CLI 只依赖标准库和 `common/http/openapi`。
- [x] 1.3 调整 `tools/openapi-convert` 的默认值，移除 user-service 专属的 `/api/v1`、`/livez`、`/readyz`、`/startupz` 和 `BearerAuth` 服务语义默认值。
- [x] 1.4 删除 `user-service/internal/tools/openapi-convert/`，并确认该目录不再存在。

## 2. user-service 生成脚本调整

- [x] 2.1 修改 `user-service/scripts/openapi-generate.sh`，将转换命令改为调用 `../tools/openapi-convert`。
- [x] 2.2 在 `user-service/scripts/openapi-generate.sh` 中显式传入 `-server /api/v1`、`-root-server /`、`-root-path /livez`、`-root-path /readyz`、`-root-path /startupz`、`-bearer-auth-name BearerAuth`、BearerAuth 描述和 `-generated-by scripts/openapi-generate.sh`。
- [x] 2.3 搜索 `internal/tools/openapi-convert` 和 `openapi-convert`，确认所有调用点和文档引用均指向新的仓库级工具边界。

## 3. 验证

- [x] 3.1 在 `tools/openapi-convert` 目录运行 `go test ./...`，确认工具模块可编译。
- [x] 3.2 运行 `make user-service-openapi-generate`，确认 OpenAPI 生成流程可执行。
- [x] 3.3 检查 `user-service/docs/openapi.go`、`user-service/docs/openapi.json` 和 `user-service/docs/openapi.yaml` diff，确认没有非预期生成物 drift。
- [x] 3.4 运行 `make user-service-architecture-lint`，确认目录边界和 OPSX 文档规则通过。

## 4. 收尾

- [x] 4.1 更新必要的仓库文档或 OPSX 能力地图，确保 OpenAPI 转换工具位置描述与实现一致。
- [x] 4.2 将本次预期代码、规格和文档变更加到暂存区。
- [x] 4.3 运行 `make lint`，且失败时修复后重新运行。
- [x] 4.4 运行 `make verify`，且失败时修复后重新运行；最终 `git diff --exit-code` 只能暴露未暂存的意外变更或生成物 drift。
