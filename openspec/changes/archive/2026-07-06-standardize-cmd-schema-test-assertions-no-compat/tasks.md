## 1. 基线与范围确认

- [x] 1.1 确认 `user-service/go.mod` 已直接声明 `github.com/stretchr/testify`，本 change 不新增无关依赖或 tidy 漂移。
- [x] 1.2 扫描 `user-service/cmd/**/*_test.go` 和 `user-service/ent/schema/**/*_test.go` 中 `t.Fatalf`、`t.Fatal`、`t.Errorf`、`t.Error` 和 `Fail` 类调用，按 package 记录迁移范围。
- [x] 1.3 扫描目标范围内现有 `github.com/stretchr/testify/require` 和 `github.com/stretchr/testify/assert` 使用点，确认需要新增或调整 import 的文件。
- [x] 1.4 确认本次只修改 cmd、Ent schema `_test.go` 和本 change artifacts，不修改 CLI 生产代码、Ent schema、Ent 生成代码、Atlas migration、OpenAPI 或部署资产。

## 2. cmd 测试断言迁移

- [x] 2.1 迁移 `user-service/cmd` root/serve command 测试中的 command construction、usage、flag/env normalization、执行错误和 cleanup error 断言。
- [x] 2.2 迁移 `user-service/cmd` RBAC seed、assign-super-admin 和 create-super-admin 相关测试中的 command metadata、password/env、错误路径、输出文本和 cleanup behavior 断言。
- [x] 2.3 保持当前 CLI command graph、flag/env 名称、服务前缀 Make target 和 RBAC 引导语义不变，不新增旧命令或旧参数兼容断言。

## 3. Ent schema 测试断言迁移

- [x] 3.1 迁移 `user-service/ent/schema` 用户与认证相关 schema 测试中的 field、edge、index、annotation、default、validator 和 mixin 断言。
- [x] 3.2 迁移 `user-service/ent/schema` RBAC 相关 schema 测试中的 role、permission、user-role、role-permission field/index 和 relation 断言。
- [x] 3.3 对多个互相独立的 schema field/index/annotation 检查按需使用 `assert`，前置查找和依赖性检查继续使用 `require`。
- [x] 3.4 保持 Ent schema、Ent 生成代码、Atlas migration 和 schema 运行时行为不变。

## 4. 格式化与残留例外

- [x] 4.1 对修改过的 Go 测试文件运行 `gofmt`，清理未使用 import。
- [x] 4.2 运行 `rg "t\\.Fatalf|t\\.Fatal\\(|t\\.Errorf|t\\.Error\\(|Fail(Now)?f?\\(" user-service/cmd user-service/ent/schema --glob '*_test.go'`，确认剩余命中均符合 `docs/TESTING.md` 特殊例外。
- [x] 4.3 在本文件记录所有保留的 `t.Fatal` / `t.Error` / `Fail*` 例外及原因；如果没有剩余命中，记录为无残留例外。
- [x] 4.4 运行 `rg "github.com/stretchr/testify/(require|assert)" user-service/cmd user-service/ent/schema --glob '*_test.go'`，确认迁移后的实际使用点可定位。

## 5. 验证

- [x] 5.1 运行 `go test ./user-service/cmd ./user-service/ent/schema` 并确认通过。
- [x] 5.2 运行 `openspec validate standardize-cmd-schema-test-assertions-no-compat` 并确认通过。
- [x] 5.3 运行 `make user-service-architecture-lint`，确认 OPSX 文档语言、生成物 drift 和架构边界检查通过。
- [x] 5.4 将本次预期代码、测试和 OpenSpec 变更加到暂存区后运行 `make lint`，确认 lint 通过且不被预期 diff 影响。
- [x] 5.5 保持本次预期变更已暂存，运行 `make verify`，确认完整验证通过或记录无法运行的具体原因。

## 6. 剩余例外记录

- [x] 6.1 汇总最终残留例外；若最终扫描未发现 `t.Fatal`、`t.Error` 或 `Fail` 命中，记录为无残留例外：最终残留扫描无命中。
