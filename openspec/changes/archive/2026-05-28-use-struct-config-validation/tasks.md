## 1. Struct 校验规则建模

- [x] 1.1 在 `common/config/config.go` 的 Config、App、HTTP、Log、Redis、Database、Postgres 相关结构体字段上添加 `validate` tag，表达必填、可选和基础范围约束
- [x] 1.2 明确可选字段语义，避免 Redis username/password、PostgreSQL password、HTTP trusted proxies 因为空或省略而校验失败
- [x] 1.3 为端口、连接池大小、Redis DB、HTTP timeout、Redis timeout、PostgreSQL timeout 等字段声明基础范围或正值约束

## 2. Loader 校验逻辑收敛

- [x] 2.1 在 `common/config/loader.go` 恢复使用 `go-playground/validator/v10` 对解码后的 `Config` 执行结构体验证
- [x] 2.2 删除 `validateConfig` 中逐字段手写 if 校验，保留或新增小型 helper 只处理 validator 错误格式化和必要的显式存在性检查
- [x] 2.3 将 `requiredKeys` 或等价 key 清单定位为环境变量绑定/显式存在性用途，避免同时承担字段范围校验职责
- [x] 2.4 如 validator 默认错误不可读，添加错误格式化逻辑，使错误信息包含配置路径或字段名，便于定位缺失配置

## 3. 测试更新

- [x] 3.1 更新 `common/config` 测试，验证缺少必填字段时由 struct tag 校验失败
- [x] 3.2 更新测试，验证 Redis username/password、PostgreSQL password、HTTP trusted proxies 等可选字段为空或省略时不会仅因为空失败
- [x] 3.3 保留环境变量覆盖测试，确认无默认值且使用 struct 校验时 `AEGISCORE_` 覆盖仍生效
- [x] 3.4 保留或更新端口、连接池大小和 timeout 无效值测试，确认基础范围约束仍生效

## 4. 验证

- [x] 4.1 如修改 Go 源码，运行 `gofmt -w` 格式化相关文件
- [x] 4.2 在 `common/` 目录运行 `go test ./...` 并确认通过
- [x] 4.3 在 `user-services/` 目录运行 `go test ./...` 并确认通过
- [x] 4.4 确认未修改 Ent schema 或 `user-services/ent/` 生成代码，因此无需运行 `go generate ./ent`
- [x] 4.5 运行 `openspec status --change "use-struct-config-validation"`，确认 change 处于可应用状态
