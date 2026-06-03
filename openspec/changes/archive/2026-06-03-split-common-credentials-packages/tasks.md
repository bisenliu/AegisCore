## 1. 拆分 common 包结构

- [x] 1.1 创建 `common/password` 包，将密码 hash/verify 实现与测试从 `common/credentials` 迁移过去，并将 package 名改为 `password`。
- [x] 1.2 将密码公开函数从 `HashPassword`、`VerifyPassword` 调整为 `Hash`、`Verify`，保留 `ErrEmptyPassword`、`ErrInvalidHash`、Argon2id 参数和 hash 格式。
- [x] 1.3 创建 `common/auth` 包，将认证 context helper、Authorization/Bearer 常量、JWT service、claims、subject、sign input、错误变量与测试从 `common/credentials` 迁移过去，并将 package 名改为 `auth`。
- [x] 1.4 删除 `common/credentials` 目录，确认不保留旧包路径兼容层。

## 2. 更新调用方导入与 API 使用

- [x] 2.1 更新 `common/middleware/auth.go`、`common/middleware/cors.go` 和对应测试，改为导入并使用 `common/auth`。
- [x] 2.2 更新 `user-services/internal/bootstrap`、`user-services/internal/router`、`user-services/internal/controller` 中的认证常量、JWT service 和类型引用，改为使用 `common/auth`。
- [x] 2.3 更新 `user-services/internal/service/auth_service.go`，密码 hash/verify 使用 `common/password`，JWT、token prefix、token type 和 context helper 使用 `common/auth`。
- [x] 2.4 更新 `user-services/internal/service/user_service.go` 和相关测试，密码 hash/verify 改为 `password.Hash`、`password.Verify`。
- [x] 2.5 全仓搜索 `common/credentials`、`credentials.`、`HashPassword`、`VerifyPassword`，确认旧导入和旧函数名已清除。

## 3. 验证与格式化

- [x] 3.1 对迁移后的 Go 文件运行 `gofmt`。
- [x] 3.2 在 `common/` 运行 `go test ./...`，验证 password、auth 和 middleware 测试通过。
- [x] 3.3 在 `user-services/` 运行 `go test ./...`，验证 bootstrap、router、controller、service 和认证会话测试通过。
- [x] 3.4 确认本变更不涉及 Ent schema、生成代码或 Atlas migration，因此无需运行 `go generate ./ent` 或迁移脚本。
