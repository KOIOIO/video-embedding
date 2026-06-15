package uploads

import (
	"context"
	"errors"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"nlp-video-analysis/internal/application/videoapp"
	"nlp-video-analysis/internal/http/dto"
	httperrors "nlp-video-analysis/internal/http/errors"
)

type Handler struct {
	app uploadApp
}

type uploadApp interface {
	UploadVideo(ctx context.Context, input videoapp.UploadVideoInput) (videoapp.UploadResult, error)
	UploadVideoArchive(ctx context.Context, input videoapp.UploadVideoArchiveInput) (videoapp.ArchiveUploadResult, error)
	UploadVideoCover(ctx context.Context, videoID uint64, input videoapp.UploadCoverInput) (string, bool, error)
	InitiateChunkedUpload(ctx context.Context, input videoapp.InitiateChunkedUploadInput) (videoapp.ChunkedUploadStatus, error)
	UploadVideoChunk(ctx context.Context, input videoapp.UploadVideoChunkInput) (videoapp.ChunkedUploadStatus, error)
	GetChunkedUploadStatus(ctx context.Context, uploadID string) (videoapp.ChunkedUploadStatus, error)
	CompleteChunkedUpload(ctx context.Context, input videoapp.CompleteChunkedUploadInput) (videoapp.UploadResult, error)
	InitiateChunkedArchiveUpload(ctx context.Context, input videoapp.InitiateChunkedUploadInput) (videoapp.ChunkedUploadStatus, error)
	CompleteChunkedArchiveUpload(ctx context.Context, input videoapp.CompleteChunkedUploadInput) (videoapp.ArchiveUploadResult, error)
}

func New(app any) *Handler {
	switch v := app.(type) {
	case uploadApp:
		return &Handler{app: v}
	default:
		panic("unsupported upload app")
	}
}

func (h *Handler) UploadVideo(c *gin.Context) {
	defer cleanupMultipartForm(c.Request)

	file, header, ok := readRequiredFormFile(c)
	if !ok {
		return
	}
	defer file.Close()

	result, err := h.app.UploadVideo(c.Request.Context(), videoapp.UploadVideoInput{
		FileName:    header.Filename,
		ContentType: header.Header.Get("Content-Type"),
		Title:       strings.TrimSpace(c.PostForm("title")),
		Description: c.PostForm("description"),
		Reader:      file,
	})
	if err != nil {
		writeAppError(c, err, "upload video failed")
		return
	}

	writeSuccess(c, dto.UploadVideoData{
		VideoID: result.VideoID,
		TaskID:  result.TaskID,
		RawURL:  result.RawURL,
		HLSURL:  result.HLSURL,
		Name:    result.Name,
	})
}

func (h *Handler) UploadVideoArchive(c *gin.Context) {
	defer cleanupMultipartForm(c.Request)

	file, header, ok := readRequiredFormFile(c)
	if !ok {
		return
	}
	defer file.Close()

	result, err := h.app.UploadVideoArchive(c.Request.Context(), videoapp.UploadVideoArchiveInput{
		FileName:    header.Filename,
		ContentType: header.Header.Get("Content-Type"),
		Description: c.PostForm("description"),
		Reader:      file,
	})
	if err != nil {
		writeAppError(c, err, "upload video archive failed")
		return
	}

	writeSuccess(c, mapArchiveUploadResult(result))
}

func (h *Handler) UploadVideoCover(c *gin.Context) {
	defer cleanupMultipartForm(c.Request)

	videoID, ok := parsePositiveUintParam(c, "id")
	if !ok {
		return
	}

	file, header, ok := readRequiredFormFile(c)
	if !ok {
		return
	}
	defer file.Close()

	coverURL, updated, err := h.app.UploadVideoCover(c.Request.Context(), videoID, videoapp.UploadCoverInput{
		FileName:    header.Filename,
		ContentType: header.Header.Get("Content-Type"),
		Size:        header.Size,
		Reader:      file,
	})
	if err != nil {
		writeAppError(c, err, "upload video cover failed")
		return
	}
	if !updated {
		httperrors.Write(c, httperrors.NotFound("video_not_found", "video does not exist"))
		return
	}

	writeSuccess(c, dto.UploadCoverData{VideoID: videoID, CoverURL: coverURL})
}

func (h *Handler) InitiateChunkedUpload(c *gin.Context) {
	var req dto.InitiateChunkedUploadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httperrors.Write(c, httperrors.InvalidArgument("invalid request body"))
		return
	}

	status, err := h.app.InitiateChunkedUpload(c.Request.Context(), videoapp.InitiateChunkedUploadInput{
		FileName:    req.FileName,
		ContentType: req.ContentType,
		Title:       strings.TrimSpace(req.Title),
		Description: req.Description,
		FileSize:    req.FileSize,
		ChunkSize:   req.ChunkSize,
		TotalChunks: req.TotalChunks,
	})
	if err != nil {
		writeAppError(c, err, "initiate chunked upload failed")
		return
	}

	writeSuccess(c, mapChunkedUploadStatus(status))
}

func (h *Handler) UploadVideoChunk(c *gin.Context) {
	uploadID, ok := parseRequiredStringParam(c, "uploadId", "upload_id is required")
	if !ok {
		return
	}
	chunkIndex, ok := parseNonNegativeIntParam(c, "chunkIndex")
	if !ok {
		return
	}

	status, err := h.app.UploadVideoChunk(c.Request.Context(), videoapp.UploadVideoChunkInput{
		UploadID:   uploadID,
		ChunkIndex: chunkIndex,
		Reader:     c.Request.Body,
	})
	if err != nil {
		writeAppError(c, err, "upload video chunk failed")
		return
	}

	writeSuccess(c, mapChunkedUploadStatus(status))
}

func (h *Handler) GetChunkedUploadStatus(c *gin.Context) {
	uploadID, ok := parseRequiredStringParam(c, "uploadId", "upload_id is required")
	if !ok {
		return
	}

	status, err := h.app.GetChunkedUploadStatus(c.Request.Context(), uploadID)
	if err != nil {
		writeAppError(c, err, "get chunked upload status failed")
		return
	}

	writeSuccess(c, mapChunkedUploadStatus(status))
}

func (h *Handler) CompleteChunkedUpload(c *gin.Context) {
	uploadID, ok := parseRequiredStringParam(c, "uploadId", "upload_id is required")
	if !ok {
		return
	}

	result, err := h.app.CompleteChunkedUpload(c.Request.Context(), videoapp.CompleteChunkedUploadInput{
		UploadID: uploadID,
	})
	if err != nil {
		writeAppError(c, err, "complete chunked upload failed")
		return
	}

	writeSuccess(c, dto.UploadVideoData{
		VideoID: result.VideoID,
		TaskID:  result.TaskID,
		RawURL:  result.RawURL,
		HLSURL:  result.HLSURL,
		Name:    result.Name,
	})
}

func (h *Handler) InitiateChunkedArchiveUpload(c *gin.Context) {
	var req dto.InitiateChunkedUploadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httperrors.Write(c, httperrors.InvalidArgument("invalid request body"))
		return
	}

	status, err := h.app.InitiateChunkedArchiveUpload(c.Request.Context(), videoapp.InitiateChunkedUploadInput{
		FileName:    req.FileName,
		ContentType: req.ContentType,
		Description: req.Description,
		FileSize:    req.FileSize,
		ChunkSize:   req.ChunkSize,
		TotalChunks: req.TotalChunks,
	})
	if err != nil {
		writeAppError(c, err, "initiate chunked archive upload failed")
		return
	}

	writeSuccess(c, mapChunkedUploadStatus(status))
}

func (h *Handler) CompleteChunkedArchiveUpload(c *gin.Context) {
	uploadID, ok := parseRequiredStringParam(c, "uploadId", "upload_id is required")
	if !ok {
		return
	}

	result, err := h.app.CompleteChunkedArchiveUpload(c.Request.Context(), videoapp.CompleteChunkedUploadInput{
		UploadID: uploadID,
	})
	if err != nil {
		writeAppError(c, err, "complete chunked archive upload failed")
		return
	}

	writeSuccess(c, mapArchiveUploadResult(result))
}

func readRequiredFormFile(c *gin.Context) (multipart.File, *multipart.FileHeader, bool) {
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		if err == http.ErrMissingFile || strings.Contains(strings.ToLower(err.Error()), "no such file") {
			httperrors.Write(c, httperrors.InvalidArgument("file is required"))
			return nil, nil, false
		}
		httperrors.Write(c, httperrors.InvalidArgument("invalid multipart form"))
		return nil, nil, false
	}
	if header == nil || strings.TrimSpace(header.Filename) == "" {
		_ = file.Close()
		httperrors.Write(c, httperrors.InvalidArgument("file is required"))
		return nil, nil, false
	}
	return file, header, true
}

func mapChunkedUploadStatus(status videoapp.ChunkedUploadStatus) dto.ChunkedUploadData {
	uploadedChunks := status.UploadedChunks
	if uploadedChunks == nil {
		uploadedChunks = []int{}
	}
	return dto.ChunkedUploadData{
		UploadID:       status.UploadID,
		FileName:       status.FileName,
		FileSize:       status.FileSize,
		ChunkSize:      status.ChunkSize,
		TotalChunks:    status.TotalChunks,
		UploadedChunks: uploadedChunks,
		Completed:      status.Completed,
	}
}

func mapArchiveUploadResult(result videoapp.ArchiveUploadResult) dto.UploadVideoArchiveData {
	videos := make([]dto.UploadVideoData, 0, len(result.Uploaded))
	for _, item := range result.Uploaded {
		videos = append(videos, dto.UploadVideoData{
			VideoID: item.VideoID,
			TaskID:  item.TaskID,
			RawURL:  item.RawURL,
			HLSURL:  item.HLSURL,
			Name:    item.Name,
		})
	}
	errors := make([]dto.UploadArchiveError, 0, len(result.Failed))
	for _, item := range result.Failed {
		errors = append(errors, dto.UploadArchiveError{
			FileName: item.FileName,
			Error:    item.Error,
		})
	}
	return dto.UploadVideoArchiveData{
		Total:        result.Total,
		Uploaded:     len(result.Uploaded),
		Failed:       len(result.Failed),
		Skipped:      len(result.Skipped),
		Videos:       videos,
		Errors:       errors,
		SkippedFiles: result.Skipped,
	}
}

func cleanupMultipartForm(req *http.Request) {
	if req == nil || req.MultipartForm == nil {
		return
	}
	_ = req.MultipartForm.RemoveAll()
}

func writeSuccess[T any](c *gin.Context, data T) {
	c.JSON(http.StatusOK, dto.SuccessResponse[T]{Success: true, Data: data})
}

func parsePositiveUintParam(c *gin.Context, name string) (uint64, bool) {
	raw := strings.TrimSpace(c.Param(name))
	value, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || value == 0 {
		httperrors.Write(c, httperrors.InvalidArgument(name+" must be a positive integer"))
		return 0, false
	}
	return value, true
}

func parseNonNegativeIntParam(c *gin.Context, name string) (int, bool) {
	raw := strings.TrimSpace(c.Param(name))
	value, err := strconv.Atoi(raw)
	if err != nil || value < 0 {
		httperrors.Write(c, httperrors.InvalidArgument(name+" must be a non-negative integer"))
		return 0, false
	}
	return value, true
}

func parseRequiredStringParam(c *gin.Context, name string, message string) (string, bool) {
	value := strings.TrimSpace(c.Param(name))
	if value == "" {
		httperrors.Write(c, httperrors.InvalidArgument(message))
		return "", false
	}
	return value, true
}

func writeAppError(c *gin.Context, err error, fallback string) {
	if err == nil {
		httperrors.Write(c, httperrors.Internal(fallback))
		return
	}
	var validationErr videoapp.ValidationError
	if errors.As(err, &validationErr) {
		httperrors.Write(c, httperrors.InvalidArgument(err.Error()))
		return
	}
	httperrors.Write(c, httperrors.Internal(fallback))
}
