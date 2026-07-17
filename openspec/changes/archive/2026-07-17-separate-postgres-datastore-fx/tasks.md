## 1. 规格与核心实现

- [x] 1.1 创建 proposal、design、shared-platform-primitives spec delta 和 tasks。
- [x] 1.2 实现框架无关的单资源 PostgreSQL constructor、Ping、Close 和失败回滚。
- [x] 1.3 删除 `NewPostgresPools`、map result 和旧 `NewPostgres` 签名。

## 2. Fx 与服务组装

- [x] 2.1 将 PostgreSQL 和 Redis Fx adapter 作为独立 `*_fx.go` 文件共置在 `common/runtime/datastore`。
- [x] 2.2 将 user-service PostgreSQL composition 改为显式 `NewPrimaryDB`，只选择 `primary_db` 配置。
- [x] 2.3 更新核心、Fx adapter 和 user-service 直接测试。
- [x] 2.4 将 Redis Fx constructor 和 user-service composition 收敛为显式单资源配置。

## 3. 验证与归档

- [x] 3.1 运行相关 common 和 user-service Go 测试。
- [x] 3.2 运行 datastore 相关测试和 `make user-service-architecture-lint`。
- [x] 3.3 暂存本次预期变更后运行 `make lint` 和 `make verify`。
- [x] 3.4 归档 change 并确认 shared-platform-primitives 主规格已更新。
