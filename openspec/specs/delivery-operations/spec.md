## Purpose

定义 AegisCore 的交付运维能力，覆盖构建、运行、测试、lint、架构检查、代码生成、数据库迁移、部署资产和发布顺序。

## Requirements

### Requirement: 构建与本地运行

系统 MUST 提供统一 Makefile 入口构建和运行 user-service，并支持通过配置文件启动服务。

#### Scenario: 构建全部服务

- **WHEN** 协作者执行 `make build`
- **THEN** 系统 MUST 构建 user-service 二进制到配置的 `USER_SERVICE_BIN`

#### Scenario: 运行 user-service

- **WHEN** 协作者执行 `make user-service-run`
- **THEN** 系统 MUST 使用 `USER_SERVICE_CONFIG` 指定的 YAML 配置运行 `aegiscore-user-services serve`

#### Scenario: 查看命令帮助

- **WHEN** 协作者执行 `make help` 或 `make -C user-service help`
- **THEN** 系统 MUST 输出可用命令及中文说明

### Requirement: 测试、lint 和完整验证

系统 MUST 提供统一测试、lint、架构边界检查和完整 verify 入口。

#### Scenario: 运行全部测试

- **WHEN** 协作者执行 `make test`
- **THEN** 系统 MUST 运行 `common` 和 `user-service` 的 Go 测试

#### Scenario: 运行 lint

- **WHEN** 协作者执行 `make lint`
- **THEN** 系统 MUST 运行各 Go 模块的 `golangci-lint`

#### Scenario: 运行完整验证

- **WHEN** 协作者执行 `make verify`
- **THEN** 系统 MUST 依次执行 lint、user-service 架构边界检查、测试、OpenAPI 生成，并通过 `git diff --exit-code` 暴露生成物 drift

### Requirement: 架构边界检查

系统 MUST 提供 user-service 架构 lint，用于检查 feature 分层依赖、禁止跨层违规引用，并校验 OpenSpec/OPSX Markdown 语言约束。

#### Scenario: 分层引用合法

- **WHEN** feature 内代码遵循 domain、application、infrastructure、transport 的依赖方向
- **THEN** `make user-service-architecture-lint` MUST 通过

#### Scenario: 分层引用违规

- **WHEN** 代码出现违反架构边界的 import 或跨 feature 非法依赖
- **THEN** 架构 lint MUST 失败并输出违规位置

#### Scenario: OPSX 文档残留英文模板

- **WHEN** `openspec/specs/`、`openspec/changes/` 或 `docs/opsx/` 下 Markdown 保留默认英文模板标题或说明
- **THEN** 架构 lint MUST 失败并要求改为简体中文正文

### Requirement: 代码生成与数据库迁移

系统 MUST 提供 Ent 代码生成、Atlas migration diff、migration validate 和 migration apply 入口，并要求 schema 相关变更同步生成物。

#### Scenario: 生成 Ent 代码

- **WHEN** Ent schema 变化后执行 `make user-service-generate`
- **THEN** 系统 MUST 运行 `go generate ./ent` 并更新 Ent 生成代码

#### Scenario: 生成 migration

- **WHEN** 数据库 schema 变化需要生成 migration
- **THEN** 协作者 MUST 执行 `make user-service-migrate-diff name=<migration-name>` 生成 Atlas migration

#### Scenario: 校验并应用 migration

- **WHEN** migration 准备进入环境或发布流程
- **THEN** 系统 MUST 支持 `make user-service-migrate-validate` 校验 migration，并通过 `DATABASE_URL` 执行 `make user-service-migrate-apply`

### Requirement: 发布和部署资产

系统 MUST 维护 Docker、Compose、Kubernetes、Helm 和观测部署资产，并明确生产发布中 migration 与 RBAC seed 的顺序。

#### Scenario: 构建 Docker 镜像

- **WHEN** 协作者执行 Docker build 命令并指定 `deployments/docker/user-service.Dockerfile`
- **THEN** 系统 MUST 能从仓库根目录构建 user-service 镜像

#### Scenario: 本地 Compose 启动

- **WHEN** 协作者使用 `deployments/compose` 运行本地环境
- **THEN** 系统 MUST 提供 user-service 所需的数据库、缓存和观测服务配置

#### Scenario: 生产发布顺序

- **WHEN** user-service 发布到生产环境
- **THEN** 运维 MUST 先执行 user-service `user_db` Atlas migration，再执行 `make user-service-seed-rbac`，按需创建超级管理员，最后启动或滚动更新 HTTP 副本
