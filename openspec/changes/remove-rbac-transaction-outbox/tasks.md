## 1. 规格与数据模型

- [x] 1.1 更新 RBAC 与 observability 主规格，移除事务 outbox 和 dispatcher 要求
- [x] 1.2 删除 outbox Ent schema 与 revision edge，并生成 Ent 代码和 Atlas migration

## 2. 同步实现

- [x] 2.1 让 policy/user-role 事务只提交业务事实和 revision
- [x] 2.2 将提交后的 Redis 发布改为 best-effort hint，删除 dispatcher、store 和配置
- [x] 2.3 增加 user-role revision 水位补偿，并清理健康检查、指标和测试

## 3. 验证

- [x] 3.1 暂存本次预期变更后运行 `make lint` 与 `make verify`
