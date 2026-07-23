## 1. Compose 默认监听面

- [x] 1.1 删除 Compose 中 pprof 默认启用环境变量和 `6060:6060` 宿主端口暴露。
- [x] 1.2 将 Compose 中 gRPC 默认配置改为关闭，并删除 `19090:9090` 宿主端口暴露。

## 2. 文档与依赖卫生

- [x] 2.1 更新 `deployments/compose/README.md`，保持 pprof 不进入默认配置的说明，并明确本地 tracing 的 Ent/otelsql 分层定位。
- [x] 2.2 将 `github.com/XSAM/otelsql` 移到 `common/go.mod` direct require 并去掉 `// indirect`。

## 3. 测试覆盖

- [x] 3.1 在 `common/runtime/datastore/redis_test.go` 增加 Redis command filter 单元测试，覆盖 `auth`、`hello ... auth`、`ping` 和普通命令。

## 4. 验证与收尾

- [x] 4.1 运行 `go test ./runtime/datastore`。
- [x] 4.2 运行带占位环境变量的 `docker compose -f deployments/compose/docker-compose.yml config` 并确认 pprof/gRPC 端口不再发布。
- [x] 4.3 运行 `make user-service-architecture-lint`。
- [x] 4.4 检查 `git diff`，确认只包含本 change 预期代码、文档、部署资产和 OpenSpec artifacts。
- [x] 4.5 将本次预期变更加到暂存区后运行 `make lint`。
- [x] 4.6 保持本次预期变更处于暂存区后运行 `make verify`。
- [x] 4.7 所有验证通过后，将已完成任务 checkbox 更新为 `- [x]`。
