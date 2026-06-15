package impl

import (
	"net/http"
	"strconv"

	"legacy-video/internal/api/client"
	"legacy-video/video"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc/status"
)

// ListVideos 获取视频列表API
func ListVideos(c *gin.Context) {
	// 获取过滤类型
	filterType := c.DefaultQuery("type", "ALL")
	var videoType video.VideoType

	switch filterType {
	case "RAW":
		videoType = video.VideoType_RAW
	case "HLS":
		videoType = video.VideoType_HLS
	default:
		videoType = video.VideoType_ALL
	}

	// 调用gRPC服务
	response, err := client.GetVideoClient().ListVideos(c, &video.ListVideosRequest{
		FilterType: videoType,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "获取视频列表失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"videos":  response.Videos,
		"total":   response.Total,
	})
}

// GetTranscodeStatus 获取转码状态API
func GetTranscodeStatus(c *gin.Context) {
	// 获取任务ID
	taskId := c.Param("taskId")
	if taskId == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "任务ID不能为空",
		})
		return
	}

	// 调用gRPC服务
	response, err := client.GetVideoClient().GetTranscodeStatus(c, &video.GetTranscodeStatusRequest{
		TaskId: taskId,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "获取转码状态失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"task_id": response.TaskId,
		"status":  response.Status.String(),
		"message": response.Message,
		"hls_url": response.HlsUrl,
	})
}

func DeleteVideo(c *gin.Context) {
	id, ok := parseUint64Param(c, "id")
	if !ok || id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "id 非法",
		})
		return
	}

	resp, err := client.GetVideoClient().DeleteVideo(c, &video.DeleteVideoRequest{VideoId: id})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "删除失败: " + err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": resp.Success,
		"message": resp.Message,
	})
}

func PlayVideo(c *gin.Context) {
	id, ok := parseUint64Param(c, "id")
	if !ok || id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "id 非法",
		})
		return
	}

	resp, err := client.GetVideoClient().PlayVideo(c, &video.PlayVideoRequest{VideoId: id})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "播放失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":  true,
		"play_url": resp.PlayUrl,
		"video":    resp.Video,
	})
}

func GetSimilarVideos(c *gin.Context) {
	id, ok := parseUint64Param(c, "id")
	if !ok || id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "id 非法",
		})
		return
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "6"))
	resp, err := client.GetVideoClient().GetSimilarVideos(c, &video.GetSimilarVideosRequest{
		VideoId: id,
		Limit:   int32(limit),
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "获取相近视频失败: " + err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"videos":  resp.Videos,
		"total":   resp.Total,
	})
}

func GetViewCount(c *gin.Context) {
	id, ok := parseUint64Param(c, "id")
	if !ok || id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "id 非法",
		})
		return
	}

	resp, err := client.GetVideoClient().GetViewCount(c, &video.GetViewCountRequest{VideoId: id})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "获取观看次数失败: " + err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"video_id":   resp.VideoId,
		"view_count": resp.ViewCount,
	})
}

// ListRecommendPoolVideos 处理推荐池视频列表查询。
func ListRecommendPoolVideos(c *gin.Context) {
	resp, err := client.GetVideoClient().ListRecommendPoolVideos(c, &video.ListRecommendPoolVideosRequest{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "获取推荐池列表失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"videos":  resp.Videos,
		"total":   resp.Total,
	})
}

// publishRequest 描述发布状态切换接口的请求体。
type publishRequest struct {
	IsPublished bool `json:"is_published"`
}

// recommendRequest 描述推荐状态切换接口的请求体。
type recommendRequest struct {
	IsRecommend bool `json:"is_recommend"`
}

// updateVideoMetadataRequest 描述视频标题和描述更新接口的请求体。
type updateVideoMetadataRequest struct {
	Title       string `json:"title" binding:"required,max=200"`
	Description string `json:"description" binding:"max=5000"`
}

// UpdateVideoMetadata 处理视频标题和描述更新请求。
func UpdateVideoMetadata(c *gin.Context) {
	id, ok := parseUint64Param(c, "id")
	if !ok || id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "id 非法",
		})
		return
	}

	var req updateVideoMetadataRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "请求体非法: " + err.Error(),
		})
		return
	}

	resp, err := client.GetVideoClient().UpdateVideoMetadata(c, &video.UpdateVideoMetadataRequest{
		VideoId:     id,
		Title:       req.Title,
		Description: req.Description,
	})
	if err != nil {
		switch status.Code(err) {
		case 5:
			c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "视频不存在或已删除"})
		case 3:
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "参数非法: " + err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "更新视频信息失败: " + err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":     resp.Success,
		"message":     resp.Message,
		"video_id":    resp.VideoId,
		"title":       resp.Title,
		"description": resp.Description,
	})
}

// SetVideoPublished 处理发布状态切换请求。
func SetVideoPublished(c *gin.Context) {
	id, ok := parseUint64Param(c, "id")
	if !ok || id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "id 非法",
		})
		return
	}

	var req publishRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "请求体非法: " + err.Error(),
		})
		return
	}

	resp, err := client.GetVideoClient().SetVideoPublished(c, &video.SetVideoPublishedRequest{
		VideoId:     id,
		IsPublished: req.IsPublished,
	})
	if err != nil {
		switch status.Code(err) {
		case 5:
			c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "视频不存在或已删除"})
		case 3:
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "参数非法: " + err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "设置发布状态失败: " + err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":      resp.Success,
		"message":      resp.Message,
		"video_id":     resp.VideoId,
		"is_published": resp.IsPublished,
	})
}

// SetVideoRecommend 处理推荐池状态切换请求。
// 当前 API 层会补一组测试参数，便于前端联调推荐池功能。
func SetVideoRecommend(c *gin.Context) {
	id, ok := parseUint64Param(c, "id")
	if !ok || id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "id 非法",
		})
		return
	}

	var req recommendRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "请求体非法: " + err.Error(),
		})
		return
	}

	resp, err := client.GetVideoClient().SetVideoRecommend(c, &video.SetVideoRecommendRequest{
		VideoId:        id,
		IsRecommend:    req.IsRecommend,
		UserId:         1,
		RecommendLevel: 1,
		RecommendScore: 0.99,
	})
	if err != nil {
		switch status.Code(err) {
		case 5:
			c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "视频不存在或已删除"})
		case 3:
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "参数非法: " + err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "设置推荐池失败: " + err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":      resp.Success,
		"message":      resp.Message,
		"video_id":     resp.VideoId,
		"is_recommend": resp.IsRecommend,
	})
}
