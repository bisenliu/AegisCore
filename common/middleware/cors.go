package middleware

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

const (
	HeaderAccessControlAllowOrigin      = "Access-Control-Allow-Origin"
	HeaderAccessControlAllowMethods     = "Access-Control-Allow-Methods"
	HeaderAccessControlAllowHeaders     = "Access-Control-Allow-Headers"
	HeaderAccessControlExposeHeaders    = "Access-Control-Expose-Headers"
	HeaderAccessControlAllowCredentials = "Access-Control-Allow-Credentials"
	HeaderAccessControlMaxAge           = "Access-Control-Max-Age"
	HeaderOrigin                        = "Origin"
	HeaderVary                          = "Vary"
)

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
	AllowedOrigins: []string{"*"},
	AllowedMethods: []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodOptions},
	AllowedHeaders: []string{"Authorization", "Content-Type", HeaderTraceID},
}

func CORS() gin.HandlerFunc {
	return CORSWithOptions(defaultCORSOptions)
}

func CORSWithOptions(options CORSOptions) gin.HandlerFunc {
	options = normalizeCORSOptions(options)
	return func(c *gin.Context) {
		origin := strings.Join(options.AllowedOrigins, ",")
		if options.ReflectOrigin {
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
