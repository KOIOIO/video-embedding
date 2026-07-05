package upload

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"
)

type Plan struct {
	OriginalFileName string
	StoredFileName   string
	DatePath         string
	RawAbsPath       string
	RawObjectKey     string
	RawURL           string
	HLSAbsDir        string
	HLSObjectPrefix  string
	HLSURL           string
	RawUploaded      bool
}

type Result struct {
	VideoID uint64
	TaskID  string
	RawURL  string
	HLSURL  string
	Name    string
}

type ArchiveResult struct {
	Total    int
	Uploaded []Result
	Failed   []ArchiveFailure
	Skipped  []string
}

type ArchiveFailure struct {
	FileName string
	Error    string
}

type Meta struct {
	Title       string
	Description string
	UserID      uint64
}

type UploadVideoInput struct {
	FileName    string
	ContentType string
	Title       string
	Description string
	UserID      uint64
	Reader      io.Reader
}

type UploadVideoArchiveInput struct {
	FileName    string
	ContentType string
	Description string
	UserID      uint64
	Reader      io.Reader
}

type UploadCoverInput struct {
	FileName    string
	ContentType string
	Size        int64
	Reader      io.Reader
}

type Repository interface {
	GetByID(ctx context.Context, id uint64) (any, bool, error)
	SetVideoCover(ctx context.Context, videoID uint64, coverURL string) (bool, error)
}

type Planner interface {
	BuildUploadPlan(originalFileName string) (Plan, error)
	OpenUploadWriter(plan Plan) (io.WriteCloser, error)
	FinalizeUpload(ctx context.Context, plan Plan, meta Meta) (Result, error)
}

type ObjectStore interface {
	Put(ctx context.Context, objectKey string, r io.Reader, size int64, contentType string) error
	Delete(ctx context.Context, objectKey string) error
}

type FileRemover interface {
	Remove(path string) error
}

type Service struct {
	Repo            Repository
	Planner         Planner
	Store           ObjectStore
	FS              FileRemover
	Now             func() time.Time
	InvalidArgument func(message string) error
	CoverURLPrefix  string
}

func (s Service) UploadVideo(ctx context.Context, input UploadVideoInput) (result Result, err error) {
	if strings.TrimSpace(input.FileName) == "" {
		return Result{}, s.InvalidArgument("file is required")
	}
	if input.Reader == nil {
		return Result{}, s.InvalidArgument("file is required")
	}
	if IsArchiveMetadataEntryName(input.FileName) {
		return Result{}, s.InvalidArgument("unsupported metadata file")
	}

	plan, err := s.Planner.BuildUploadPlan(input.FileName)
	if err != nil {
		return Result{}, s.InvalidArgument("file is required")
	}
	defer func() {
		if err == nil || strings.TrimSpace(plan.RawAbsPath) == "" {
			return
		}
		_ = s.FS.Remove(plan.RawAbsPath)
	}()

	writer, err := s.Planner.OpenUploadWriter(plan)
	if err != nil {
		return Result{}, err
	}
	defer func() { _ = writer.Close() }()

	if _, err := io.Copy(writer, input.Reader); err != nil {
		return Result{}, err
	}
	if err := writer.Close(); err != nil {
		return Result{}, err
	}

	return s.Planner.FinalizeUpload(ctx, plan, Meta{
		Title:       input.Title,
		Description: input.Description,
		UserID:      input.UserID,
	})
}

func (s Service) UploadVideoArchive(ctx context.Context, input UploadVideoArchiveInput) (ArchiveResult, error) {
	if strings.TrimSpace(input.FileName) == "" {
		return ArchiveResult{}, s.InvalidArgument("file is required")
	}
	if input.Reader == nil {
		return ArchiveResult{}, s.InvalidArgument("file is required")
	}
	if !IsZipFileName(input.FileName) {
		return ArchiveResult{}, s.InvalidArgument("zip archive is required")
	}

	payload, err := io.ReadAll(input.Reader)
	if err != nil {
		return ArchiveResult{}, err
	}
	zr, err := zip.NewReader(bytes.NewReader(payload), int64(len(payload)))
	if err != nil {
		return ArchiveResult{}, s.InvalidArgument("invalid zip archive")
	}

	result := ArchiveResult{Total: len(zr.File)}
	for _, entry := range zr.File {
		name := strings.TrimSpace(entry.Name)
		if entry.FileInfo().IsDir() || !IsSafeArchiveEntryName(name) || IsArchiveMetadataEntryName(name) || !IsSupportedVideoFileName(name) {
			result.Skipped = append(result.Skipped, name)
			zap.L().Info("video_archive_entry_skipped",
				zap.String("archive", input.FileName),
				zap.String("entry", name))
			continue
		}
		rc, err := entry.Open()
		if err != nil {
			result.Failed = append(result.Failed, ArchiveFailure{FileName: name, Error: err.Error()})
			continue
		}
		uploadResult, err := s.UploadVideo(ctx, UploadVideoInput{
			FileName:    filepath.Base(name),
			ContentType: ContentTypeFromVideoExtension(filepath.Ext(name)),
			Title:       strings.TrimSuffix(filepath.Base(name), filepath.Ext(name)),
			Description: input.Description,
			UserID:      input.UserID,
			Reader:      rc,
		})
		_ = rc.Close()
		if err != nil {
			result.Failed = append(result.Failed, ArchiveFailure{FileName: name, Error: err.Error()})
			zap.L().Warn("video_archive_entry_failed",
				zap.String("archive", input.FileName),
				zap.String("entry", name),
				zap.Error(err))
			continue
		}
		uploadResult.Name = filepath.Base(name)
		result.Uploaded = append(result.Uploaded, uploadResult)
		zap.L().Info("video_archive_entry_uploaded",
			zap.String("archive", input.FileName),
			zap.String("entry", name),
			zap.Uint64("video_id", uploadResult.VideoID))
	}
	return result, nil
}

func (s Service) UploadVideoCover(ctx context.Context, videoID uint64, input UploadCoverInput) (string, bool, error) {
	if videoID == 0 {
		return "", false, s.InvalidArgument("video_id is required")
	}
	if strings.TrimSpace(input.FileName) == "" {
		return "", false, s.InvalidArgument("file is required")
	}
	if input.Reader == nil {
		return "", false, s.InvalidArgument("file is required")
	}
	if _, ok, err := s.Repo.GetByID(ctx, videoID); err != nil {
		return "", false, err
	} else if !ok {
		return "", false, nil
	}

	ext := filepath.Ext(input.FileName)
	if ext == "" {
		ext = ".jpg"
	}
	now := s.Now()
	datePath := now.Format("2006/01/02")
	ts := now.UnixNano()
	objectKey := filepath.ToSlash(filepath.Join("cover", datePath, "vid_"+strconv.FormatUint(videoID, 10)+"_"+strconv.FormatInt(ts, 10)+ext))
	contentType := strings.TrimSpace(input.ContentType)
	if contentType == "" {
		contentType = ContentTypeFromExtension(ext)
	}
	if input.Size < 0 {
		input.Size = -1
	}

	if err := s.Store.Put(ctx, objectKey, input.Reader, input.Size, contentType); err != nil {
		return "", false, err
	}
	coverURL := strings.TrimRight(defaultString(s.CoverURLPrefix, "/videos"), "/") + "/" + objectKey
	updated, err := s.Repo.SetVideoCover(ctx, videoID, coverURL)
	if err != nil {
		RollbackObject(ctx, s.Store, objectKey)
		return "", false, err
	}
	if !updated {
		RollbackObject(ctx, s.Store, objectKey)
		return "", false, nil
	}
	return coverURL, updated, nil
}

func IsZipFileName(name string) bool {
	return strings.EqualFold(filepath.Ext(strings.TrimSpace(name)), ".zip")
}

func IsSafeArchiveEntryName(name string) bool {
	name = strings.TrimSpace(name)
	if name == "" || filepath.IsAbs(name) {
		return false
	}
	clean := filepath.Clean(name)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return false
	}
	parts := strings.FieldsFunc(clean, func(r rune) bool {
		return r == '/' || r == '\\'
	})
	for _, part := range parts {
		if part == "" || part == "." || part == ".." {
			return false
		}
	}
	return true
}

func IsArchiveMetadataEntryName(name string) bool {
	clean := filepath.ToSlash(filepath.Clean(strings.TrimSpace(name)))
	base := filepath.Base(clean)
	if clean == "__MACOSX" || strings.HasPrefix(clean, "__MACOSX/") {
		return true
	}
	return base == ".DS_Store" || strings.HasPrefix(base, "._")
}

func IsSupportedVideoFileName(name string) bool {
	switch strings.ToLower(filepath.Ext(strings.TrimSpace(name))) {
	case ".mp4", ".mov", ".m4v", ".avi", ".mkv", ".webm":
		return true
	default:
		return false
	}
}

func ContentTypeFromVideoExtension(ext string) string {
	switch strings.ToLower(strings.TrimSpace(ext)) {
	case ".mp4", ".m4v":
		return "video/mp4"
	case ".mov":
		return "video/quicktime"
	case ".avi":
		return "video/x-msvideo"
	case ".mkv":
		return "video/x-matroska"
	case ".webm":
		return "video/webm"
	default:
		return "application/octet-stream"
	}
}

func RollbackObject(ctx context.Context, store ObjectStore, objectKey string) {
	if store == nil || strings.TrimSpace(objectKey) == "" {
		return
	}
	rollbackCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	_ = store.Delete(rollbackCtx, objectKey)
}

func ContentTypeFromExtension(ext string) string {
	switch strings.ToLower(strings.TrimSpace(ext)) {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".webp":
		return "image/webp"
	case ".gif":
		return "image/gif"
	default:
		return "application/octet-stream"
	}
}

func defaultString(value string, fallback string) string {
	value = strings.TrimSpace(value)
	if value != "" {
		return value
	}
	return fallback
}
