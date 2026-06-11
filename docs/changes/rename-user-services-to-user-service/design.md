# Design

## Overview

本变更是一次命名一致性迁移。核心做法是先完成物理目录重命名，再全量替换 Go module path 和路径引用，最后用 Go 测试与文本扫描验证没有遗漏。

为避免引入行为变化，迁移过程中只处理路径和 module 名称：

- 文件系统路径：`user-services` -> `user-service`
- Go module/import path：旧复数 module -> `github.com/aegiscore/user-service`

以下运行时标识保持不变：

- CLI 名称 `aegiscore-user-services`
- 配置中的 `app.name`
- JWT issuer
- 日志文件名基准
- Redis key prefix
- HTTP API path
- PostgreSQL schema 和 migration 内容

## Implementation Approach

1. 重命名目录

   使用 `git mv user-services user-service` 保留 Git rename 语义。目录内分层保持不变，包括 `cmd/`、`internal/features/*`、`ent/`、`migrations/`、`configs/`、`scripts/` 和 `docs/`。

2. 更新 workspace 和 module

   更新根目录 `go.work`，将旧复数目录改为 `./user-service`。更新用户服务模块 `go.mod`，将 module path 改为 `github.com/aegiscore/user-service`。

3. 更新 Go import

   将所有 Go 文件中的旧复数 module import 更新为 `github.com/aegiscore/user-service/...`。Ent 生成代码中包含旧 module path 的 import 和注释也需要同步更新。迁移后运行 `gofmt`，必要时运行 `go generate ./ent` 让生成产物保持一致。

4. 更新构建、脚本和部署引用

   更新 Dockerfile、entrypoint 注释、迁移脚本注释、Swagger 生成脚本、CI workflow、lint 文档、开发文档和测试文档中的文件路径。容器内路径可一并从 `/app/user-services` 改为 `/app/user-service`，但服务二进制名、app name 和外部配置语义保持不变。

5. 同步 Swagger 产物

   Swagger definition key 由 Go package path 派生，module path 变更后需要同步 `user-service/docs/docs.go`、`swagger.json` 和 `swagger.yaml` 中的旧复数 module 派生标识。优先使用现有 `scripts/swagger-generate.sh` 重新生成；如果本地缺少生成工具，再进行机械替换并在任务记录中说明。

6. 更新长期规则文档

   同步 `AGENTS.md` 和 `docs/ARCHITECTURE.md`，确保仓库结构规则、关键入口、开发命令和禁止事项都指向 `user-service`。

## Compatibility

本变更不应影响对外协议和运行时数据：

- HTTP route 仍位于 `/api/v1/...`。
- 响应 envelope 不变。
- 配置 key 和 `AEGISCORE_` 环境变量映射不变。
- PostgreSQL migration 文件不新增、不删除、不重写 schema 语义。
- Redis key prefix 继续来自配置中的 app name。

需要注意的是，Go import path 是编译期契约，所有本仓库内引用必须同步迁移。仓库外如果有代码直接 import 旧复数 module，需要跟随改为 `github.com/aegiscore/user-service/...`。

## Verification Strategy

- 文本扫描：
  - 扫描旧复数 module path 与旧复数目录引用
  - 根据结果判断是否仍有需要迁移的旧引用。
- Workspace 验证：
  - `go work sync` 或 `go list` 能识别 `common` 与 `user-service` 两个模块。
- 测试：
  - 在 `common/` 执行 `go test ./...`。
  - 在 `user-service/` 执行 `go test ./...`。
- 生成产物检查：
  - 如运行 Swagger 或 Ent 生成命令，检查生成文件 diff 仅包含路径/module 命名变化。
