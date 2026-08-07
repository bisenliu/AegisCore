## MODIFIED Requirements

### Requirement: 镜像、部署与受控发布

user-service 镜像 MUST 使用 BuildKit、不可变基础镜像、只读 Go module 解析和分离缓存构建，并以最小非 root 运行时交付同一可验证工件。CI MUST 运行覆盖 common 与 user-service 的真实依赖测试。Docker、Compose、Kubernetes、Helm 和观测资产 MUST 使用一致的 runtime config、release 工件、安全上下文、探针、Secret、资源和网络边界，MUST NOT 包含真实 secret 或自动 migration apply。外部服务标识与镜像名 MUST 使用 `aegiscore-user-service`，容器、二进制和本地目录语义 MUST 使用 `user-service`，MUST NOT 保留旧复数命名。

#### Scenario: 发布顺序与兼容边界

- **WHEN** 发布或滚动 user-service
- **THEN** 受控发布阶段 MUST 先机器校验本 release 对应 SQL migration 已确认执行，再使用同一不可变镜像 digest 执行 release 唯一 RBAC seed Job 并等待成功，最后应用不包含 seed Job 的 runtime manifest 并等待 Deployment rollout
- **AND** migration 确认缺失、确认不匹配或 RBAC seed Job 失败时，发布流水线 MUST 在应用新版 Deployment 前失败，MUST NOT 更新 HTTP 工作负载 revision
- **AND** release artifact MUST 分别记录同一镜像 digest、seed manifest 和最终 runtime manifest；最终 runtime manifest MUST NOT 包含 RBAC seed Job
- **WHEN** 在全新数据库发布 user-service
- **THEN** 运维 MUST 先确认已提交 migration 经 DBA 工单或受控平台执行，再运行 RBAC seed、`rbac bootstrap-super-admin`，最后启动或滚动 HTTP 副本；任一前置步骤失败 MUST 阻止 rollout
- **AND** 初始管理员 MUST 在 HTTP 副本启动后通过强制改密设置正式密码，seed 与 bootstrap Job MUST 使用和 Deployment 相同的当前发布镜像与工件集合
- **WHEN** 发布包含 `rbac bootstrap-super-admin` 的版本
- **THEN** 系统 MUST 只支持全新数据库路径，MUST NOT 支持旧库原地升级、旧管理员识别、bootstrap marker 回填、旧命令别名、双版本 CLI 或自动恢复已有管理员，也 MUST NOT 新增 Ent schema、业务表或 Atlas migration
- **WHEN** 普通运行时容器启动
- **THEN** 容器 MUST 直接执行服务或显式 CLI，`RUN_MIGRATIONS=true` MUST NOT 触发 Atlas migration
