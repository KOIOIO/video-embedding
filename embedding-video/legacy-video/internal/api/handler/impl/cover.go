package impl

import (
	"net/http"
	"path/filepath"
	"strconv"
	"time"

	"legacy-video/internal/api/client"
	"legacy-video/video"

	"github.com/gin-gonic/gin"
)

// RustfsStore 由外部设置
var RustfsStore interface {
	Put(ctx *gin.Context, key string, r interface{}, size int64, contentType string) error
}

// UploadVideoCover 由前端上传封面图，API 存 RustFS，然后通知 RPC 写入 cover_url
func UploadVideoCover(c *gin.Context) {
	if RustfsStore == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "message": "对象存储未初始化"})
		return
	}
	id, ok := parseUint64Param(c, "id")
	if !ok || id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "id 非法"})
		return
	}
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "缺少文件: " + err.Error()})
		return
	}
	defer file.Close()

	ext := filepath.Ext(header.Filename)
	if ext == "" {
		ext = ".jpg"
	}
	now := time.Now()
	datePath := now.Format("2006/01/02")
	ts := now.UnixNano()
	objectKey := filepath.ToSlash(filepath.Join("cover", datePath, "vid_"+strconv.FormatUint(id, 10)+"_"+strconv.FormatInt(ts, 10)+ext))
	ct := header.Header.Get("Content-Type")
	if err := RustfsStore.Put(c, objectKey, file, header.Size, ct); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "上传对象存储失败: " + err.Error()})
		return
	}
	coverURL := "/videos/" + objectKey

	resp, err := client.GetVideoClient().SetVideoCover(c, &video.SetVideoCoverRequest{
		VideoId:  id,
		CoverUrl: coverURL,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "更新封面失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success":   resp.Success,
		"message":   resp.Message,
		"video_id":  resp.VideoId,
		"cover_url": resp.CoverUrl,
	})
}
