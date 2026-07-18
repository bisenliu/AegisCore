## 1. 测试与现状定位

- [x] 1.1 梳理 `common/runtime/observability/tracing/fx.go`、`common/runtime/datastore/redis_fx.go`、`user-service/internal/features/auth/fx.go`、`user-service/internal/features/permission/fx.go`、`user-service/internal/providers/ent.go` 中 constructor 创建运行资源和 stop hook 注册点。
- [x] 1.2 为 tracing Fx provider 增加启动失败路径测试，覆盖 `OnStart` 成功后后续 hook 失败时 provider/exporter 被关闭。
- [x] 1.3 为 Redis Fx adapter 增加启动 PING 失败和后续启动失败路径测试，覆盖 client 关闭和错误保留。
- [x] 1.4 为 auth session purge pool 与 token-version local cache 增加启动失败和停止幂等测试，验证不关闭共享 Redis client。
- [x] 1.5 为 permission/RBAC watcher 与 user-role cache 增加启动失败和停止幂等测试，验证 watcher/cache 不关闭共享 Redis、Ent 或 PostgreSQL 资源。
- [x] 1.6 为 Ent wrapper 或相关 provider 增加部分构造失败清理测试，覆盖已创建资源即时关闭和错误链保留。

## 2. Common Runtime 生命周期实现

- [x] 2.1 调整 `common/runtime/observability/tracing/fx.go`，让 Fx constructor 只完成静态依赖投影或返回 holder，真实 tracing provider/exporter 在 `OnStart` 创建并在 `OnStop` 关闭。
- [x] 2.2 保持 tracing disabled 模式的非 nil no-op 语义，验证禁用时不连接 OTLP exporter 且不启动 batch processor。
- [x] 2.3 调整 `common/runtime/datastore/redis_fx.go` 的启动探测和关闭路径，确保启动 PING 失败、后续启动失败和正常停止均关闭同一个 client。
- [x] 2.4 检查 workerpool、localcache、scheduler 的拥有者关闭契约，必要时补齐幂等关闭或测试覆盖，不新增业务语义到 `common`。

## 3. User-Service Feature 生命周期实现

- [x] 3.1 调整 `user-service/internal/features/auth/fx.go`，将 session purge pool、token-version local cache 等主动资源迁移为 `OnStart` 创建、`OnStop` 关闭或补齐 constructor 局部 rollback。
- [x] 3.2 保持 auth 登录、refresh、强制改密、退出、token version 校验和安全撤销行为不变，并确保 holder 未启动或关闭时不会安全降级。
- [x] 3.3 调整 `user-service/internal/features/permission/fx.go`，确保 initial load、Redis policy watcher 和 user-role cache 由 composition 层显式 lifecycle hook 编排。
- [x] 3.4 保持 permission/RBAC policy reload、Redis policy version、Pub/Sub、周期性补偿、授权 fail-closed 和同步错误传播语义不变。
- [x] 3.5 调整 `user-service/internal/providers/ent.go` 或相关 Ent observability provider 的部分失败清理，保持 Ent 查询 tracing 和 metrics 语义不变。

## 4. 验证与收尾

- [x] 4.1 运行相关包测试：`go test ./common/runtime/observability/tracing/...`、`go test ./common/runtime/datastore/...`、`go test ./user-service/internal/features/auth/...`、`go test ./user-service/internal/features/permission/...`、`go test ./user-service/internal/providers/...`。
- [x] 4.2 运行 `make user-service-architecture-lint`，确认 feature/infrastructure 边界没有引入 Fx/Dig 回归或共享边界违规。
- [x] 4.3 若 API 注解、Ent schema、部署观测资产未变化，确认无需运行 OpenAPI、migration 或 dashboard 生成；若发生变化，运行对应生成与 drift 检查。
- [x] 4.4 运行 `openspec status --change "harden-fx-resource-lifecycle"`，确认 artifacts 完整且 implementation checklist 可解析。
- [x] 4.5 将本次预期代码、测试、OpenSpec artifacts 和必要文档变更加到暂存区。
- [x] 4.6 运行 `make lint`。
- [x] 4.7 运行 `make verify`，确认最终验证通过且没有未暂存的预期变更阻塞 drift 检查。
