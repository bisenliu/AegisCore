## 1. 配置加载与运行时契约

- [x] 1.1 将 `common/runtime/config` 改为只读取一份显式完整 YAML，删除全部 Viper 环境变量绑定、fallback 和多文件 overlay，并补充缺失文件和严格字段测试
- [x] 1.2 将进程 timezone 纳入共享 runtime 配置，删除 `TZ` 读取路径并更新 timezone、默认值和配置校验测试
- [x] 1.3 更新 user-service 配置 schema、示例与 production-like secret 校验，使运行时、资源和认证配置都来自同一份完整 YAML

## 2. CLI 与应用装配

- [x] 2.1 将 `serve`、`rbac seed` 和 `rbac bootstrap-super-admin` 统一改为单个显式 `--config` 参数，并确保创建 Fx App 或 use case 前只加载一次配置
- [x] 2.2 删除 RBAC bootstrap、测试 harness 和服务启动路径中的环境变量读取，改为配置对象或临时 YAML，并更新相关单元与集成测试

## 3. Compose 与容器配置

- [x] 3.1 重构 `deployments/compose/docker-compose.yml`，移除 `environment`、`env_file` 和 `${VAR}`，为 user-service、PostgreSQL、Redis、Prometheus 与 Grafana 挂载文件化配置
- [x] 3.2 为无法直接从文件读取初始化 secret 的第三方容器新增最小 `deployments` wrapper/配置资产，并验证全新 volume、已有 volume、健康检查和启动顺序
- [x] 3.3 更新 user-service runtime image、镜像验证脚本和 Compose 辅助脚本，使其通过显式配置路径工作且不读取环境变量配置

## 4. Kubernetes、Helm 与交付工具

- [x] 4.1 将原生 Kubernetes 与 Helm 改为只读投影一份完整配置文件，删除 ConfigMap/Secret overlay、`env`、`envFrom` 和环境变量 secret 注入并保持 seed/bootstrap/Deployment 配置一致
- [x] 4.2 将 Makefile、CI、migration/OpenAPI/architecture lint 和测试开关迁移为显式参数、专用 target 或工具配置文件，删除配置用途的 shell 环境变量读取
- [x] 4.3 增加仓库静态门禁，拒绝 Go 环境变量配置读取、Viper env binding、Compose/Kubernetes 环境配置和脚本配置变量，并为允许的非配置 shell 机制建立最小 allowlist

## 5. 文档与验证

- [x] 5.1 更新开发、测试、部署和 RBAC bootstrap 文档，说明完整配置文件、文件权限、挂载、轮换和迁移/回滚流程
- [x] 5.2 运行相关 Go 测试、`make user-service-architecture-lint`、Compose 配置/健康检查、Kubernetes/Helm render 与 schema/dry-run 验证，并修复本 change 引入的问题
- [x] 5.3 暂存本次预期源码、测试、部署、文档和 `openspec/changes/remove-env-config/` 变更，运行 `make lint` 与 `make verify`，并用限定路径的 drift 检查确认生成物与工作区结果
