# Integration Boundary

`user-service/internal/integration` 是用户服务访问外部系统的防腐层边界。这里承载外部协议、传输细节、外部 DTO、外部错误归一化和 client adapter；不承载 feature 业务编排。

## 子目录

- `http/`：外部 HTTP API client adapter。
- `grpc/`：外部 gRPC service client adapter。
- `events/`：外部事件系统 producer/consumer adapter。

## 规则

- Feature application service 仍通过自己拥有的 ports 表达外部能力需求，例如 `internal/features/<feature>/application/ports.go`。
- Integration adapter 只在确有真实外部系统调用时实现 feature application port。
- 不要在这里定义为了 adapter 方便而扩张的大接口。
- 不要在这里实现登录状态机、跨 store 事务、HTTP controller、Gin response 输出或本服务持久化访问。
- 不要把尚不存在的外部系统 client、broker dependency 或 generated code 预先放进来。

当前没有真实外部系统调用，因此本目录只保留边界说明。
