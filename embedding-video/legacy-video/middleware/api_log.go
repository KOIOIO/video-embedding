package middleware

import (
	"os"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

var (
	apiLogOnce sync.Once
	apiLogFile *os.File
)

// ensureAPILogger 确保 API 层日志已切换到文件与标准输出双写。
func ensureAPILogger() {
	apiLogOnce.Do(func() {
		f, err := InitFileLogger("api")
		if err != nil {
			return
		}
		apiLogFile = f
	})
}

// AccessLogMiddleware 记录每个 HTTP 请求的基本访问信息与处理耗时。
func AccessLogMiddleware() gin.HandlerFunc {
	ensureAPILogger()
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		latency := time.Since(start)
		status := c.Writer.Status()
		method := c.Request.Method
		path := c.Request.URL.Path
		rawQuery := c.Request.URL.RawQuery
		clientIP := c.ClientIP()
		errMsg := c.Errors.ByType(gin.ErrorTypePrivate).String()

		if rawQuery != "" {
			path = path + "?" + rawQuery
		}

		zap.L().Info("api_access",
			zap.Int("status", status),
			zap.String("method", method),
			zap.String("path", path),
			zap.String("ip", clientIP),
			zap.Int64("latency_ms", latency.Milliseconds()),
			zap.String("err", errMsg),
		)
	}
}
