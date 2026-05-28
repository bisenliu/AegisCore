## 1. 配置加载策略调整

- [x] 1.1 移除 `common/config/loader.go` 中的 Viper `SetDefault` 默认配置注册，确保主要配置不再由代码自动填充
- [x] 1.2 调整或重命名 `applyDefaults`，移除 app、HTTP、log、Redis、PostgreSQL 主要运行时字段的默认补齐逻辑
- [x] 1.3 为 app、HTTP、log、Redis、PostgreSQL 关键字段添加显式校验，缺失或无效时返回清晰错误
- [x] 1.4 确认 `AEGISCORE_` 环境变量覆盖在无 Viper 默认值时仍可覆盖已声明配置；如无法覆盖关键字段，显式绑定必要配置 key

## 2. 配置示例与兼容性检查

- [x] 2.1 检查 `user-services/configs/config.yaml` 是否完整声明 app、HTTP、log、Redis、PostgreSQL 主要配置字段，并补齐无默认值后必需的字段
- [x] 2.2 搜索仓库中关于默认配置或可省略主要配置的说明，并更新为“主要配置必须显式提供”
- [x] 2.3 确认配置缺失时用户服务会在 `common/config.Load` 或 Fx app 初始化早期失败，不进入 HTTP server 启动

## 3. 测试更新

- [x] 3.1 更新 `common/config` 测试 fixture，使完整 YAML 在无代码默认值时仍可成功加载
- [x] 3.2 添加缺失 app、HTTP、log、Redis、PostgreSQL 主要配置字段的失败测试，验证错误信息指向具体字段
- [x] 3.3 添加或保留环境变量覆盖测试，确认 `AEGISCORE_` 覆盖规则在移除 Viper 默认值后仍有效
- [x] 3.4 更新 PostgreSQL 配置测试，使 driver、端口、连接池大小和 timeout 等运行时参数必须由 YAML 或环境变量提供

## 4. 验证

- [x] 4.1 如修改 Go 源码，运行 `gofmt -w` 格式化相关文件
- [x] 4.2 在 `common/` 目录运行 `go test ./...` 并确认通过
- [x] 4.3 在 `user-services/` 目录运行 `go test ./...` 并确认通过
- [x] 4.4 确认未修改 Ent schema 或 `user-services/ent/` 生成代码，因此无需运行 `go generate ./ent`
- [x] 4.5 运行 `openspec status --change "require-explicit-config"`，确认 change 处于可应用状态
