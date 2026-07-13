## 1. 目标识别

- [x] 1.1 检查 `common/runtime/config/validation.go`、`common/runtime/observability/metrics/labels.go`、`common/runtime/observability/metrics/scheduler.go`、`common/http/middleware/metrics.go`、`common/http/middleware/cors.go`、`common/validation/types.go` 和 `user-service/internal/features/permission/domain/method.go` 中的 package-level map、slice 和默认 struct。
- [x] 1.2 列出不迁移的 package-level var，并确认每处保留理由属于 sentinel error、regexp 编译结果、Fx Module、`sync.Pool`、atomic counter、Prometheus 状态或其他非只读集合对象。

## 2. common 共享 primitive 加固

- [x] 2.1 将 `common/runtime/config/validation.go` 中固定配置允许值、production-like environment 和 insecure JWT secret denylist 从共享可写 map 迁移为 `switch`、私有查询 helper 或等价不可共享写入表达。
- [x] 2.2 补齐或调整 `common/runtime/config` 测试，覆盖合法值、非法值、production-like 弱配置拒绝和错误消息语义不变。
- [x] 2.3 将 `common/http/middleware/cors.go` 默认 CORS options 改为构造时深拷贝，确保 `CORS()` 和 `CORSWithOptions` 创建后的 middleware 不受默认 slice 或调用方传入 slice 后续修改影响。
- [x] 2.4 补齐或调整 CORS 测试，验证默认响应头不变，并验证构造 middleware 后修改 options slice 不影响已创建 middleware。
- [x] 2.5 将 `common/validation/types.go` 中 request tag 顺序改为不暴露共享可写 slice 底层状态的表达，并保持字段名推导顺序不变。
- [x] 2.6 补齐或调整 `common/validation` 测试，覆盖 JSON、form、URI、query、header 等当前 tag 解析语义和优先级。

## 3. runtime observability 加固

- [x] 3.1 将 `common/runtime/observability/metrics/labels.go` 中 low-cardinality label key allowlist 迁移为 `switch`、私有查询 helper 或等价不可共享写入表达。
- [x] 3.2 调整 metrics label 测试，确认所有当前 label key 常量仍被接受，非法 label key 仍被拒绝且错误语义不变。
- [x] 3.3 将 `common/runtime/observability/metrics/scheduler.go` 中默认 scheduler duration buckets 改为数组源、函数返回副本或等价不可共享写入表达，并保持 bucket 顺序和值不变。
- [x] 3.4 将 `common/http/middleware/metrics.go` 中 HTTP metrics label names 改为数组源、函数返回副本或等价不可共享写入表达，并保持 descriptor label 顺序和值不变。
- [x] 3.5 运行或补齐 metrics 相关测试，确认 Prometheus metric family、label key、label value、label 顺序和数值语义不变。

## 4. RBAC permission method 加固

- [x] 4.1 将 `user-service/internal/features/permission/domain/method.go` 中 HTTP method allowlist 从共享可写 map 迁移为 `switch`、私有查询 helper 或等价不可共享写入表达。
- [x] 4.2 补齐或调整 permission domain 测试，覆盖全部当前允许 method、大小写归一化和非法 method 错误语义不变。
- [x] 4.3 确认 route diff、policy loader、policy sync、超级管理员通配授权和授权失败响应未因 method allowlist 表达调整而改变。

## 5. 验证与交付

- [x] 5.1 在 `common` 模块运行 `go test ./runtime/config ./runtime/observability/metrics ./http/middleware ./validation` 并确认通过。
- [x] 5.2 在 `user-service` 模块运行权限 HTTP method 校验相关测试，例如 `go test ./internal/features/permission/domain`，并确认通过。
- [x] 5.3 运行 `make user-service-architecture-lint`，确认 OpenSpec change artifacts 和架构规则通过校验。
- [x] 5.4 检查 `git diff`，确认没有修改 `user-service/ent/` 生成代码、OpenAPI 生成物、数据库 migration、部署资产或非本次范围文件。
- [x] 5.5 将本次预期代码、测试、OpenSpec artifacts 和相关文档变更加到暂存区。
- [x] 5.6 运行 `make lint` 并确认通过。
- [x] 5.7 运行 `make verify` 并确认通过，若 verify 产生生成物或 drift，修复后重复暂存与验证。
