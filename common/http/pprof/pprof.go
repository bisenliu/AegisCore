// Package pprof 提供 Gin 版本的 Go runtime pprof 路由注册能力。
//
// 该包只负责将 Go 原生的性能分析接口（pprof）挂载到 HTTP 路由上，不包含任何业务代码，也不决定是否开启、监听地址、
// 访问白名单或网关暴露策略；具体的安全控制策略应由各服务通过配置文件和运维部署策略决定。
package pprof

import (
	"net/http"
	stdpprof "net/http/pprof"
	"path"
	"strings"

	"github.com/gin-gonic/gin"
)

const defaultBasePath = "/debug/pprof"

// Options 配置 pprof 路由注册方式。
type Options struct {
	// BasePath 是 pprof 路由根路径；为空时使用 /debug/pprof。
	//
	// 接入示例：
	//
	//	engine := gin.New()
	//	pprof.Register(engine, pprof.Options{BasePath: "/debug/pprof"})
	//
	// 注册后可通过以下地址采集 profile：
	//
	//	go tool pprof http://127.0.0.1:8080/debug/pprof/heap
	//	go tool pprof http://127.0.0.1:8080/debug/pprof/profile?seconds=30
	BasePath string
}

// Register 将 Go 标准库 net/http/pprof 的诊断端点挂载到 Gin router。
//
// 该方法会注册 <base> 和 <base>/*profile 两类 GET 路由，并在 handler
// 内部分发 /cmdline、/profile、/symbol、/trace 以及 heap、goroutine 等
// runtime profile 名称。
//
// pprof 端点会暴露进程命令行、goroutine、heap、CPU profile 和 trace 等
// 运行时诊断信息。服务接入时应默认关闭或仅在受控网络中开启，并在服务侧
// 决定是否叠加鉴权、独立 debug 端口、网关规则或 IP allowlist。
func Register(router gin.IRoutes, opts Options) {
	basePath := normalizeBasePath(opts.BasePath)
	handler := dispatchHandler()
	router.GET(basePath, redirectToIndex(basePath))
	router.GET(basePath+"/*profile", handler)
}

// Handler 返回已注册 pprof 路由的 http.Handler，供独立 debug server 复用。
//
// 接入示例：
//
//	handler := pprof.Handler(pprof.Options{BasePath: "/debug/pprof"})
//	go http.ListenAndServe("127.0.0.1:6060", handler)
//
// 生产环境如使用独立 debug server，应只绑定内网或 localhost，并由部署层限制访问。
func Handler(opts Options) http.Handler {
	engine := gin.New()
	Register(engine, opts)
	return engine
}

// redirectToIndex 将访问系统性能监控目录的请求自动重定向到首页，避免直接打开时显示空白页面。
func redirectToIndex(basePath string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, basePath+"/")
	}
}

// dispatchHandler 根据请求的具体监控项目（例如查看内存占用、CPU 繁忙程度），分发展示对应的系统性能诊断内容。
func dispatchHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		profile := strings.Trim(path.Clean("/"+c.Param("profile")), "/")
		switch profile {
		case "":
			stdpprof.Index(c.Writer, c.Request)
		case "cmdline":
			stdpprof.Cmdline(c.Writer, c.Request)
		case "profile":
			stdpprof.Profile(c.Writer, c.Request)
		case "symbol":
			stdpprof.Symbol(c.Writer, c.Request)
		case "trace":
			stdpprof.Trace(c.Writer, c.Request)
		default:
			stdpprof.Handler(profile).ServeHTTP(c.Writer, c.Request)
		}
	}
}

// normalizeBasePath 规范化性能诊断入口地址，确保以斜杠开头且末尾不带多余斜杠，避免地址拼接错误。
func normalizeBasePath(basePath string) string {
	path := strings.TrimSpace(basePath)
	if path == "" {
		return defaultBasePath
	}
	path = "/" + strings.Trim(path, "/")
	if path == "/" {
		return defaultBasePath
	}
	return path
}
