# AegisCore Agent Guide

本文件为 AI 代理提供仓库导航。修改代码前先确认改动所属 capability，并优先阅读对应文档与主规格。

## 1. Quick Start

- 架构文档：`docs/ARCHITECTURE.md`
- 开发说明：`docs/DEVELOPMENT.md`
- 产品上下文：`docs/PRODUCT.md`
- 测试说明：`docs/TESTING.md`
- 能力地图：`docs/opsx/CAPABILITY_MAP.md`
- OPSX 工作流：`docs/opsx/CHANGE_WORKFLOW.md`

## 2. Repository Shape

- `go.work`：Go workspace，包含 `common` 和 `user-services` 两个模块。
- `common/`：共享配置、基础设施、Gin 中间件、响应信封和错误模型。
- `user-services/`：用户服务 HTTP 运行时，包含 Cobra 入口、Fx 组装、Gin 路由、用户 controller/service/repository、Ent schema、Atlas 配置、服务内 migration 和生成代码。
- `openspec/`：OPSX/OpenSpec 配置、主规格和后续 change artifacts。

## 3. Key Entry Points

- CLI 入口：`user-services/cmd/main.go`
- 服务组装：`user-services/internal/bootstrap/bootstrap.go`
- HTTP 路由：`user-services/internal/router/router.go`
- 用户查询控制器：`user-services/internal/controller/user_controller.go`
- 用户查询服务：`user-services/internal/service/user_service.go`
- 用户数据访问：`user-services/internal/repository/user_repository.go`
- 共享配置加载：`common/config/loader.go`
- 共享基础设施 Fx 模块：`common/infrastructure/module.go`
- Atlas 迁移配置：`user-services/atlas.hcl`
- 用户服务迁移目录：`user-services/migrations/`
- 迁移脚本：`user-services/scripts/migrate-diff.sh`、`user-services/scripts/migrate-validate.sh`、`user-services/scripts/migrate-apply.sh`

## 4. Core Capabilities

- `user-profile-query`：通过 `GET /api/v1/users/:id` 查询用户资料。
- `http-service-runtime`：启动、运行和优雅停止用户服务 HTTP server。
- `shared-infrastructure`：加载配置，提供 Zap 日志，并支持服务侧声明具名 Redis/PostgreSQL/Ent 运行时依赖。
- `api-response-contract`：统一成功/失败响应信封和应用错误映射。
- `database-schema-migrations`：通过 Ent schema 和 Atlas 维护用户服务 SQL migration。
- `go-toolchain-baseline`：统一 `go.work`、`common/go.mod` 和 `user-services/go.mod` 的 Go 1.26 工具链基线。

详见 `docs/opsx/CAPABILITY_MAP.md` 与 `openspec/specs/*/spec.md`。

## 5. Development Commands

- 运行全部测试：分别在 `common/` 和 `user-services/` 执行 `go test ./...`。
- 运行用户服务：`go run ./user-services/cmd serve --config ./user-services/configs/config.yaml`。
- 生成 Ent 代码：在 `user-services` 模块中运行 `go generate ./ent`。
- 生成迁移：在 `user-services/` 执行 `./scripts/migrate-diff.sh <name>`。
- 校验迁移：在 `user-services/` 执行 `./scripts/migrate-validate.sh`。
- 格式化 Go 代码：`gofmt -w <files>`。

## 6. Change Workflow

1. 先阅读 `docs/opsx/CAPABILITY_MAP.md`，定位相关 capability。
2. 如需求不清，先用 `/opsx:explore` 澄清问题和方案。
3. 用 `/opsx:propose <change-name>` 创建 proposal、design、tasks。
4. 准备实现时使用 `/opsx:apply <change-name>`。
5. 实现后验证测试，再用 `/opsx:archive <change-name>` 归档已完成 change。

## 7. Repository Rules

- 不要手写 `user-services/ent/` 下的生成代码；修改 Ent schema 后重新生成。
- 不要用运行时 `client.Schema.Create(ctx)` 表达 schema 变更；修改 Ent schema 后生成 Ent 代码和 Atlas SQL migration。
- 保持 controller/service/repository 分层：HTTP 解析在 controller，业务编排在 service，数据库访问在 repository。
- 共享基础能力优先放在 `common/`，避免在服务模块中重复实现中间件、响应信封或基础设施初始化。
- HTTP API 应使用 `common/response.Envelope` 格式返回。
- 配置通过 YAML 与 `AEGISCORE_` 环境变量覆盖加载，Redis/PostgreSQL 使用 `redis.<name>` 与 `postgres.<name>` 命名实例，避免硬编码运行时配置。
