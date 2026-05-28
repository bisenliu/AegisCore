# Development

## 1. Prerequisites

- Go workspace 使用 `go 1.24` 和 `toolchain go1.24.1`，见 `go.work`。
- 本地运行用户服务需要 PostgreSQL 和 Redis。
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
| 格式化 Go 文件 | `gofmt -w <files>` | 任意目录 |

## 4. Configuration

配置加载逻辑位于 `common/config/loader.go`。

- 默认配置文件路径由服务命令传入，`serve` 默认使用 `./configs/config.yaml`。
- 从仓库根目录运行时应显式传入 `./user-services/configs/config.yaml`。
- 环境变量前缀为 `AEGISCORE`。
- 配置 key 中的 `.` 和 `-` 会映射为环境变量中的 `_`。

示例：`AEGISCORE_HTTP_PORT=8081` 可覆盖 `http.port`。

## 5. Coding Conventions

- HTTP 层只处理请求解析、参数校验和响应输出。
- Service 层负责业务编排和 DTO 映射。
- Repository 层负责 Ent/数据库访问和存储错误转换。
- 共享中间件、响应模型、配置和基础设施放在 `common/`。
- Ent 生成代码不要手动编辑；修改 schema 后重新生成。
- Go 文件提交前运行 `gofmt`。

## 6. API Conventions

- 成功响应使用 `common/response.OK` 或 `common/response.Created`。
- 失败响应使用 `common/response.Fail` 或便捷方法 `BadRequest`、`NotFound`。
- 响应信封字段为 `success`、`code`、`message`、`data`。
- API 错误码目前包括 `OK`、`BAD_REQUEST`、`NOT_FOUND`、`INTERNAL_ERROR`。

## 7. Adding Features

1. 在 `docs/opsx/CAPABILITY_MAP.md` 中定位或新增 capability。
2. 如新增长期能力，先添加 `openspec/specs/<capability>/spec.md`。
3. 使用 `/opsx:propose <change-name>` 生成 change artifacts。
4. 使用 `/opsx:apply <change-name>` 实现。
5. 增加或更新测试，并在受影响模块目录运行相关 `go test` 命令；跨模块变更时分别在 `common/` 和 `user-services/` 运行。

## 8. Local Runtime Notes

用户服务启动时会 ping Redis 和 PostgreSQL。若本地没有外部依赖，启动会失败。开发纯业务逻辑时优先通过单元测试覆盖 service/repository 边界，集成验证再连接真实依赖。
