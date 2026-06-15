package middleware

import (
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	apiLogOnce sync.Once
	apiLogFile *os.File
)

type FileLoggerOptions struct {
	LogDir string
}

type AccessLogOptions struct {
	LogDir               string
	SlowRequestThreshold time.Duration
}

// InitFileLogger 初始化服务日志输出，并把 log 与 zap 同时接到相同输出目标。
func InitFileLogger(serviceName string) (*os.File, error) {
	return InitFileLoggerWithOptions(serviceName, FileLoggerOptions{})
}

// InitFileLoggerWithOptions 初始化服务日志输出，并允许调用方配置日志目录。
func InitFileLoggerWithOptions(serviceName string, opts FileLoggerOptions) (*os.File, error) {
	if serviceName == "" {
		serviceName = "app"
	}
	logDir := strings.TrimSpace(opts.LogDir)
	if logDir == "" {
		logDir = "logs"
	}
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, err
	}
	logPath := filepath.Join(logDir, serviceName+".log")
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, err
	}

	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	log.SetOutput(io.MultiWriter(os.Stdout, f))
	_ = initZapLogger(io.MultiWriter(os.Stdout, f))
	return f, nil
}

func initZapLogger(out io.Writer) error {
	encCfg := zap.NewProductionEncoderConfig()
	encCfg.TimeKey = "ts"
	encCfg.EncodeTime = zapcore.ISO8601TimeEncoder

	core := zapcore.NewCore(
		zapcore.NewJSONEncoder(encCfg),
		zapcore.AddSync(out),
		zap.InfoLevel,
	)
	l := zap.New(core)
	zap.ReplaceGlobals(l)
	return nil
}

// ensureAPILogger 确保 API 层日志已切换到文件与标准输出双写。
func ensureAPILogger() {
	ensureAPILoggerWithOptions(AccessLogOptions{})
}

func ensureAPILoggerWithOptions(opts AccessLogOptions) {
	apiLogOnce.Do(func() {
		f, err := InitFileLoggerWithOptions("api", FileLoggerOptions{LogDir: opts.LogDir})
		if err != nil {
			return
		}
		apiLogFile = f
	})
}

// AccessLogMiddleware 记录每个 HTTP 请求的基本访问信息与处理耗时。
func AccessLogMiddleware() gin.HandlerFunc {
	return AccessLogMiddlewareWithOptions(AccessLogOptions{})
}

func AccessLogMiddlewareWithOptions(opts AccessLogOptions) gin.HandlerFunc {
	ensureAPILoggerWithOptions(opts)
	threshold := opts.SlowRequestThreshold
	if threshold <= 0 {
		threshold = time.Second
	}
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

		if !shouldLogAccess(path, status, latency, errMsg, threshold) {
			return
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

func shouldLogAccess(path string, status int, latency time.Duration, errMsg string, slowRequestThreshold time.Duration) bool {
	if status >= http.StatusBadRequest {
		return true
	}
	if latency >= slowRequestThreshold {
		return true
	}
	if strings.TrimSpace(errMsg) != "" {
		return true
	}
	if path == "/healthz" || path == "/api/healthz" {
		return false
	}
	if strings.HasPrefix(path, "/swagger/") {
		return false
	}
	return false
}
