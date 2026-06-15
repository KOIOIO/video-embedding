package impl

import (
	"strconv"

	"github.com/gin-gonic/gin"
)

// parseUint64Param 读取路径参数并解析成 uint64。
func parseUint64Param(c *gin.Context, key string) (uint64, bool) {
	s := c.Param(key)
	if s == "" {
		return 0, false
	}
	v, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return 0, false
	}
	return v, true
}
