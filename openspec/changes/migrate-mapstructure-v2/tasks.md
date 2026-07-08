## 1. 迁移范围确认

- [x] 1.1 使用 `rg "github.com/(mitchellh|go-viper)/mapstructure|mapstructure\."` 搜索仓库内所有 Go 源码、测试、文档和 module 文件中的 `mapstructure` 使用点。
- [x] 1.2 确认迁移仅影响 `common/runtime/config`、Go module 依赖、相关测试和必要文档，不涉及 HTTP API、数据库 schema、OpenAPI、部署清单或观测资产。

## 2. 代码与依赖迁移

- [x] 2.1 将所有 Go 源码中的 `mapstructure` 导入统一改为 `github.com/go-viper/mapstructure/v2`。
- [x] 2.2 按 v2 API 和标准行为调整 `common/runtime/config` 的 decode hook 组合、Viper `Unmarshal` 调用和错误处理，不新增旧版兼容层、fallback、alias 或构建标签。
- [x] 2.3 在 `common` 目录运行 `GOWORK=off go mod tidy`，确保旧版 `github.com/mitchellh/mapstructure` 不再作为当前模块未使用依赖残留。
- [x] 2.4 在 `user-service` 目录运行 `GOWORK=off go mod tidy`，确保服务模块依赖图与迁移后的共享配置 loader 一致。

## 3. 测试与文档

- [x] 3.1 更新或新增 `common/runtime/config` 测试，覆盖配置文件加载、环境变量覆盖、duration、slice、具名 Postgres、具名 Redis、具名 `local_cache` 和 validation 失败路径。
- [x] 3.2 确认测试预期直接采用 `mapstructure/v2` 标准行为，不保留旧版行为断言或兼容测试。
- [x] 3.3 同步更新涉及配置解析依赖、迁移说明或验证命令的相关文档；如无需文档变更，在实施记录中说明原因。

## 4. 验证与收尾

- [x] 4.1 运行 `GOWORK=off go test ./runtime/config` 于 `common` 目录，验证配置加载相关测试通过。
- [x] 4.2 运行 `rg "github.com/mitchellh/mapstructure"`，确认仓库内没有旧版导入或依赖残留。
- [x] 4.3 运行 `openspec status --change "migrate-mapstructure-v2"`，确认 change artifacts 状态正常。
- [x] 4.4 将本次预期代码、测试、依赖、文档和 OpenSpec 变更加到暂存区，避免最终 `make verify` 被未暂存的预期 diff 阻塞。
- [x] 4.5 运行 `make lint` 并修复所有与本变更相关的问题。
- [x] 4.6 运行 `make verify` 并修复所有与本变更相关的问题。
- [x] 4.7 检查 `git diff --cached` 和 `git diff`，确认暂存内容仅包含本次迁移的预期改动且没有生成物 drift。
