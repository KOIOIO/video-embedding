package handler

import (
	"context"
	"net/http"
	"path"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"video-service/internal/application/knowledgevideo"
	objecthandler "video-service/internal/http/handler/objects"
)

type knowledgeVideoMediaResolver interface {
	GetVideo(ctx context.Context, id uint64) (knowledgevideo.Video, bool, error)
}

type KnowledgeVideoMediaHandler struct {
	resolver knowledgeVideoMediaResolver
	proxy    *objecthandler.Handler
}

func NewKnowledgeVideoMediaHandler(resolver knowledgeVideoMediaResolver, store objecthandler.Reader) *KnowledgeVideoMediaHandler {
	return &KnowledgeVideoMediaHandler{resolver: resolver, proxy: objecthandler.New(store)}
}

// Proxy godoc
// @Summary 获取知识点视频 HLS 媒体
// @Description 返回 master.m3u8、媒体播放清单或视频分片，支持 Range 请求。
// @Tags 媒体访问
// @Produce application/vnd.apple.mpegurl,video/mp2t,application/octet-stream
// @Param videoId path int true "知识视频ID"
// @Param filepath path string true "HLS 相对文件路径"
// @Success 200 {file} file
// @Success 206 {file} file
// @Failure 400 {string} string
// @Failure 404 {string} string
// @Router /knowledge-video-media/hls/{videoId}/{filepath} [get]
func (h *KnowledgeVideoMediaHandler) Proxy(c *gin.Context) {
	if h == nil || h.resolver == nil || h.proxy == nil {
		c.Status(http.StatusNotFound)
		return
	}
	videoID, err := strconv.ParseUint(c.Param("videoId"), 10, 64)
	if err != nil || videoID == 0 {
		c.Status(http.StatusBadRequest)
		return
	}
	relative := strings.TrimPrefix(c.Param("filepath"), "/")
	clean := path.Clean(relative)
	if relative == "" || clean == "." || clean == ".." || strings.HasPrefix(clean, "../") || strings.HasPrefix(relative, "/") || strings.Contains(relative, "\\") || containsParentSegment(relative) {
		c.Status(http.StatusBadRequest)
		return
	}
	video, found, err := h.resolver.GetVideo(c, videoID)
	if err != nil || !found || video.Status != knowledgevideo.VideoReady {
		c.Status(http.StatusNotFound)
		return
	}
	prefix := strings.Trim(video.HLSObjectPrefix, "/")
	if prefix == "" {
		c.Status(http.StatusNotFound)
		return
	}
	for index := range c.Params {
		if c.Params[index].Key == "filepath" {
			c.Params[index].Value = "/" + prefix + "/" + clean
		}
	}
	h.proxy.ProxyVideo(c)
}

func containsParentSegment(value string) bool {
	for _, segment := range strings.Split(value, "/") {
		if segment == ".." {
			return true
		}
	}
	return false
}
