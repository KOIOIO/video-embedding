package knowledgevideos

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"video-service/internal/application/knowledgevideo"
	"video-service/internal/http/dto"
	httperrors "video-service/internal/http/errors"
	"video-service/middleware"
)

type Service interface {
	Import(ctx context.Context, input knowledgevideo.ImportInput) (knowledgevideo.ImportResult, error)
	GetBatch(ctx context.Context, batchID uint64) (knowledgevideo.Batch, []knowledgevideo.Video, bool, error)
	ListPlayback(ctx context.Context, knowledgePointID uint64) (knowledgevideo.PlaybackResolution, error)
	RecordPlayback(ctx context.Context, userID, knowledgeVideoID uint64) error
	ListTree(ctx context.Context) ([]knowledgevideo.KnowledgeTreeNode, error)
}

// Tree godoc
// @Summary 查询知识点视频树
// @Tags 知识点视频
// @Produce json
// @Success 200 {object} dto.KnowledgeVideoTreeResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/knowledge-videos/tree [get]
func (h *Handler) Tree(c *gin.Context) {
	nodes, err := h.service.ListTree(c)
	if err != nil {
		httpError(c, err)
		return
	}
	c.JSON(http.StatusOK, dto.SuccessResponse[dto.KnowledgeVideoTreeData]{Success: true, Data: dto.KnowledgeVideoTreeData{Nodes: treeNodes(nodes)}})
}

func treeNodes(nodes []knowledgevideo.KnowledgeTreeNode) []dto.KnowledgeVideoTreeNode {
	result := make([]dto.KnowledgeVideoTreeNode, 0, len(nodes))
	for _, node := range nodes {
		videos := node.Videos
		if len(videos) == 0 && node.Video != nil {
			videos = []knowledgevideo.KnowledgeTreeVideo{*node.Video}
		}
		item := dto.KnowledgeVideoTreeNode{ID: node.ID, ParentID: node.ParentID, Name: node.Name, Children: treeNodes(node.Children), Videos: make([]dto.KnowledgeVideoTreeVideo, 0, len(videos))}
		for _, video := range videos {
			item.Videos = append(item.Videos, dto.KnowledgeVideoTreeVideo{ID: video.ID, BatchID: video.BatchID, VideoName: video.VideoName, Duration: video.Duration, Status: videoStatus(video.Status), ErrorMessage: video.ErrorMessage})
		}
		if len(item.Videos) > 0 {
			item.Video = &item.Videos[0]
		}
		result = append(result, item)
	}
	return result
}

type Handler struct {
	service         Service
	maxRequestBytes int64
}

func New(service Service, maxRequestBytes int64) *Handler {
	return &Handler{service: service, maxRequestBytes: maxRequestBytes}
}

// Import godoc
// @Summary 批量导入知识点视频
// @Tags 知识点视频
// @Accept multipart/form-data
// @Produce json
// @Param archive formData file true "ZIP video archive"
// @Param mapping formData file true "XLSX knowledge-point mapping"
// @Success 202 {object} dto.KnowledgeVideoImportResponse
// @Failure 400 {object} dto.KnowledgeVideoValidationError
// @Failure 500 {object} dto.ErrorResponse
// @Security BearerAuth
// @Router /api/admin/knowledge-videos/batches [post]
func (h *Handler) Import(c *gin.Context) {
	if h.maxRequestBytes > 0 {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, h.maxRequestBytes)
	}
	defer func() {
		if c.Request.MultipartForm != nil {
			_ = c.Request.MultipartForm.RemoveAll()
		}
	}()
	archive, archiveHeader, err := c.Request.FormFile("archive")
	if err != nil {
		httpError(c, &knowledgevideo.InvalidArgument{Field: "archive"})
		return
	}
	defer archive.Close()
	mapping, mappingHeader, err := c.Request.FormFile("mapping")
	if err != nil {
		httpError(c, &knowledgevideo.InvalidArgument{Field: "mapping"})
		return
	}
	defer mapping.Close()
	uploadUserID, ok := middleware.AdminID(c)
	if !ok {
		c.AbortWithStatusJSON(http.StatusUnauthorized, dto.ErrorResponse{Success: false, Error: dto.ErrorBody{Code: "unauthorized", Message: "administrator authentication required"}})
		return
	}
	result, err := h.service.Import(c, knowledgevideo.ImportInput{UploadUserID: uploadUserID, ArchiveName: archiveHeader.Filename, MappingName: mappingHeader.Filename, Archive: archive, Mapping: mapping})
	if err != nil {
		httpError(c, err)
		return
	}
	data := dto.KnowledgeVideoImportData{BatchID: result.BatchID, TotalCount: result.TotalCount, Status: batchStatus(result.Status), ProgressURL: fmt.Sprintf("/api/admin/knowledge-videos/batches/%d", result.BatchID)}
	c.JSON(http.StatusAccepted, dto.SuccessResponse[dto.KnowledgeVideoImportData]{Success: true, Data: data})
}

// Progress godoc
// @Summary 查询知识点视频导入进度
// @Tags 知识点视频
// @Produce json
// @Param batchId path int true "Batch ID"
// @Success 200 {object} dto.KnowledgeVideoBatchResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Security BearerAuth
// @Router /api/admin/knowledge-videos/batches/{batchId} [get]
func (h *Handler) Progress(c *gin.Context) {
	id, ok := positiveParam(c, "batchId")
	if !ok {
		return
	}
	batch, videos, found, err := h.service.GetBatch(c, id)
	if err != nil {
		httpError(c, err)
		return
	}
	if !found {
		httpError(c, &knowledgevideo.NotFound{Resource: "knowledge video batch"})
		return
	}
	items := make([]dto.KnowledgeVideoItem, 0, len(videos))
	for _, video := range videos {
		items = append(items, dto.KnowledgeVideoItem{ID: video.ID, KnowledgePointID: video.KnowledgePointID, KnowledgePointName: video.KnowledgePointName, VideoName: video.SourceFileName, Duration: video.Duration, Status: videoStatus(video.Status), ErrorMessage: video.ErrorMessage})
	}
	data := dto.KnowledgeVideoBatchData{BatchID: batch.ID, TotalCount: batch.TotalCount, ReadyCount: batch.ReadyCount, FailedCount: batch.FailedCount, Status: batchStatus(batch.Status), Videos: items}
	c.JSON(http.StatusOK, dto.SuccessResponse[dto.KnowledgeVideoBatchData]{Success: true, Data: data})
}

// Playback godoc
// @Summary 获取知识点视频播放地址
// @Tags 知识点视频
// @Produce json
// @Param knowledgePointId path int true "Knowledge point ID"
// @Success 200 {object} dto.KnowledgeVideoPlaybackResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 409 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/knowledge-points/{knowledgePointId}/video [get]
// @Router /api/knowledge-points/{knowledgePointId}/videos [get]
func (h *Handler) Playback(c *gin.Context) {
	knowledgePointID, ok := positiveParam(c, "knowledgePointId")
	if !ok {
		return
	}
	result, err := h.service.ListPlayback(c, knowledgePointID)
	if err != nil {
		httpError(c, err)
		return
	}
	data := dto.KnowledgeVideoPlaybackData{KnowledgePointID: result.KnowledgePointID, KnowledgePointName: result.KnowledgePointName, Videos: make([]dto.KnowledgeVideoPlaybackItem, 0, len(result.Videos))}
	for _, video := range result.Videos {
		data.Videos = append(data.Videos, dto.KnowledgeVideoPlaybackItem{KnowledgeVideoID: video.KnowledgeVideoID, SourceFileName: video.SourceFileName, DisplayName: video.DisplayName, Duration: video.Duration, PlaybackURL: video.PlaybackURL})
	}
	if len(data.Videos) > 0 {
		first := data.Videos[0]
		data.VideoID, data.VideoName, data.Duration, data.PlaybackURL = &first.KnowledgeVideoID, &first.SourceFileName, &first.Duration, &first.PlaybackURL
	}
	c.JSON(http.StatusOK, dto.SuccessResponse[dto.KnowledgeVideoPlaybackData]{Success: true, Data: data})
}

// RecordPlayback godoc
// @Summary 记录知识点视频实际播放
// @Tags 知识点视频
// @Accept json
// @Produce json
// @Param knowledgeVideoId path int true "Knowledge video ID"
// @Param request body dto.KnowledgeVideoPlaybackRecordRequest true "Playback user"
// @Success 200 {object} dto.KnowledgeVideoPlaybackRecordResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 409 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/knowledge-videos/{knowledgeVideoId}/playbacks [post]
func (h *Handler) RecordPlayback(c *gin.Context) {
	knowledgeVideoID, ok := positiveParam(c, "knowledgeVideoId")
	if !ok {
		return
	}
	var request dto.KnowledgeVideoPlaybackRecordRequest
	if err := c.ShouldBindJSON(&request); err != nil || request.UserID == 0 {
		httpError(c, &knowledgevideo.InvalidArgument{Field: "user_id"})
		return
	}
	if err := h.service.RecordPlayback(c, request.UserID, knowledgeVideoID); err != nil {
		httpError(c, err)
		return
	}
	c.JSON(http.StatusOK, dto.KnowledgeVideoPlaybackRecordResponse{Success: true, Data: dto.KnowledgeVideoPlaybackRecordData{}})
}

func positiveParam(c *gin.Context, name string) (uint64, bool) {
	id, err := strconv.ParseUint(c.Param(name), 10, 64)
	if err != nil || id == 0 {
		httpError(c, &knowledgevideo.InvalidArgument{Field: name})
		return 0, false
	}
	return id, true
}

func httpError(c *gin.Context, err error) {
	var validation *knowledgevideo.ValidationError
	if errors.As(err, &validation) {
		response := dto.KnowledgeVideoValidationError{}
		response.Error.Code, response.Error.Message, response.Error.Issues = "validation_failed", validation.Error(), validation.Issues
		c.JSON(http.StatusBadRequest, response)
		return
	}
	var invalid *knowledgevideo.InvalidArgument
	if errors.As(err, &invalid) {
		httperrors.Write(c, httperrors.InvalidArgument(invalid.Error()))
		return
	}
	var notFound *knowledgevideo.NotFound
	if errors.As(err, &notFound) {
		httperrors.Write(c, httperrors.NotFound("not_found", notFound.Error()))
		return
	}
	var notReady *knowledgevideo.NotReady
	if errors.As(err, &notReady) {
		httperrors.Write(c, &httperrors.APIError{Status: http.StatusConflict, Code: "VIDEO_NOT_READY", Message: notReady.Error()})
		return
	}
	var failed *knowledgevideo.TranscodeFailed
	if errors.As(err, &failed) {
		httperrors.Write(c, &httperrors.APIError{Status: http.StatusConflict, Code: "VIDEO_TRANSCODE_FAILED", Message: failed.Error()})
		return
	}
	httperrors.Write(c, httperrors.Internal("internal server error"))
}

func batchStatus(status knowledgevideo.BatchStatus) string {
	return map[knowledgevideo.BatchStatus]string{knowledgevideo.BatchProcessing: "processing", knowledgevideo.BatchCompleted: "completed", knowledgevideo.BatchPartialFailed: "partial_failed", knowledgevideo.BatchFailed: "failed"}[status]
}

func videoStatus(status knowledgevideo.VideoStatus) string {
	return map[knowledgevideo.VideoStatus]string{knowledgevideo.VideoPending: "pending", knowledgevideo.VideoTranscoding: "transcoding", knowledgevideo.VideoReady: "ready", knowledgevideo.VideoFailed: "failed"}[status]
}
