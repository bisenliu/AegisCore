
## 1. OpenSpec 基线

- [X] 1.1 完成 proposal、design、五个 capability spec delta 和本 tasks 清单
- [X] 1.2 运行 `openspec status --change simplify-runtime-config` 和 `openspec validate simplify-runtime-config`
- [X] 1.3 将原 `shrink-runtime-config-core` artifacts 合并到本 change，并移除重复 change

## 2. 共享资源配置

- [X] 2.1 新增 `common/runtime/resources` 的 Redis/PostgreSQL 具名资源类型
- [X] 2.2 实现资源默认值和多具名资源强校验
- [X] 2.3 增加 resources 单元测试并运行对应包测试
- [X] 2.4 暂存本任务全部预期变更

## 3. 核心配置契约

- [X] 3.1 将核心 `Config` 收敛为 App、Server、Log 和 Observability，并删除旧公共类型与 helper
- [X] 3.2 更新新结构加载、duration、环境变量和服务扩展 mapstructure 测试
- [X] 3.3 运行 `go test ./runtime/config`
- [X] 3.4 补齐核心默认值、至少一个 server enabled、log format、metrics 和 tracing 强校验
- [X] 3.5 实现严格 unknown key 检查及旧字段路径失败测试
- [X] 3.6 暂存本任务全部预期变更

## 4. Datastore 迁移

- [X] 4.1 将 Redis/PostgreSQL client 和 DSN 构建迁移到 resources 类型
- [X] 4.2 应用 Redis timeout、PostgreSQL pool 和内部 ping timeout 默认值
- [X] 4.3 更新 datastore 与 `common/testing/containers` 测试并运行对应包测试
- [X] 4.4 暂存本任务全部预期变更

## 5. User-service 资源配置与 providers

- [X] 5.1 定义服务级 Config 和 ResourcesConfig，迁移具名 Redis/PostgreSQL 路径
- [X] 5.2 迁移 PostgreSQL、Redis、metrics、health check 和 RBAC CLI 初始化路径
- [X] 5.3 解除 timezone primitive 和启动日志对 `system.timezone` 的依赖，改用平台 `TZ` 和实际进程时区
- [X] 5.4 更新 provider 测试替身、多资源、timezone 和服务配置校验测试
- [X] 5.5 暂存本任务全部预期变更

## 6. HTTP server 接线

- [X] 6.1 将 bootstrap、providers、router 和 CLI 迁移到 `cfg.Server.HTTP`
- [X] 6.2 让 HTTP 生命周期遵守 enabled，移除业务 router 的旧 pprof/trusted proxies 接线，并解除 metrics path 对 PprofConfig 的依赖
- [X] 6.3 更新 HTTP server、router 和 provider 定向测试
- [X] 6.4 暂存本任务全部预期变更

## 7. Feature cache 配置

- [X] 7.1 将 auth token version cache 迁移到 auth 自有配置
- [X] 7.2 将 permission/Casbin user role cache 迁移到 RBAC 自有配置
- [X] 7.3 覆盖默认值、disabled 和业务正确性测试
- [X] 7.4 暂存本任务全部预期变更

## 8. Logging、pprof、trusted proxies 和 tracing

- [X] 8.1 移除文件日志与轮转实现，统一 stdout/stderr 和结构化字段
- [X] 8.2 统一 Ent SQL debug、slow 和 error 日志级别及字段
- [X] 8.3 将 pprof 迁移到默认关闭的独立诊断监听并增加生产地址校验
- [X] 8.4 验证 Gin trusted proxy 安全默认值，且不重新引入核心 proxy 配置
- [X] 8.5 简化 OTLP tracing provider，删除 exporter 分支
- [X] 8.6 运行 logger、SQL、pprof、HTTP 初始化和 tracing 定向测试
- [X] 8.7 暂存本任务全部预期变更

## 9. 配置、部署和文档迁移

- [X] 9.1 更新 user-service configs、测试 fixture 和本地开发配置
- [X] 9.2 更新 Compose、Docker、Kubernetes、Helm、脚本、环境变量和 Secret 示例
- [X] 9.3 更新 README、ARCHITECTURE、DEVELOPMENT、TESTING 和 CAPABILITY_MAP
- [X] 9.4 全仓扫描并清理旧 system/http/redis/postgres/local_cache/pprof/trusted_proxies/file log/exporter 契约
- [X] 9.5 运行 `make user-service-architecture-lint`
- [X] 9.6 暂存本任务全部预期变更

## 10. 最终验证和归档

- [X] 10.1 运行相关 common 和 user-service 包测试
- [X] 10.2 运行 `openspec validate simplify-runtime-config`、`openspec list --specs` 和 `openspec validate --specs`
- [X] 10.3 确认本 change 全部预期代码、测试、配置、部署、文档和规格变更已暂存
- [X] 10.4 运行 `make lint` 和 `make verify`
- [X] 10.5 执行 `/opsx:verify simplify-runtime-config`，确认 artifacts、实现和规格一致
- [X] 10.6 确认所有 tasks 已完成，且无生成物 drift、预期外未暂存变更或未跟踪文件
