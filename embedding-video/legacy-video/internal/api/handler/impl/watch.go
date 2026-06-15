package impl

import (
	"net/http"

	"legacy-video/internal/api/client"
	"legacy-video/video"

	"github.com/gin-gonic/gin"
)

// reportWatchRequest 描述观看记录上报接口的请求体。
type reportWatchRequest struct {
	QuestionID     uint64 `json:"question_id"`
	UserID         uint64 `json:"user_id"`
	VideoSegmentID uint64 `json:"video_segment_id"`
	IsWatched      bool   `json:"is_watched"`
	WatchDuration  int32  `json:"watch_duration"`
}

// ReportWatch 上报用户对推荐视频片段的观看行为。
func ReportWatch(c *gin.Context) {
	var req reportWatchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "请求体非法: " + err.Error(),
		})
		return
	}

	resp, err := client.GetVideoClient().ReportWatch(c, &video.ReportWatchRequest{
		QuestionId:     req.QuestionID,
		UserId:         req.UserID,
		VideoSegmentId: req.VideoSegmentID,
		IsWatched:      req.IsWatched,
		WatchDuration:  req.WatchDuration,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "上报失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": resp.GetSuccess(),
		"message": resp.GetMessage(),
	})
}
