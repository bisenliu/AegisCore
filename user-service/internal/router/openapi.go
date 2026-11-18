package router

import (
	"io/fs"
	"mime"
	"net/http"
	"path"
	"strings"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files/v2"

	"github.com/aegiscore/user-service/docs"
)

const (
	openAPIJSONPath = "/openapi.json"
	openAPIUIPath   = "/openapi/index.html"
)

// RegisterOpenAPI 按条件挂载 OpenAPI UI、JSON 文档和文档重定向路由。
func RegisterOpenAPI(engine *gin.Engine, environment string) {
	if !openAPIEnabled(environment) {
		return
	}

	engine.GET(openAPIJSONPath, serveOpenAPI)
	engine.GET("/openapi/*any", serveOpenAPIUI)
	engine.GET("/docs", redirectToOpenAPI)
	engine.GET("/api-docs", redirectToOpenAPI)
}

// serveOpenAPIUI 展示可视化的接口在线测试与文档页面。
func serveOpenAPIUI(c *gin.Context) {
	// 清理并限制 swagger 静态资源路径，阻止 ../ 逃逸；index 和 initializer 会替换默认 petstore 文档地址。
	assetPath := path.Clean(strings.TrimPrefix(c.Param("any"), "/"))
	if assetPath == "." {
		assetPath = "index.html"
	}
	if strings.HasPrefix(assetPath, "../") {
		c.Status(http.StatusNotFound)
		return
	}
	if assetPath == "index.html" {
		serveOpenAPIIndex(c)
		return
	}
	if assetPath == "swagger-initializer.js" {
		serveOpenAPIInitializer(c)
		return
	}

	serveOpenAPIAsset(c, assetPath)
}

// serveOpenAPIIndex 提供接口文档页面的 HTML 首页内容。
func serveOpenAPIIndex(c *gin.Context) {
	index, err := fs.ReadFile(swaggerFiles.FS, "index.html")
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}

	content := strings.ReplaceAll(string(index), "https://petstore.swagger.io/v2/swagger.json", openAPIJSONPath)
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(content))
}

// serveOpenAPIInitializer 提供接口文档页面初始化配置脚本。
func serveOpenAPIInitializer(c *gin.Context) {
	initializer, err := fs.ReadFile(swaggerFiles.FS, "swagger-initializer.js")
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}

	content := strings.ReplaceAll(string(initializer), "https://petstore.swagger.io/v2/swagger.json", openAPIJSONPath)
	c.Data(http.StatusOK, "application/javascript; charset=utf-8", []byte(content))
}

// serveOpenAPIAsset 提供接口文档所需的静态资源文件（如样式表、图标）；防止非法路径跳出静态资源目录。
func serveOpenAPIAsset(c *gin.Context, assetPath string) {
	asset, err := fs.ReadFile(swaggerFiles.FS, assetPath)
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}

	contentType := mime.TypeByExtension(path.Ext(assetPath))
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	c.Data(http.StatusOK, contentType, asset)
}

// serveOpenAPI 对外输出系统自动生成的 OpenAPI 规范标准接口描述内容（JSON格式）。
func serveOpenAPI(c *gin.Context) {
	c.Data(http.StatusOK, "application/json; charset=utf-8", docs.ReadOpenAPI())
}

// redirectToOpenAPI 将访问文档根路径的用户自动引导跳转到文档首页。
func redirectToOpenAPI(c *gin.Context) {
	c.Redirect(http.StatusMovedPermanently, openAPIUIPath)
}

// openAPIEnabled 判断当前运行环境是否允许对外公开接口文档。
func openAPIEnabled(environment string) bool {
	switch strings.ToLower(strings.TrimSpace(environment)) {
	case "prod", "production":
		// 生产环境默认关闭 OpenAPI 文档路由。
		return false
	default:
		return true
	}
}
