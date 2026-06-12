# Rename user-services to user-service

## What

将用户服务的仓库目录名和 Go module path 从复数形式统一为单数形式：

- 旧的复数用户服务目录重命名为 `user-service/`。
- `go.work` 中的 workspace module 路径从旧复数目录更新为 `./user-service`。
- 用户服务 Go module path 从旧复数 module 更新为 `github.com/aegiscore/user-service`。
- 更新 Go imports、Dockerfile、脚本、部署资产、CI/lint 文档、Swagger 生成产物和项目文档中的旧路径引用。

本变更只调整目录、module path 和路径引用，不改变外部 HTTP API、数据库 schema、migration 内容、配置 key、Redis key、日志 service name 或业务行为。

## Why

当前目录名和 module path 使用 `user-services`，但目标结构希望以单个服务模块 `user-service` 表达用户服务边界。统一命名可以减少后续文档、导入路径、部署路径和开发命令中的歧义，也为未来新增其他服务模块时保持一致的服务命名模式。

## Scope

包括：

- 重命名用户服务目录。
- 更新 Go workspace、用户服务 module path 和所有 Go import。
- 更新 Docker build/copy/workdir/entrypoint 路径。
- 更新迁移、Swagger、lint、测试和开发脚本文档中的文件路径引用。
- 重新生成或同步 Swagger 产物中由 Go package path 派生的 definition key。
- 更新 `AGENTS.md` 与 `docs/ARCHITECTURE.md` 中的结构规则和入口说明。

不包括：

- 不改变 feature 分层或包职责。
- 不将 `app` 改名为 `application`。
- 不将 `infra` 改名为 `infrastructure`。
- 不新增 role、permission、team 或其他业务能力。
- 不修改用户、认证、HTTP API、配置 key、数据库 schema、migration 或运行时业务逻辑。

## Acceptance Criteria

- 旧复数 module path 与旧复数目录引用的仓库扫描不再出现需要迁移的结果。
- `go.work` 指向 `./user-service`。
- `user-service/go.mod` 的 module path 为 `github.com/aegiscore/user-service`。
- `common/` 中 `go test ./...` 通过。
- `user-service/` 中 `go test ./...` 通过。
- 外部 HTTP API 路径、配置 key、数据库 migration 和业务行为保持不变。
