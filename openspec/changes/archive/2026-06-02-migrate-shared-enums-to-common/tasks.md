## 1. 枚举与常量审查

- [x] 1.1 扫描 `common/` 和 `user-services/` 中的 enum-like 类型、`const` 组、重复字符串常量和测试断言，形成迁移清单与非迁移清单。
- [x] 1.2 确认 `common/response`、`common/validation`、`common/jwt`、`common/contextutil` 中已存在的公共枚举/常量，避免重复创建等价定义。
- [x] 1.3 明确标记 `user-services/internal/apperror` 业务文案和 `user-services/ent/` 生成常量为非迁移项，并记录原因。

## 2. Common 常量迁移

- [x] 2.1 在 `common/infrastructure` 增加共享运行时资源名称常量，覆盖 `user_db`、`common_db`、`cache_redis`，并保持字符串值不变。
- [x] 2.2 整合 Bearer/Auth 边界常量，确保 `Authorization`、`Bearer`、`Bearer ` 由 `common` 统一表达，同时保留现有导出名称兼容性。
- [x] 2.3 在 `common/response` 增加通用认证失败公开文案常量，保持值为 `登录状态无效或已过期，请重新登录`。

## 3. 引用更新

- [x] 3.1 更新 `user-services/internal/bootstrap` 中 Redis/PostgreSQL provider 的非 tag 资源名称引用，改用 `common` 运行时资源名称常量。
- [x] 3.2 更新 `user-services/internal/entclient`、`user-services/internal/repository` 和相关测试中的可替换资源名称引用；保留 Go struct tag 中必须存在的 Fx name 字符串。
- [x] 3.3 更新 `common/middleware/auth.go` 和认证相关测试，使认证失败响应 message 使用 `common/response` 共享常量。
- [x] 3.4 更新 `user-services/internal/service`、`common/middleware` 和相关测试中的 Bearer/Auth 常量引用，移除可替换的重复硬编码。

## 4. 验证与兼容性确认

- [x] 4.1 运行 `gofmt` 格式化所有修改过的 Go 文件。
- [x] 4.2 在 `common/` 执行 `go test ./...`，确认 response、validation、jwt、contextutil、middleware 和 infrastructure 测试通过。
- [x] 4.3 在 `user-services/` 执行 `go test ./...`，确认 bootstrap、auth service、repository、controller 和 Swagger 相关测试通过。
- [x] 4.4 确认未修改 `user-services/ent/` 生成代码、Ent schema、Atlas migration、数据库字段、HTTP API 响应结构、业务码数值、header 名或配置 key。

## 5. 交付说明

- [x] 5.1 输出已迁移或整合常量的清单，包括原位置、目标位置和主要影响文件。
- [x] 5.2 输出未迁移常量清单，包括用户服务业务文案、Ent 生成常量和已在 common 中满足要求的公共枚举。
- [x] 5.3 输出兼容性注意事项，特别说明 Fx struct tag 字符串保留原因以及外部协议值未变化。
