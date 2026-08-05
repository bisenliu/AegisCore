## 1. 固化变更契约

- [x] 1.1 完成 proposal、design 和 `shared-platform-primitives` spec delta，明确公开 API 不变、泛型 nil value 安全和容量驱逐同步统计
- [x] 1.2 运行 `openspec validate harden-runtime-cache-and-workerpool-layout` 并确认 apply artifacts 完整

## 2. 拆分与修复 localcache

- [x] 2.1 将 package 文档、公开类型、核心读取、singleflight loading、失效和统计按同 package 文件拆分
- [x] 2.2 使用包内泛型 flight result 修复接口类型 nil value panic，并增加首次加载与缓存命中测试
- [x] 2.3 移除 `ttlcache.OnEviction`，在发布锁内同步累计容量驱逐，覆盖 TTL、单 key 失效和全量失效不增加统计

## 3. 拆分 workerpool

- [x] 3.1 将 package 文档、公开类型、准入、任务执行、生命周期和统计按同 package 文件拆分
- [x] 3.2 保持构造、阻塞提交、context 联动、panic recovery、统计和重复 Stop drain 行为不变，并修正 ants 预分配说明

## 4. 验证与交付

- [x] 4.1 执行 `gofmt`、`cd common && go vet ./runtime/workerpool ./runtime/localcache`、相关普通测试和 `go test -race ./runtime/workerpool ./runtime/localcache ./runtime/observability/metrics`
- [x] 4.2 运行 `openspec validate harden-runtime-cache-and-workerpool-layout`、`openspec list --specs`、`openspec validate --specs`、`make user-service-architecture-lint` 和 `git diff --check`
- [x] 4.3 检查 `git status --short`，只暂存本次预期 change、workerpool 和 localcache 文件
- [x] 4.4 在预期变更已暂存后运行 `make lint`，成功后勾选本任务
- [x] 4.5 在预期变更已暂存后运行 `make verify`，成功后勾选本任务并复核 staged diff
