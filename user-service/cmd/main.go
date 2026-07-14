package main

// @title AegisCore User Services API
// @version 1.0.0
// @description AegisCore 用户服务 API 文档，覆盖认证会话、用户资料、角色管理、权限目录、RBAC 授权保护的业务接口和服务健康检查。
// @host localhost:8080
// @BasePath /api/v1
// @schemes http https
// @accept json
// @produce json
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description 输入 Bearer token，格式为：Bearer <token>

import (
	"fmt"
	"os"
)

func main() {
	if err := newRootCommand(defaultRootCommandDependencies()).Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
