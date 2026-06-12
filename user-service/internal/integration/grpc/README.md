# gRPC Integration

`integration/grpc` 用于用户服务访问外部 gRPC service 的 client adapter。

可以放置：

- generated gRPC client 的薄包装。
- metadata、deadline、status code 和外部错误语义映射。
- protobuf DTO 与 feature application command/result 或 domain 值对象的转换。
- 对 feature application port 的实现。

禁止放置：

- 本服务 gRPC server 入口或 route 层逻辑。
- 未被真实外部调用使用的 protobuf schema 或 generated code。
- feature service 业务编排、跨 store 事务或 HTTP 错误响应映射。

当前没有真实外部 gRPC 调用，因此本目录只保留 README 占位。
