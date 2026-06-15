package handler

import (
	"fmt"
	"net/http"
	"strings"

	"legacy-video/internal/infrastructure/objectstorage"

	"github.com/gin-gonic/gin"
	"github.com/minio/minio-go/v7"
)

var rustfsStore *objectstorage.RustFS

// InitRustFS 注入对象存储客户端，供视频与 HLS 代理路由复用。
func InitRustFS(store *objectstorage.RustFS) {
	rustfsStore = store
}

// ProxyVideo 直接代理对象存储中的媒体文件。
// 这里支持 Range 请求，便于 HLS 播放器和浏览器按需拉取内容。
func ProxyVideo(c *gin.Context) {
	if rustfsStore == nil {
		c.Status(http.StatusNotFound)
		return
	}

	fp := c.Param("filepath")
	key := strings.TrimPrefix(fp, "/")
	if key == "" {
		c.Status(http.StatusNotFound)
		return
	}

	st, err := rustfsStore.Stat(c, key)
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}

	var opts minio.GetObjectOptions
	rangeHeader := c.GetHeader("Range")
	if r, ok := objectstorage.ParseRangeHeader(rangeHeader, st.Size); ok {
		_ = opts.SetRange(r.Start, r.End)
		c.Header("Accept-Ranges", "bytes")
		c.Header("Content-Range", fmt.Sprintf("bytes %d-%d/%d", r.Start, r.End, st.Size))
		c.Header("Content-Length", fmt.Sprintf("%d", (r.End-r.Start)+1))
		c.Header("Content-Type", st.ContentType)
		c.Status(http.StatusPartialContent)
		obj, err := rustfsStore.Get(c, key, opts)
		if err != nil {
			c.Status(http.StatusNotFound)
			return
		}
		_ = objectstorage.CopyAndClose(c.Writer, obj)
		return
	}

	c.Header("Accept-Ranges", "bytes")
	if st.ContentType != "" {
		c.Header("Content-Type", st.ContentType)
	}
	c.Header("Content-Length", fmt.Sprintf("%d", st.Size))
	c.Status(http.StatusOK)
	obj, err := rustfsStore.Get(c, key, opts)
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	_ = objectstorage.CopyAndClose(c.Writer, obj)
}
