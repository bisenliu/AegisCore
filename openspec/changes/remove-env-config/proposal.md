## Why

项目当前同时支持配置文件、环境变量覆盖、Compose `environment`/插值以及脚本级环境变量，导致配置来源不唯一、启动行为难以复现，也增加了部署资产与运行时配置发生漂移的风险。需要统一为显式配置文件加载，使本地、容器和集群环境使用同一套可审查、可验证的配置契约。

## What Changes

- **BREAKING**：移除 `common/runtime/config`、user-service 配置加载、timezone 初始化及 CLI/启动路径中的环境变量读取与覆盖能力，应用配置只能来自显式指定的配置文件。
- **BREAKING**：移除 Compose 中的 `environment`、`env_file` 和 `${VAR}` 插值；为 user-service、PostgreSQL、Redis、Prometheus、Grafana 及一次性命令提供文件化配置或 secret file 挂载。
- 将 JWT、数据库密码、Grafana 管理员凭据、RBAC bootstrap 凭据等敏感项迁移为不入库的受控配置/secret 文件，并提供可提交的无真实 secret 示例。
- 更新 Docker、Compose、Kubernetes、Helm、Makefile 与启动/生成脚本，使其通过命令参数或固定挂载路径选择配置文件，不再依赖进程环境变量传参。
- 增加静态检查和测试，阻止生产代码、部署清单与启动脚本重新引入环境变量配置入口，并验证缺失、未知或不安全的文件配置会在启动前失败。
- 更新配置示例、开发文档和交付说明，明确不同环境配置文件的生成、挂载、权限和 secret 轮换方式。

## Capabilities

### New Capabilities

无。

### Modified Capabilities

- `shared-platform-primitives`：将共享配置加载、进程时区和服务装配契约改为仅从显式配置文件读取，取消环境变量覆盖与 `TZ` 特例。
- `delivery-operations`：将 Compose、容器、Kubernetes、Helm、Makefile 和脚本的运行配置统一迁移为文件挂载与显式配置路径，并增加禁止环境变量配置的交付门禁。

## Impact

- 影响 `common/runtime/config/`、`common/runtime/timezone/`、user-service CLI/config/bootstrap/provider 装配以及相关测试。
- 影响 `deployments/compose/`、Docker image/entrypoint、Kubernetes/Helm 配置与 secret 挂载、观测组件配置、根与服务级 Makefile 及 `user-service/scripts/`。
- 不改变 HTTP API、OpenAPI 契约、Ent schema 或 Atlas migration；会改变所有启动、seed、bootstrap 和部署调用方式，现有依赖环境变量的命令与发布配置必须同步迁移。
- 敏感值不再通过环境变量注入，改由部署平台生成或挂载权限受控且不提交仓库的配置/secret 文件；仓库只保留安全示例和校验规则。
