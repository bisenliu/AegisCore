## 1. OpenSpec

- [x] 1.1 创建 `split-nacos-config-source-client` change。
- [x] 1.2 为 `shared-platform-primitives` 增加 Nacos source adapter 职责和失败语义 delta。
- [x] 1.3 为 `delivery-operations` 增加 Nacos adapter 示例和测试门禁 delta。

## 2. 实现

- [x] 2.1 将 `client.go` 中的 failover、auth、HTTP transport 和 endpoint 解析拆为同 package 聚焦文件。
- [x] 2.2 保持 server 顺序、总 timeout 分配、response size limit、Bearer token 和 safe error message 行为不变。
- [x] 2.3 新增 `doc.go` 描述 Nacos package 边界。
- [x] 2.4 新增 `example_test.go`，使用本地 `httptest.Server` 展示多 dataId source 到 `config.LoadSource` 的组合。

## 3. 验证

- [x] 3.1 运行 `openspec validate split-nacos-config-source-client --strict`。
- [x] 3.2 运行 `cd common && go test ./runtime/config/nacos`。
- [x] 3.3 运行 `cd common && go test -race ./runtime/config/nacos`。
- [x] 3.4 运行 `cd common && go vet ./runtime/config/nacos`。
- [x] 3.5 按范围运行必要的 common 或架构门禁；无法完成整仓库验证时说明原因。
