package middleware

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/aegiscore/common/security/auth"
)

const (
	// HeaderAccessControlAllowOrigin 是声明允许来源的 CORS 响应头。
	HeaderAccessControlAllowOrigin = "Access-Control-Allow-Origin"
	// HeaderAccessControlAllowMethods 是声明允许方法的 CORS 响应头。
	HeaderAccessControlAllowMethods = "Access-Control-Allow-Methods"
	// HeaderAccessControlAllowHeaders 是声明允许请求头的 CORS 响应头。
	HeaderAccessControlAllowHeaders = "Access-Control-Allow-Headers"
	// HeaderAccessControlExposeHeaders 是声明浏览器可读取响应头的 CORS 响应头。
	HeaderAccessControlExposeHeaders = "Access-Control-Expose-Headers"
	// HeaderAccessControlAllowCredentials 是声明是否允许携带凭据的 CORS 响应头。
	HeaderAccessControlAllowCredentials = "Access-Control-Allow-Credentials"
	// HeaderAccessControlMaxAge 是声明预检缓存时长的 CORS 响应头。
	HeaderAccessControlMaxAge = "Access-Control-Max-Age"
	// HeaderOrigin 是携带浏览器来源的请求头。
	HeaderOrigin = "Origin"
	// HeaderVary 是启用来源反射时保护缓存正确性的响应头。
	HeaderVary = "Vary"
)

// CORSOptions 配置 CORS 中间件策略。
type CORSOptions struct {
	AllowedOrigins   []string
	AllowedMethods   []string
	AllowedHeaders   []string
	ExposedHeaders   []string
	AllowCredentials bool
	MaxAgeSeconds    int
	ReflectOrigin    bool
}

var defaultCORSOptions = CORSOptions{
	// 共享默认策略对服务内 API 保持宽松，部署环境可传入更严格的选项。
	AllowedOrigins: []string{"*"},
	AllowedMethods: []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodOptions},
	AllowedHeaders: []string{auth.AuthorizationHeader, "Content-Type", HeaderTraceID},
}

// CORS 返回使用包默认 CORS 策略的中间件。
func CORS() gin.HandlerFunc {
	return CORSWithOptions(defaultCORSOptions)
}

// CORSWithOptions 返回使用调用方自定义 CORS 策略的中间件。
func CORSWithOptions(options CORSOptions) gin.HandlerFunc {
	options = normalizeCORSOptions(options)
	return func(c *gin.Context) {
		origin := strings.Join(options.AllowedOrigins, ",")
		if options.ReflectOrigin {
			// 来源反射必须按 Origin 区分缓存，否则一个调用方的反射值可能泄露给另一个调用方。
			origin = c.GetHeader(HeaderOrigin)
			c.Header(HeaderVary, HeaderOrigin)
		}
		c.Header(HeaderAccessControlAllowOrigin, origin)
		c.Header(HeaderAccessControlAllowMethods, strings.Join(options.AllowedMethods, ","))
		c.Header(HeaderAccessControlAllowHeaders, strings.Join(options.AllowedHeaders, ","))
		if len(options.ExposedHeaders) > 0 {
			c.Header(HeaderAccessControlExposeHeaders, strings.Join(options.ExposedHeaders, ","))
		}
		if options.AllowCredentials {
			c.Header(HeaderAccessControlAllowCredentials, "true")
		}
		if options.MaxAgeSeconds > 0 {
			c.Header(HeaderAccessControlMaxAge, strconv.Itoa(options.MaxAgeSeconds))
		}
		if c.Request.Method == http.MethodOptions {
			// 预检请求由中间件直接响应，不能继续进入业务 handler。
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

func normalizeCORSOptions(options CORSOptions) CORSOptions {
	if len(options.AllowedOrigins) == 0 {
		options.AllowedOrigins = defaultCORSOptions.AllowedOrigins
	}
	if len(options.AllowedMethods) == 0 {
		options.AllowedMethods = defaultCORSOptions.AllowedMethods
	}
	if len(options.AllowedHeaders) == 0 {
		options.AllowedHeaders = defaultCORSOptions.AllowedHeaders
	}
	return options
}
