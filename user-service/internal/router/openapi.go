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

func serveOpenAPIIndex(c *gin.Context) {
	index, err := fs.ReadFile(swaggerFiles.FS, "index.html")
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}

	content := strings.ReplaceAll(string(index), "https://petstore.swagger.io/v2/swagger.json", openAPIJSONPath)
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(content))
}

func serveOpenAPIInitializer(c *gin.Context) {
	initializer, err := fs.ReadFile(swaggerFiles.FS, "swagger-initializer.js")
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}

	content := strings.ReplaceAll(string(initializer), "https://petstore.swagger.io/v2/swagger.json", openAPIJSONPath)
	c.Data(http.StatusOK, "application/javascript; charset=utf-8", []byte(content))
}

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

func serveOpenAPI(c *gin.Context) {
	c.Data(http.StatusOK, "application/json; charset=utf-8", docs.ReadOpenAPI())
}

func redirectToOpenAPI(c *gin.Context) {
	c.Redirect(http.StatusMovedPermanently, openAPIUIPath)
}

func openAPIEnabled(environment string) bool {
	switch strings.ToLower(strings.TrimSpace(environment)) {
	case "prod", "production":
		// 生产环境默认关闭 OpenAPI 文档路由。
		return false
	default:
		return true
	}
}
