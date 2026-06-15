package impl

import (
	"net/http"
	"strings"

	"legacy-video/internal/api/client"
	"legacy-video/video"

	"github.com/gin-gonic/gin"
)

// recommendByQuestionRequest 描述题目检索推荐接口的请求体。
type recommendByQuestionRequest struct {
	QuestionID   uint64 `json:"question_id"`
	QuestionText string `json:"question_text"`
	UserID       uint64 `json:"user_id"`
	Limit        int32  `json:"limit"`
}

// RecommendByQuestion 调用推荐接口，并为每个命中的视频补一个播放地址。
func RecommendByQuestion(c *gin.Context) {
	var req recommendByQuestionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "请求体非法: " + err.Error(),
		})
		return
	}
	qt := strings.TrimSpace(req.QuestionText)
	if qt == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "question_text 不能为空",
		})
		return
	}
	limit := req.Limit
	if limit <= 0 {
		limit = 3
	}

	reco, err := client.GetVideoClient().RecommendByQuestion(c, &video.RecommendByQuestionRequest{
		QuestionId:   req.QuestionID,
		QuestionText: qt,
		UserId:       req.UserID,
		Limit:        limit,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "推荐失败: " + err.Error(),
		})
		return
	}

	// 同一视频可能匹配多个片段，这里缓存 PlayVideo 结果，避免重复 RPC。
	playURLCache := make(map[uint64]string, 8)
	items := make([]gin.H, 0, len(reco.Items))
	for _, it := range reco.Items {
		videoID := it.GetVideoId()
		playURL := playURLCache[videoID]
		if playURL == "" {
			pr, err := client.GetVideoClient().PlayVideo(c, &video.PlayVideoRequest{VideoId: videoID})
			if err == nil {
				playURL = pr.GetPlayUrl()
			}
			playURLCache[videoID] = playURL
		}

		title := ""
		coverURL := ""
		if it.GetVideo() != nil {
			title = it.GetVideo().GetName()
			coverURL = it.GetVideo().GetCoverUrl()
		}

		items = append(items, gin.H{
			"question_id":      it.GetQuestionId(),
			"video_id":         videoID,
			"video_segment_id": it.GetVideoSegmentId(),
			"recommend_score":  it.GetRecommendScore(),
			"is_watched":       it.GetIsWatched(),
			"watch_duration":   it.GetWatchDuration(),
			"start_time_sec":   it.GetStartTimeSec(),
			"end_time_sec":     it.GetEndTimeSec(),
			"title":            title,
			"cover_url":        coverURL,
			"play_url":         playURL,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"total":   len(items),
		"items":   items,
	})
}
