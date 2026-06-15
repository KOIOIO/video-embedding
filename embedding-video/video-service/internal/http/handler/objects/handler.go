package objects

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/minio/minio-go/v7"

	"nlp-video-analysis/internal/infrastructure/objectstorage"
)

type Reader interface {
	Stat(ctx context.Context, objectKey string) (objectstorage.ObjectInfo, error)
	Open(ctx context.Context, objectKey string, opts minio.GetObjectOptions) (io.ReadCloser, error)
}

type Handler struct {
	store    Reader
	statTTL  time.Duration
	statMu   sync.Mutex
	statByID map[string]cachedObjectStat
}

type cachedObjectStat struct {
	info      objectstorage.ObjectInfo
	expiresAt time.Time
}

const defaultObjectStatCacheTTL = 30 * time.Second

func New(store Reader) *Handler {
	return &Handler{
		store:    store,
		statTTL:  defaultObjectStatCacheTTL,
		statByID: make(map[string]cachedObjectStat),
	}
}

func (h *Handler) ProxyVideo(c *gin.Context) {
	if h == nil || h.store == nil {
		c.Status(http.StatusNotFound)
		return
	}

	key := strings.TrimPrefix(c.Param("filepath"), "/")
	if key == "" {
		c.Status(http.StatusNotFound)
		return
	}

	st, err := h.stat(c, key)
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	h.setCacheHeaders(c, key, st)
	if matchesETag(c.GetHeader("If-None-Match"), st.ETag) {
		c.Status(http.StatusNotModified)
		return
	}

	var opts minio.GetObjectOptions
	if r, ok := objectstorage.ParseRangeHeader(c.GetHeader("Range"), st.Size); ok {
		_ = opts.SetRange(r.Start, r.End)
		c.Header("Accept-Ranges", "bytes")
		c.Header("Content-Range", fmt.Sprintf("bytes %d-%d/%d", r.Start, r.End, st.Size))
		c.Header("Content-Length", fmt.Sprintf("%d", (r.End-r.Start)+1))
		c.Header("Content-Type", st.ContentType)
		c.Status(http.StatusPartialContent)
		obj, err := h.store.Open(c, key, opts)
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
	obj, err := h.store.Open(c, key, opts)
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	_ = objectstorage.CopyAndClose(c.Writer, obj)
}

func (h *Handler) stat(ctx context.Context, key string) (objectstorage.ObjectInfo, error) {
	if h.statTTL > 0 {
		now := time.Now()
		h.statMu.Lock()
		cached, ok := h.statByID[key]
		if ok && now.Before(cached.expiresAt) {
			h.statMu.Unlock()
			return cached.info, nil
		}
		h.statMu.Unlock()
	}

	info, err := h.store.Stat(ctx, key)
	if err != nil {
		return objectstorage.ObjectInfo{}, err
	}
	if h.statTTL > 0 {
		h.statMu.Lock()
		h.statByID[key] = cachedObjectStat{info: info, expiresAt: time.Now().Add(h.statTTL)}
		h.statMu.Unlock()
	}
	return info, nil
}

func (h *Handler) setCacheHeaders(c *gin.Context, key string, st objectstorage.ObjectInfo) {
	c.Header("Cache-Control", cacheControlForObject(key))
	if etag := quoteETag(st.ETag); etag != "" {
		c.Header("ETag", etag)
	}
}

func cacheControlForObject(key string) string {
	switch strings.ToLower(filepath.Ext(key)) {
	case ".ts", ".m4s", ".mp4", ".jpg", ".jpeg", ".png", ".webp", ".gif":
		return "public, max-age=31536000, immutable"
	case ".m3u8":
		return "public, max-age=60"
	default:
		return "public, max-age=3600"
	}
}

func matchesETag(header string, etag string) bool {
	header = strings.TrimSpace(header)
	if header == "" {
		return false
	}
	quoted := quoteETag(etag)
	if quoted == "" {
		return false
	}
	for _, part := range strings.Split(header, ",") {
		value := strings.TrimSpace(part)
		if value == "*" || value == quoted {
			return true
		}
	}
	return false
}

func quoteETag(etag string) string {
	etag = strings.TrimSpace(etag)
	if etag == "" {
		return ""
	}
	if strings.HasPrefix(etag, `"`) && strings.HasSuffix(etag, `"`) {
		return etag
	}
	return `"` + etag + `"`
}
