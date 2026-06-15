package videoapp

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"

	uploadapp "nlp-video-analysis/internal/application/videoapp/upload"
)

// UploadVideo 复用既有上传三段式流程，把协议层 reader 桥接到当前应用服务能力。
func (s *Service) UploadVideo(ctx context.Context, input UploadVideoInput) (UploadResult, error) {
	result, err := newUploadService(s).UploadVideo(ctx, uploadapp.UploadVideoInput{
		FileName:    input.FileName,
		ContentType: input.ContentType,
		Title:       input.Title,
		Description: input.Description,
		Reader:      input.Reader,
	})
	if err != nil {
		return UploadResult{}, err
	}
	return mapUploadResultFromApp(result), nil
}

func (s *Service) UploadVideoArchive(ctx context.Context, input UploadVideoArchiveInput) (ArchiveUploadResult, error) {
	if strings.TrimSpace(input.FileName) == "" {
		return ArchiveUploadResult{}, InvalidArgumentError("file is required")
	}
	if input.Reader == nil {
		return ArchiveUploadResult{}, InvalidArgumentError("file is required")
	}
	if !isZipFileName(input.FileName) {
		return ArchiveUploadResult{}, InvalidArgumentError("zip archive is required")
	}

	uploadID, err := newChunkedUploadID()
	if err != nil {
		return ArchiveUploadResult{}, err
	}
	tempDir := filepath.Join(s.chunkedUploadRoot(), uploadID)
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return ArchiveUploadResult{}, err
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	archivePath := filepath.Join(tempDir, "archive.zip")
	file, err := os.Create(archivePath)
	if err != nil {
		return ArchiveUploadResult{}, err
	}
	_, copyErr := io.Copy(file, input.Reader)
	closeErr := file.Close()
	if copyErr != nil {
		return ArchiveUploadResult{}, copyErr
	}
	if closeErr != nil {
		return ArchiveUploadResult{}, closeErr
	}

	return s.importVideoArchiveFile(ctx, archivePath, input.Description)
}

// UploadVideoCover 把封面写入对象存储后，通过既有业务逻辑更新 cover_url。
func (s *Service) UploadVideoCover(ctx context.Context, videoID uint64, input UploadCoverInput) (string, bool, error) {
	return newUploadService(s).UploadVideoCover(ctx, videoID, uploadapp.UploadCoverInput{
		FileName:    input.FileName,
		ContentType: input.ContentType,
		Size:        input.Size,
		Reader:      input.Reader,
	})
}

func isZipFileName(name string) bool {
	return uploadapp.IsZipFileName(name)
}

func isSafeArchiveEntryName(name string) bool {
	return uploadapp.IsSafeArchiveEntryName(name)
}

func isArchiveMetadataEntryName(name string) bool {
	return uploadapp.IsArchiveMetadataEntryName(name)
}

func isSupportedVideoFileName(name string) bool {
	return uploadapp.IsSupportedVideoFileName(name)
}

func contentTypeFromVideoExtension(ext string) string {
	return uploadapp.ContentTypeFromVideoExtension(ext)
}

func rollbackObject(ctx context.Context, store ObjectStore, objectKey string) {
	if store == nil || strings.TrimSpace(objectKey) == "" {
		return
	}
	uploadapp.RollbackObject(ctx, objectStoreAdapter{store: store}, objectKey)
}

func contentTypeFromExtension(ext string) string {
	return uploadapp.ContentTypeFromExtension(ext)
}

func newUploadService(s *Service) uploadapp.Service {
	return uploadapp.Service{
		Repo:            uploadRepositoryAdapter{repo: s.Repo, owner: s},
		Planner:         uploadPlannerAdapter{service: s},
		Store:           objectStoreAdapter{store: s.Store},
		FS:              fileRemoverAdapter{fs: s.FS},
		Now:             s.Now,
		InvalidArgument: InvalidArgumentError,
		CoverURLPrefix:  s.Paths.CoverURLPrefix,
	}
}

type uploadRepositoryAdapter struct {
	repo  VideoRepository
	owner *Service
}

func (a uploadRepositoryAdapter) GetByID(ctx context.Context, id uint64) (any, bool, error) {
	video, ok, err := a.repo.GetByID(ctx, id)
	if err != nil || !ok {
		return nil, ok, err
	}
	return video, true, nil
}

func (a uploadRepositoryAdapter) SetVideoCover(ctx context.Context, videoID uint64, coverURL string) (bool, error) {
	return a.owner.SetVideoCover(ctx, videoID, coverURL)
}

type uploadPlannerAdapter struct {
	service *Service
}

func (a uploadPlannerAdapter) BuildUploadPlan(originalFileName string) (uploadapp.Plan, error) {
	plan, err := a.service.BuildUploadPlan(originalFileName)
	if err != nil {
		return uploadapp.Plan{}, err
	}
	return uploadapp.Plan{
		OriginalFileName: plan.OriginalFileName,
		StoredFileName:   plan.StoredFileName,
		DatePath:         plan.DatePath,
		RawAbsPath:       plan.RawAbsPath,
		RawObjectKey:     plan.RawObjectKey,
		RawURL:           plan.RawURL,
		HLSAbsDir:        plan.HLSAbsDir,
		HLSObjectPrefix:  plan.HLSObjectPrefix,
		HLSURL:           plan.HLSURL,
		RawUploaded:      plan.RawUploaded,
	}, nil
}

func (a uploadPlannerAdapter) OpenUploadWriter(plan uploadapp.Plan) (io.WriteCloser, error) {
	return a.service.OpenUploadWriter(UploadPlan{
		OriginalFileName: plan.OriginalFileName,
		StoredFileName:   plan.StoredFileName,
		DatePath:         plan.DatePath,
		RawAbsPath:       plan.RawAbsPath,
		RawObjectKey:     plan.RawObjectKey,
		RawURL:           plan.RawURL,
		HLSAbsDir:        plan.HLSAbsDir,
		HLSObjectPrefix:  plan.HLSObjectPrefix,
		HLSURL:           plan.HLSURL,
		RawUploaded:      plan.RawUploaded,
	})
}

func (a uploadPlannerAdapter) FinalizeUpload(ctx context.Context, plan uploadapp.Plan, meta uploadapp.Meta) (uploadapp.Result, error) {
	result, err := a.service.FinalizeUpload(ctx, UploadPlan{
		OriginalFileName: plan.OriginalFileName,
		StoredFileName:   plan.StoredFileName,
		DatePath:         plan.DatePath,
		RawAbsPath:       plan.RawAbsPath,
		RawObjectKey:     plan.RawObjectKey,
		RawURL:           plan.RawURL,
		HLSAbsDir:        plan.HLSAbsDir,
		HLSObjectPrefix:  plan.HLSObjectPrefix,
		HLSURL:           plan.HLSURL,
		RawUploaded:      plan.RawUploaded,
	}, UploadMeta{
		Title:       meta.Title,
		Description: meta.Description,
	})
	if err != nil {
		return uploadapp.Result{}, err
	}
	return uploadapp.Result{
		VideoID: result.VideoID,
		TaskID:  result.TaskID,
		RawURL:  result.RawURL,
		HLSURL:  result.HLSURL,
		Name:    result.Name,
	}, nil
}

type objectStoreAdapter struct {
	store ObjectStore
}

func (a objectStoreAdapter) Put(ctx context.Context, objectKey string, r io.Reader, size int64, contentType string) error {
	return a.store.Put(ctx, objectKey, r, size, contentType)
}

func (a objectStoreAdapter) Delete(ctx context.Context, objectKey string) error {
	return a.store.Delete(ctx, objectKey)
}

type fileRemoverAdapter struct {
	fs FileStorage
}

func (a fileRemoverAdapter) Remove(path string) error {
	return a.fs.Remove(path)
}

func mapUploadResultFromApp(result uploadapp.Result) UploadResult {
	return UploadResult{
		VideoID: result.VideoID,
		TaskID:  result.TaskID,
		RawURL:  result.RawURL,
		HLSURL:  result.HLSURL,
		Name:    result.Name,
	}
}

func mapArchiveResultFromApp(result uploadapp.ArchiveResult) ArchiveUploadResult {
	uploaded := make([]UploadResult, 0, len(result.Uploaded))
	for _, item := range result.Uploaded {
		uploaded = append(uploaded, mapUploadResultFromApp(item))
	}
	failed := make([]ArchiveUploadFailure, 0, len(result.Failed))
	for _, item := range result.Failed {
		failed = append(failed, ArchiveUploadFailure{
			FileName: item.FileName,
			Error:    item.Error,
		})
	}
	return ArchiveUploadResult{
		Total:    result.Total,
		Uploaded: uploaded,
		Failed:   failed,
		Skipped:  result.Skipped,
	}
}
