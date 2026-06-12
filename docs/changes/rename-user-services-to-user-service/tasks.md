# Tasks

## Implementation

- [x] 使用 `git mv user-services user-service` 重命名用户服务目录。
- [x] 更新 `go.work`，将 workspace module path 改为 `./user-service`。
- [x] 更新 `user-service/go.mod`，将 module path 改为 `github.com/aegiscore/user-service`。
- [x] 全量更新 Go imports：旧复数 module -> `github.com/aegiscore/user-service`。
- [x] 运行 `gofmt` 处理被修改的 Go 文件。
- [x] 更新 `user-service/Dockerfile` 中的 build context、copy path、workdir、容器内路径和命令引用。
- [x] 更新 `user-service/scripts/*.sh` 中的路径引用和注释。
- [x] 更新 `.github/workflows/lint.yml` 中的模块路径、缓存路径和执行目录。
- [x] 更新 `deployments/` 中引用 `user-services` 的部署、Compose、Kubernetes 或 Helm 资产。
- [x] 更新 `AGENTS.md`、`docs/ARCHITECTURE.md`、`docs/DEVELOPMENT.md`、`docs/TESTING.md` 和 `docs/GO_LINT_AUTOMATION.md` 中的目录、命令和 module path。
- [x] 同步 `user-service/docs/docs.go`、`user-service/docs/swagger.json` 和 `user-service/docs/swagger.yaml` 中由 Go package path 派生的 Swagger definition key。
- [x] 检查 Ent 生成代码中的旧 module path import 或注释，并通过重新生成或机械替换保持一致。

## Verification

- [x] 运行旧复数 module path 与旧复数目录引用扫描，确认不再出现需要迁移的旧引用。
- [x] 在 `common/` 执行 `go test ./...`。
- [x] 在 `user-service/` 执行 `go test ./...`。
- [x] 如 Swagger 产物通过生成命令更新，检查生成结果只包含路径/module 命名变化。
- [x] 如 Ent 产物通过 `go generate ./ent` 更新，检查生成结果只包含路径/module 命名变化。

## Review Notes

- [x] 确认外部 HTTP API path 未变化。
- [x] 确认数据库 schema、migration SQL 和 `atlas.sum` 未因本次重命名产生语义变化。
- [x] 确认配置 key、`AEGISCORE_` 环境变量映射、JWT issuer、Redis key prefix 和日志 service name 未变化。
- [x] 确认没有新增横向 `internal/controller`、`internal/service`、`internal/repository`、`internal/api`、`internal/domain` 或 `internal/shared` 包。
