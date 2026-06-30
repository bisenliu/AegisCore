## MODIFIED Requirements

### Requirement: 发布和部署资产

系统 MUST 维护 Docker、Compose、Kubernetes、Helm 和观测部署资产，并明确生产发布中 migration 与 RBAC seed 的顺序。user-service 普通运行时镜像 MUST NOT 包含 Atlas 二进制；数据库 migration MUST 由独立 Atlas/migration 镜像在发布前置 Job 或 CI/CD release job 中执行。

#### Scenario: 构建 Docker 镜像

- **WHEN** 协作者执行 Docker build 命令并指定 `deployments/docker/user-service.Dockerfile`
- **THEN** 系统 MUST 能从仓库根目录构建 user-service 运行时镜像
- **AND** 运行时镜像 MUST NOT 包含 `/usr/local/bin/atlas` 或其他 Atlas CLI 二进制

#### Scenario: Dockerfile 路径约束

- **WHEN** 调整 user-service Dockerfile、entrypoint、migration Dockerfile 或 COPY 规则
- **THEN** 路径 MUST 继续以仓库根 build context 为基准
- **AND** 专用 migration 镜像 MUST 能访问 `user-service/migrations/` 当前布局和 `migrations/atlas.hcl`
- **AND** user-service 运行时镜像 MUST NOT 为了执行 migration 而复制 `user-service/migrations/` 或 Atlas 二进制

#### Scenario: 本地 Compose 启动

- **WHEN** 协作者使用 `deployments/compose` 运行本地环境
- **THEN** 系统 MUST 提供 user-service 所需的数据库、缓存和观测服务配置

#### Scenario: Compose 启动顺序

- **WHEN** 使用本地 Compose 启动包含 migration、RBAC seed 和 user-service 的环境
- **THEN** migration MUST 通过独立 Atlas/migration 镜像先于 RBAC seed 执行，RBAC seed MUST 先于 user-service app 启动

#### Scenario: 生产发布顺序

- **WHEN** user-service 发布到生产环境
- **THEN** 运维 MUST 先通过独立 Atlas/migration 镜像或 CI/CD release job 执行 user-service `user_db` Atlas migration，再执行 RBAC seed Job，按需显式创建或分配超级管理员，最后启动或滚动更新 HTTP 副本

#### Scenario: 普通容器启动不执行 migration

- **WHEN** user-service 普通运行时容器启动
- **THEN** 容器 MUST 直接启动服务或执行显式传入的 user-service CLI 命令，不得应用 migration
- **AND** `RUN_MIGRATIONS=true` MUST NOT 使普通运行时镜像尝试执行 Atlas migration

#### Scenario: 显式执行 migration

- **WHEN** 简单部署或发布流程需要在启动 HTTP 服务前应用 migration
- **THEN** 部署流程 MUST 先运行独立 Atlas/migration 镜像完成 `atlas migrate apply`
- **AND** 部署流程 MUST 在 migration 成功后再启动 user-service 运行时镜像

### Requirement: Kubernetes 发布前置作业

系统 MUST 为 user-service Kubernetes 发布提供独立 migration Job 和 RBAC seed Job，并明确它们先于 HTTP Deployment rollout 执行。migration Job MUST 使用独立 Atlas/migration 镜像；RBAC seed Job MUST 使用当前 user-service 发布镜像。

#### Scenario: Migration Job

- **WHEN** 发布流水线执行 user-service 数据库迁移
- **THEN** migration Job MUST 使用独立 Atlas/migration 镜像执行已提交的 `user-service/migrations/` SQL migration
- **AND** migration Job MUST 通过 Secret 或部署系统注入 `DATABASE_URL`
- **AND** migration Job MUST NOT 使用 user-service HTTP 运行时镜像执行 `/app/user-service/scripts/migrate-apply.sh`

#### Scenario: RBAC seed Job

- **WHEN** migration Job 成功完成后
- **THEN** RBAC seed Job MUST 使用当前 user-service 发布镜像执行 `rbac seed`，并 MUST 支持 `--reactivate-system` 与 `--sync-system-bindings` 选项

#### Scenario: 发布顺序

- **WHEN** user-service 发布到 Kubernetes 环境
- **THEN** 发布流程 MUST 等待 migration Job 成功，再等待 RBAC seed Job 成功，最后创建或滚动更新 HTTP Deployment

#### Scenario: 前置作业失败

- **WHEN** migration Job 或 RBAC seed Job 失败
- **THEN** 发布流程 MUST 停止 HTTP Deployment rollout，并保留 Job 日志用于诊断

#### Scenario: 镜像版本一致性

- **WHEN** 发布系统设置 user-service 镜像和 migration 镜像
- **THEN** migration 镜像 MUST 与 user-service 发布版本来自同一 release 工件集合或使用同一 release tag
- **AND** 发布说明 MUST 禁止混用新版 user-service 运行时镜像和旧版 migration Job 模板

### Requirement: 用户服务 Helm chart

系统 MUST 为 `aegiscore-user-services` 提供 Helm chart，用 values 模板化同等 Kubernetes 交付能力，并保持默认值不包含真实生产 Secret。chart MUST 支持为 migration Job 配置独立 Atlas/migration 镜像，同时保持 RBAC seed Job 和 HTTP Deployment 使用 user-service 镜像。

#### Scenario: Helm chart 元数据和 values

- **WHEN** 协作者查看 `deployments/helm/aegiscore-user-services/`
- **THEN** chart MUST 包含 `Chart.yaml`、`values.yaml`、templates、README 和环境覆盖示例，并 MUST 暴露 image、migration image、service、config、Secret 引用、resources、probes、autoscaling、PDB、NetworkPolicy、migration Job、RBAC seed Job 和 rollout 配置

#### Scenario: Helm 渲染 Secret 引用

- **WHEN** 协作者执行 `helm template` 渲染 chart
- **THEN** 模板 MUST 通过 `existingSecret` 或等价 values 引用外部 Secret，不得默认渲染真实敏感值

#### Scenario: Helm 渲染发布作业

- **WHEN** Helm values 启用 migration Job 和 RBAC seed Job
- **THEN** chart MUST 渲染两个独立 Job
- **AND** migration Job MUST 使用独立 Atlas/migration 镜像执行 migration command
- **AND** RBAC seed Job MUST 使用 user-service 发布镜像执行 `rbac seed`
- **AND** 两个 Job MUST 保持 command 与 Kubernetes 原生清单的职责边界一致

#### Scenario: Helm Deployment 默认行为

- **WHEN** Helm chart 渲染 user-service Deployment
- **THEN** Deployment MUST 默认不设置 `RUN_MIGRATIONS=true`，并 MUST 渲染 `/livez`、`/readyz`、`/startupz` 探针和资源 requests/limits
- **AND** Deployment 使用的 user-service 镜像 MUST NOT 依赖 Atlas 二进制或 migration SQL 文件启动 HTTP 服务
