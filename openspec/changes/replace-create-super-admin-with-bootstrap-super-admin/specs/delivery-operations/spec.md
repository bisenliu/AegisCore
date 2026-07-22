## MODIFIED Requirements

### Requirement: 构建、运行与稳定 CLI 入口

系统 MUST 通过统一 Makefile 和 user-service CLI 提供可重复的构建、运行及进程生命周期控制。公开 CLI 命令、flag、退出码和错误传播属于运维契约，变更时 MUST 通过对应 capability 明确迁移。user-service 的唯一公开 CLI 根命令 MUST 为 `aegiscore-user-service`，旧 `aegiscore-user-services` 命令名 MUST NOT 作为别名、隐藏命令或兼容入口保留。

#### Scenario: 构建和运行 user-service

- **WHEN** 协作者执行 `make build`
- **THEN** 系统 MUST 将 user-service 二进制构建到 `USER_SERVICE_BIN`
- **WHEN** 执行 `make user-service-run`
- **THEN** 系统 MUST 使用 `USER_SERVICE_CONFIG` 启动 `aegiscore-user-service serve`

#### Scenario: 命令帮助和稳定 surface

- **WHEN** 协作者执行根或 user-service help
- **THEN** 系统 MUST 输出可用命令及中文说明
- **AND** `serve`、`rbac`、`fxgraph` 的名称、公开 flag、默认配置路径、退出码和输出语义 MUST 保持稳定
- **AND** RBAC help MUST 只展示 `seed` 和 `bootstrap-super-admin` 作为公开 RBAC 运维命令
- **AND** help、测试和文档 MUST 只展示 `aegiscore-user-service` 作为 user-service 根命令
- **AND** help、测试、文档和 Makefile MUST NOT 展示 `rbac create-super-admin`、`rbac assign-super-admin`、`--reset-password`、`ADMIN_RESET_PASSWORD` 或旧超级管理员命令别名

#### Scenario: bootstrap-super-admin Makefile 入口

- **WHEN** 运维执行 `ADMIN_BOOTSTRAP_PASSWORD='<temporary-password>' ADMIN_USERNAME='initial-admin' ADMIN_NICKNAME='Initial Administrator' make user-service-bootstrap-super-admin`
- **THEN** Makefile MUST 调用 `aegiscore-user-service rbac bootstrap-super-admin --username "$ADMIN_USERNAME" --nickname "$ADMIN_NICKNAME" --password-env ADMIN_BOOTSTRAP_PASSWORD`
- **AND** 根 Makefile MUST 提供带服务名前缀的 `user-service-bootstrap-super-admin` 目标
- **AND** 根 Makefile MUST NOT 提供无服务名前缀的 `bootstrap-super-admin` 便利目标
- **AND** Makefile MUST NOT 在命令行中展开、打印或记录密码值
- **AND** 系统 MUST 删除 `user-service-create-super-admin`、`create-super-admin` 和 `ADMIN_RESET_PASSWORD` 入口

#### Scenario: 外部与内部退出协调

- **WHEN** 上游 context 取消或 `App.Wait()` 返回 shutdown signal
- **THEN** serve 命令 MUST 使用未被取消的上游 context value 和配置化预算调用且仅调用一次 `App.Stop()`
- **AND** 非零内部 exit code 或 Stop error MUST 转换为保留全部诊断信息的 Cobra error
- **AND** 命令内部 MUST NOT 调用 `os.Exit`

### Requirement: 受控发布顺序与优雅终止

生产发布 MUST 先确认 SQL migration 受控完成，再执行 RBAC seed 和一次性超级管理员 bootstrap，最后滚动 HTTP 副本。Kubernetes 与 Helm 的终止宽限期 MUST 一致，并覆盖 Fx Stop 总预算及平台安全余量。

#### Scenario: 生产发布顺序

- **WHEN** 发布全新数据库上的 user-service
- **THEN** 运维 MUST 先确认已提交 SQL migration 经 DBA 工单或受控平台执行，再运行 RBAC seed，随后运行 `rbac bootstrap-super-admin`，最后启动或滚动 HTTP 副本
- **AND** 初始管理员 MUST 在 HTTP 副本启动后通过强制改密流程把临时密码改为正式密码
- **AND** 任一前置确认、seed 或 bootstrap 失败 MUST 阻止 rollout
- **AND** seed Job 和 bootstrap Job MUST 使用当前发布镜像执行对应 RBAC CLI，并与 HTTP Deployment 使用同一 release 工件集合

#### Scenario: 不支持旧库升级和旧入口兼容

- **WHEN** 发布包含 `rbac bootstrap-super-admin` 的版本
- **THEN** 系统 MUST 只支持全新数据库路径
- **AND** 系统 MUST NOT 支持旧数据库原地升级、旧超级管理员数据识别、bootstrap marker 回填、旧命令别名、双版本 CLI 共存或自动恢复已有管理员
- **AND** 本次变更 MUST NOT 新增 Ent schema、业务表或 Atlas migration

#### Scenario: 普通容器不迁移

- **WHEN** 普通运行时容器启动
- **THEN** 容器 MUST 直接执行服务或显式 user-service CLI 命令
- **AND** `RUN_MIGRATIONS=true` MUST NOT 触发 Atlas migration

#### Scenario: 当前配置与终止预算关系

- **WHEN** 交付 user-service 配置
- **THEN** server MUST 使用 `server.http` 与默认禁用的 `server.grpc`，资源 MUST 使用 `resources.redis` 和 `resources.postgres`
- **AND** user-service 主 PostgreSQL 连接配置路径 MUST 使用 `resources.postgres.primary_db`
- **AND** 环境变量 MUST 使用当前嵌套路径，进程时区 MUST 使用 `TZ`，secret MUST 通过环境变量或 Secret 注入
- **WHEN** 默认 lifecycle timeout、preStop、原生清单或 Helm values 变化
- **THEN** 结构化测试 MUST 解析目标配置并验证 termination grace 不小于 `runtime.lifecycle.stop_timeout` 加平台安全余量，且原生 Kubernetes 与 Helm 默认值一致
- **AND** 测试 MUST NOT 依赖正则文本误匹配注释或无关字段

#### Scenario: 发布期间优雅终止

- **WHEN** 滚动发布、缩容、驱逐或故障退出终止 Pod
- **THEN** kubelet MUST 为完整 Fx Stop 链路与平台阶段保留默认宽限期
- **AND** 正常关闭提前完成时 Pod MUST 立即退出
- **AND** 只有宽限期耗尽且进程仍未退出时才能强制终止
