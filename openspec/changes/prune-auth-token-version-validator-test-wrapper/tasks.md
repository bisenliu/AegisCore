## 1. 测试 helper 整理

- [x] 1.1 在 `user-service/internal/features/auth/application/validators/token_version_validator_test.go` 中删除 `testTokenVersionValidator` 类型。
- [x] 1.2 将 `newTestTokenVersionValidator` 返回类型改为 `*TokenVersionValidator`，并直接返回 `NewCachingValidator(cache)`。
- [x] 1.3 保留 helper 内 `localcache.New` 构造和 `t.Cleanup(cache.Close)`，确保 cache 只作为局部生命周期资源存在。

## 2. 语义保持检查

- [x] 2.1 检查现有 token version validator 测试用例、测试名称、断言和并发场景不发生无关调整。
- [x] 2.2 确认没有修改 `TokenVersionValidator` 生产代码、`common/runtime/localcache` 或 token version 业务语义。

## 3. 验证与收尾

- [x] 3.1 运行 `go test ./internal/features/auth/application/validators` 于 `user-service` 模块，确认 auth validators 包测试通过。
- [x] 3.2 检查 `git diff`，确认本 change 只包含目标测试文件和 `openspec/changes/prune-auth-token-version-validator-test-wrapper/` artifacts。
- [x] 3.3 实现完成后将对应 tasks checkbox 更新为 `- [x]`，并确认 `openspec status --change prune-auth-token-version-validator-test-wrapper` 状态正常。
