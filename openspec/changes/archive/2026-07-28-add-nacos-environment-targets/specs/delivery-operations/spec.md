## ADDED Requirements

### Requirement: 独立本地 Nacos 配置目录与双初始化服务

系统 MUST 为主机和 Docker 分别保存完整、可独立查看、校验和发布的 Nacos 配置目录，并通过两个 Compose 一次性服务分别调用单目录、单 Namespace seed 工具发布到独立 Namespace。每个目录 MUST 完整包含固定三 dataId；系统 MUST 接受少量公共字段重复，并通过结构化测试约束除明确环境字段外的配置一致性。

#### Scenario: 两套完整配置目录

- **WHEN** 仓库保存本地主机和 Docker 的 Nacos 配置
- **THEN** `deployments/nacos/local-host/` 和 `deployments/nacos/local-docker/` MUST 各自完整包含 `base.yaml`、`resources.yaml`、`user-service.yaml`
- **AND** 任一目录 MUST 能在不读取另一目录、公共配置目录、overlay、target manifest 或第四文档的情况下独立发布和加载
- **AND** 主机目录 MUST 直接声明宿主机地址与映射端口，Docker 目录 MUST 直接声明 Compose DNS 与容器端口
- **AND** 旧 `deployments/compose/nacos/init/` MUST NOT 继续作为第二权威配置来源

#### Scenario: 两个单目标初始化服务

- **WHEN** Compose 启动本地 Nacos 配置初始化
- **THEN** `nacos-init-host` MUST 只读挂载 `deployments/nacos/local-host/` 并发布到 `loca-host`
- **AND** `nacos-init-docker` MUST 只读挂载 `deployments/nacos/local-docker/` 并发布到 `loca-docker`
- **AND** 两个服务 MUST 使用 `AEGISCORE` group 和 `base.yaml`、`resources.yaml`、`user-service.yaml` 三 dataId
- **AND** Docker user-service 与 RBAC seed MUST 在 `nacos-init-docker` 成功后启动，MUST NOT 依赖主机配置完成发布

#### Scenario: seed 工具保持单目录单 Namespace 契约

- **WHEN** 任一 Nacos 初始化服务调用 `nacos-config-seed`
- **THEN** 工具 MUST 继续通过 `--config-dir`、`--namespace`、`--group`、`--data-ids` 或对应环境变量描述单次发布
- **AND** 工具 MUST NOT 提供 `--config-root`、`--targets`、目录扫描或单进程多 Namespace 编排入口
- **AND** 工具 MUST 在网络写入前读取并校验当前目录的全部声明文档，再创建或复用单个 Namespace 并按 dataId 顺序覆盖发布
- **AND** 相同输入重复执行 MUST 幂等收敛；失败 MUST 返回包含 Namespace、group 和 dataId 的诊断，MUST NOT 自动删除已经发布的配置

#### Scenario: 完整配置防漂移

- **WHEN** CI 或本地验证比较 `local-host` 与 `local-docker` 配置
- **THEN** 测试 MUST 结构化解析同名 YAML 文档，并在排除精确声明的环境字段路径后要求其余配置相等
- **AND** 允许差异 MUST 仅覆盖主机名、端口、OTLP endpoint 或经审查确认依赖运行位置的叶子字段，MUST NOT 使用顶层 section 或通配路径跳过比较
- **AND** 两个目录的文件集合 MUST 恰好为固定三 dataId，每套文档 MUST 分别通过 user-service strict decode、normalize 和 validate

#### Scenario: 主机与 Compose 选择运行时来源

- **WHEN** user-service 或 RBAC seed 在 Compose 网络内运行
- **THEN** workload MUST 使用 `AEGISCORE_NACOS_NAMESPACE=loca-docker`，并按 `base.yaml`、`resources.yaml`、`user-service.yaml` 加载
- **AND** effective 配置 MUST 使用 Compose DNS 和容器端口访问 PostgreSQL、Redis 与 OTLP
- **WHEN** user-service 在主机直接运行并复用 Compose 依赖
- **THEN** 进程 MUST 使用 `AEGISCORE_NACOS_NAMESPACE=loca-host` 和相同三 dataId 顺序
- **AND** effective 配置 MUST 使用宿主机地址和 Compose 映射端口访问 PostgreSQL、Redis 与 OTLP

#### Scenario: 敏感配置边界

- **WHEN** 提交任一本地完整配置目录
- **THEN** 文件 MUST NOT 包含 Nacos 凭据、真实 JWT secret、真实 PostgreSQL/Redis 密码或其他 production-like secret
- **AND** Nacos 发布认证 MUST 仅通过进程环境或 Secret 注入，seed 工具 MUST NOT 把凭据或配置文档内容写入日志
- **AND** 固定本地开发占位值 MUST 明确限制为 local 使用且 MUST NOT 被 production-like 部署引用

#### Scenario: 从单一 loca Namespace 迁移

- **WHEN** 本地环境从旧 `loca` Namespace 和平铺配置迁移
- **THEN** 运维 MUST 先发布并验证 `loca-host` 与 `loca-docker`，再切换 Compose 和主机运行命令，最后移除 Git 中的旧平铺来源
- **AND** 迁移验证 MUST 分别检查两个 Namespace 的配置来源、严格校验、脱敏 effective render 和依赖地址
- **AND** seed 工具 MUST NOT 自动删除旧 `loca` Namespace；回滚 MAY 在旧内容仍保留时恢复旧来源选择，旧 Namespace 的最终清理由运维显式执行
