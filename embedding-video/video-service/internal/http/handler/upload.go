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
// @Summary 上传单个视频
// @Tags 视频服务
// @Accept multipart/form-data
// @Produce json
// @Param file formData file true "视频文件"
// @Param title formData string false "视频标题"
// @Param description formData string false "视频描述"
// @Param user_id formData int false "上传用户ID，默认1"
// @Success 200 {object} dto.UploadVideoResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/videos [post]
func (h *UploadHandler) UploadVideo(c *gin.Context) {
	h.inner.UploadVideo(c)
}

// UploadVideoArchive godoc
// @Summary 上传视频压缩包
// @Tags 视频服务
// @Accept multipart/form-data
// @Produce json
// @Param file formData file true "视频压缩包"
// @Param description formData string false "视频描述"
// @Param user_id formData int false "上传用户ID，默认1"
// @Success 200 {object} dto.UploadVideoArchiveResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/videos/archive [post]
func (h *UploadHandler) UploadVideoArchive(c *gin.Context) {
	h.inner.UploadVideoArchive(c)
}

// UploadVideoCover godoc
// @Summary 上传视频封面
// @Tags 视频服务
// @Accept multipart/form-data
// @Produce json
// @Param id path int true "视频ID"
// @Param file formData file true "封面文件"
// @Success 200 {object} dto.UploadCoverResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/videos/{id}/cover [post]
func (h *UploadHandler) UploadVideoCover(c *gin.Context) {
	h.inner.UploadVideoCover(c)
}

// InitiateChunkedUpload godoc
// @Summary 初始化视频分片上传
// @Tags 视频服务
// @Accept json
// @Produce json
// @Param request body dto.InitiateChunkedUploadRequest true "分片上传元信息"
// @Success 200 {object} dto.ChunkedUploadResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/videos/uploads [post]
func (h *UploadHandler) InitiateChunkedUpload(c *gin.Context) {
	h.inner.InitiateChunkedUpload(c)
}

// UploadVideoChunk godoc
// @Summary 上传视频分片
// @Tags 视频服务
// @Accept application/octet-stream
// @Produce json
// @Param uploadId path string true "上传任务ID"
// @Param chunkIndex path int true "分片序号，从0开始"
// @Param chunk body string true "分片二进制内容"
// @Success 200 {object} dto.ChunkedUploadResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/videos/uploads/{uploadId}/chunks/{chunkIndex} [put]
func (h *UploadHandler) UploadVideoChunk(c *gin.Context) {
	h.inner.UploadVideoChunk(c)
}

// GetChunkedUploadStatus godoc
// @Summary 查询视频分片上传状态
// @Tags 视频服务
// @Produce json
// @Param uploadId path string true "上传任务ID"
// @Success 200 {object} dto.ChunkedUploadResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/videos/uploads/{uploadId} [get]
func (h *UploadHandler) GetChunkedUploadStatus(c *gin.Context) {
	h.inner.GetChunkedUploadStatus(c)
}

// CompleteChunkedUpload godoc
// @Summary 完成视频分片上传
// @Tags 视频服务
// @Produce json
// @Param uploadId path string true "上传任务ID"
// @Success 200 {object} dto.UploadVideoResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/videos/uploads/{uploadId}/complete [post]
func (h *UploadHandler) CompleteChunkedUpload(c *gin.Context) {
	h.inner.CompleteChunkedUpload(c)
}

// InitiateChunkedArchiveUpload godoc
// @Summary 初始化视频压缩包分片上传
// @Tags 视频服务
// @Accept json
// @Produce json
// @Param request body dto.InitiateChunkedUploadRequest true "压缩包分片上传元信息"
// @Success 200 {object} dto.ChunkedUploadResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/videos/archive/uploads [post]
func (h *UploadHandler) InitiateChunkedArchiveUpload(c *gin.Context) {
	h.inner.InitiateChunkedArchiveUpload(c)
}

// CompleteChunkedArchiveUpload godoc
// @Summary 完成视频压缩包分片上传
// @Tags 视频服务
// @Produce json
// @Param uploadId path string true "上传任务ID"
// @Success 200 {object} dto.UploadVideoArchiveResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/videos/archive/uploads/{uploadId}/complete [post]
func (h *UploadHandler) CompleteChunkedArchiveUpload(c *gin.Context) {
	h.inner.CompleteChunkedArchiveUpload(c)
}

// GetArchiveProcessingProgress godoc
// @Summary 查询视频压缩包处理进度
// @Tags 视频服务
// @Produce json
// @Param batchId path string true "压缩包批次ID"
// @Success 200 {object} dto.ArchiveProcessingProgressResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/videos/archive/batches/{batchId}/progress [get]
func (h *UploadHandler) GetArchiveProcessingProgress(c *gin.Context) {
	h.inner.GetArchiveProcessingProgress(c)
}
