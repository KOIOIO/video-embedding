package handler

import (
	"github.com/gin-gonic/gin"

	uploadshandler "nlp-video-analysis/internal/http/handler/uploads"
)

type UploadHandler struct {
	inner *uploadshandler.Handler
}

func NewUploadHandler(app any) *UploadHandler {
	return &UploadHandler{inner: uploadshandler.New(app)}
}

// UploadVideo godoc
// @Summary Upload a video
// @Tags videos
// @Accept multipart/form-data
// @Produce json
// @Param file formData file true "Video file"
// @Param title formData string false "Video title"
// @Param description formData string false "Video description"
// @Param user_id formData int false "Uploader user ID, defaults to 1"
// @Success 200 {object} dto.UploadVideoResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/videos [post]
func (h *UploadHandler) UploadVideo(c *gin.Context) {
	h.inner.UploadVideo(c)
}

// UploadVideoArchive godoc
// @Summary Upload a zip archive containing videos
// @Tags videos
// @Accept multipart/form-data
// @Produce json
// @Param file formData file true "Zip archive"
// @Param description formData string false "Video description"
// @Param user_id formData int false "Uploader user ID, defaults to 1"
// @Success 200 {object} dto.UploadVideoArchiveResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/videos/archive [post]
func (h *UploadHandler) UploadVideoArchive(c *gin.Context) {
	h.inner.UploadVideoArchive(c)
}

// UploadVideoCover godoc
// @Summary Upload a video cover
// @Tags videos
// @Accept multipart/form-data
// @Produce json
// @Param id path int true "Video ID"
// @Param file formData file true "Cover file"
// @Success 200 {object} dto.UploadCoverResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/videos/{id}/cover [post]
func (h *UploadHandler) UploadVideoCover(c *gin.Context) {
	h.inner.UploadVideoCover(c)
}

// InitiateChunkedUpload godoc
// @Summary Initiate a chunked video upload
// @Tags videos
// @Accept json
// @Produce json
// @Param request body dto.InitiateChunkedUploadRequest true "Chunked upload metadata"
// @Success 200 {object} dto.ChunkedUploadResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/videos/uploads [post]
func (h *UploadHandler) InitiateChunkedUpload(c *gin.Context) {
	h.inner.InitiateChunkedUpload(c)
}

// UploadVideoChunk godoc
// @Summary Upload one video chunk
// @Tags videos
// @Accept application/octet-stream
// @Produce json
// @Param uploadId path string true "Upload ID"
// @Param chunkIndex path int true "Zero-based chunk index"
// @Param chunk body string true "Raw chunk bytes"
// @Success 200 {object} dto.ChunkedUploadResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/videos/uploads/{uploadId}/chunks/{chunkIndex} [put]
func (h *UploadHandler) UploadVideoChunk(c *gin.Context) {
	h.inner.UploadVideoChunk(c)
}

// GetChunkedUploadStatus godoc
// @Summary Get chunked upload status
// @Tags videos
// @Produce json
// @Param uploadId path string true "Upload ID"
// @Success 200 {object} dto.ChunkedUploadResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/videos/uploads/{uploadId} [get]
func (h *UploadHandler) GetChunkedUploadStatus(c *gin.Context) {
	h.inner.GetChunkedUploadStatus(c)
}

// CompleteChunkedUpload godoc
// @Summary Complete a chunked video upload
// @Tags videos
// @Produce json
// @Param uploadId path string true "Upload ID"
// @Success 200 {object} dto.UploadVideoResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/videos/uploads/{uploadId}/complete [post]
func (h *UploadHandler) CompleteChunkedUpload(c *gin.Context) {
	h.inner.CompleteChunkedUpload(c)
}

// InitiateChunkedArchiveUpload godoc
// @Summary Initiate a chunked zip archive upload
// @Tags videos
// @Accept json
// @Produce json
// @Param request body dto.InitiateChunkedUploadRequest true "Chunked archive upload metadata"
// @Success 200 {object} dto.ChunkedUploadResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/videos/archive/uploads [post]
func (h *UploadHandler) InitiateChunkedArchiveUpload(c *gin.Context) {
	h.inner.InitiateChunkedArchiveUpload(c)
}

// CompleteChunkedArchiveUpload godoc
// @Summary Complete a chunked zip archive upload
// @Tags videos
// @Produce json
// @Param uploadId path string true "Upload ID"
// @Success 200 {object} dto.UploadVideoArchiveResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/videos/archive/uploads/{uploadId}/complete [post]
func (h *UploadHandler) CompleteChunkedArchiveUpload(c *gin.Context) {
	h.inner.CompleteChunkedArchiveUpload(c)
}

// GetArchiveProcessingProgress godoc
// @Summary Get zip archive processing progress
// @Tags videos
// @Produce json
// @Param batchId path string true "Archive batch ID"
// @Success 200 {object} dto.ArchiveProcessingProgressResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/videos/archive/batches/{batchId}/progress [get]
func (h *UploadHandler) GetArchiveProcessingProgress(c *gin.Context) {
	h.inner.GetArchiveProcessingProgress(c)
}
