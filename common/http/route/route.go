package route

import "github.com/gin-gonic/gin"

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
