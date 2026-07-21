## 1. 收敛共享和服务实现

- [x] 1.1 删除 `JSONBinderWithOptions`、未使用的正数 int 校验 wrapper 和旧共享 Fx config loader
- [x] 1.2 将 auth 与 permission 的 `MustKeyCatalog` 从生产文件迁移为 `_test.go` 内小写 helper，并更新全部测试调用
- [x] 1.3 运行受影响 common、auth Redis 和 permission Redis package 测试及 lint

## 2. 清理交付资产和失效文档

- [x] 2.1 删除真实指标脚本未读取的 refresh token 状态和额外登录，并执行 shell 语法检查
- [x] 2.2 合并 `.gitignore` 重复规则并删除旧 `services/` 规则
- [x] 2.3 删除 Dockerfile 未消费的 tools 源复制、收窄 `.dockerignore`，并验证 user-service 镜像构建和内容
- [x] 2.4 更新 pprof 环境变量、Go lint 规格链接和 gosec 版本文档，保留 legacy 环境变量负向测试

## 3. 验证与交付

- [x] 3.1 运行 `openspec validate remove-unused-repository-code`、`openspec validate --specs` 和 `make user-service-architecture-lint`
- [x] 3.2 检查全仓反向引用、生成物和 `git diff`，确认保留项未被误删且没有 OpenAPI、Ent 或 migration drift
- [x] 3.3 将本次预期变更暂存后运行 `make lint`
- [x] 3.4 保持预期变更已暂存并运行 `make verify`，确认完整构建、测试、生成和 drift 检查通过
