package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

type CORSOptions struct {
	AllowOrigin   string
	AllowMethods  string
	AllowHeaders  string
	ExposeHeaders string
	MaxAge        string
}

// CORSMiddleware 为浏览器跨域访问补充统一响应头，并直接处理 OPTIONS 预检请求。
func CORSMiddleware() gin.HandlerFunc {
	return CORSMiddlewareWithOptions(CORSOptions{})
}

func CORSMiddlewareWithOptions(opts CORSOptions) gin.HandlerFunc {
	allowOrigin := defaultHeaderValue(opts.AllowOrigin, "*")
	allowMethods := defaultHeaderValue(opts.AllowMethods, "GET, POST, PUT, PATCH, DELETE, OPTIONS")
	allowHeaders := defaultHeaderValue(opts.AllowHeaders, "Origin, Content-Type, Accept, Authorization, X-Requested-With")
	exposeHeaders := defaultHeaderValue(opts.ExposeHeaders, "Content-Length, Content-Type")
	maxAge := defaultHeaderValue(opts.MaxAge, "86400")
	return func(c *gin.Context) {
		header := c.Writer.Header()
		header.Set("Access-Control-Allow-Origin", allowOrigin)
		header.Set("Access-Control-Allow-Methods", allowMethods)
		header.Set("Access-Control-Allow-Headers", allowHeaders)
		header.Set("Access-Control-Expose-Headers", exposeHeaders)
		header.Set("Access-Control-Max-Age", maxAge)

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

func defaultHeaderValue(value string, fallback string) string {
	value = strings.TrimSpace(value)
	if value != "" {
		return value
	}
	return fallback
}
