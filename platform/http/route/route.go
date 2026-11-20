package route

import "github.com/gin-gonic/gin"

// Unmatched 是未匹配到 Gin 路由模板时使用的低基数占位值。
const Unmatched = "__unmatched__"

// TemplateOrUnmatched 返回 Gin 路由模板；未匹配路由统一返回低基数固定值。
func TemplateOrUnmatched(c *gin.Context) string {
	if c == nil {
		return Unmatched
	}
	if path := c.FullPath(); path != "" {
		return path
	}
	return Unmatched
}
