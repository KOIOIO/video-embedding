package knowledgevideo

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.uber.org/zap"
)

type ImportInput struct {
	UploadUserID uint64
	ArchiveName  string
	MappingName  string
	Archive      io.Reader
	Mapping      io.Reader
}

type ImportRepository interface {
	IDReservation
	BatchCreator
	SetEnqueueTime(ctx context.Context, id uint64, enqueueTime time.Time) (bool, error)
}

type ImportObjectStore interface {
	PutFile(ctx context.Context, objectKey string, filePath string, contentType string) error
	Delete(ctx context.Context, objectKey string) error
}

type ImportQueue interface {
	Enqueue(ctx context.Context, task TranscodeTask) error
}

type ImportService struct {
	Validator  Validator
	Repository ImportRepository
	Store      ImportObjectStore
	Queue      ImportQueue
	TempRoot   string
	Now        func() time.Time
}

func (s *ImportService) Import(ctx context.Context, input ImportInput) (ImportResult, error) {
	if issues := validateImportInput(input); len(issues) > 0 {
		return ImportResult{}, &ValidationError{Issues: issues}
	}
	if err := os.MkdirAll(s.TempRoot, 0o750); err != nil {
		return ImportResult{}, err
	}
	tempDir, err := os.MkdirTemp(s.TempRoot, "import-*")
	if err != nil {
		return ImportResult{}, err
	}
	defer os.RemoveAll(tempDir)

	archivePath := filepath.Join(tempDir, "archive.zip")
	if err := writeUploadFile(archivePath, input.Archive, s.Validator.Limits.MaxArchiveBytes); err != nil {
		if errors.Is(err, errUploadTooLarge) {
			return ImportResult{}, &ValidationError{Issues: []ValidationIssue{{Field: "archive", Message: "archive exceeds compressed size limit"}}}
		}
		return ImportResult{}, err
	}
	mappingPath := filepath.Join(tempDir, "mapping.xlsx")
	if err := writeUploadFile(mappingPath, input.Mapping, 0); err != nil {
		return ImportResult{}, err
	}

	archiveFile, err := os.Open(archivePath)
	if err != nil {
		return ImportResult{}, err
	}
	mappingFile, err := os.Open(mappingPath)
	if err != nil {
		archiveFile.Close()
		return ImportResult{}, err
	}
	manifest, validationErr := s.Validator.Validate(ctx, archiveFile, mappingFile)
	archiveFile.Close()
	mappingFile.Close()
	if validationErr != nil {
		return ImportResult{}, validationErr
	}

	batchID, err := s.Repository.ReserveBatchID(ctx)
	if err != nil {
		return ImportResult{}, err
	}
	videoIDs, err := s.Repository.ReserveVideoIDs(ctx, len(manifest.Rows))
	if err != nil {
		return ImportResult{}, err
	}
	if len(videoIDs) != len(manifest.Rows) {
		return ImportResult{}, errors.New("reserved knowledge video ID count does not match manifest")
	}

	extracted, err := extractValidatedVideos(archivePath, tempDir, manifest.Rows, videoIDs, s.Validator.Limits.MaxEntryBytes)
	if err != nil {
		return ImportResult{}, err
	}
	now := time.Now()
	if s.Now != nil {
		now = s.Now()
	}
	manifestKey := fmt.Sprintf("manifests/%d/mapping.xlsx", batchID)
	batch := Batch{
		ID:            batchID,
		UploadUserID:  input.UploadUserID,
		ZipFileName:   input.ArchiveName,
		XLSXFileName:  input.MappingName,
		XLSXObjectKey: manifestKey,
		TotalCount:    len(manifest.Rows),
		Status:        BatchProcessing,
		CreateTime:    now,
		UpdateTime:    now,
	}
	videos := make([]Video, 0, len(manifest.Rows))
	for index, row := range manifest.Rows {
		extension := filepath.Ext(row.SourceFileName)
		videos = append(videos, Video{
			ID:                 videoIDs[index],
			BatchID:            batchID,
			KnowledgePointID:   row.KnowledgePointID,
			KnowledgePointName: row.KnowledgePointName,
			SourceFileName:     row.SourceFileName,
			SourceObjectKey:    fmt.Sprintf("raw/%d/source%s", videoIDs[index], extension),
			HLSObjectPrefix:    fmt.Sprintf("hls/%d", videoIDs[index]),
			Status:             VideoPending,
			CreateTime:         now,
			UpdateTime:         now,
		})
	}

	uploadedKeys := make([]string, 0, len(videos)+1)
	compensate := func() {
		cleanupCtx := context.WithoutCancel(ctx)
		for index := len(uploadedKeys) - 1; index >= 0; index-- {
			if deleteErr := s.Store.Delete(cleanupCtx, uploadedKeys[index]); deleteErr != nil {
				zap.L().Error("knowledge_video_import_compensation_failed", zap.String("object_key", uploadedKeys[index]), zap.Error(deleteErr))
			}
		}
	}
	if err := s.Store.PutFile(ctx, manifestKey, mappingPath, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"); err != nil {
		return ImportResult{}, err
	}
	uploadedKeys = append(uploadedKeys, manifestKey)
	for index, video := range videos {
		if err := s.Store.PutFile(ctx, video.SourceObjectKey, extracted[index], videoContentType(filepath.Ext(video.SourceFileName))); err != nil {
			compensate()
			return ImportResult{}, err
		}
		uploadedKeys = append(uploadedKeys, video.SourceObjectKey)
	}
	if err := s.Repository.CreateBatch(ctx, batch, videos); err != nil {
		compensate()
		return ImportResult{}, err
	}

	for _, video := range videos {
		task := TranscodeTask{
			KnowledgeVideoID: video.ID,
			SourceObjectKey:  video.SourceObjectKey,
			HLSObjectPrefix:  video.HLSObjectPrefix,
			TaskID:           fmt.Sprintf("knowledge-video-%d", video.ID),
		}
		if err := s.Queue.Enqueue(ctx, task); err != nil {
			zap.L().Error("knowledge_video_import_enqueue_failed", zap.Uint64("knowledge_video_id", video.ID), zap.Error(err))
			continue
		}
		if _, err := s.Repository.SetEnqueueTime(ctx, video.ID, now); err != nil {
			zap.L().Error("knowledge_video_import_enqueue_time_failed", zap.Uint64("knowledge_video_id", video.ID), zap.Error(err))
		}
	}
	return ImportResult{BatchID: batchID, TotalCount: len(videos), Status: BatchProcessing}, nil
}

func validateImportInput(input ImportInput) []ValidationIssue {
	issues := make([]ValidationIssue, 0)
	if input.UploadUserID == 0 {
		issues = append(issues, ValidationIssue{Field: "upload_user_id", Message: "upload user id must be a positive integer"})
	}
	if input.Archive == nil {
		issues = append(issues, ValidationIssue{Field: "archive", Message: "archive is required"})
	} else if !strings.EqualFold(filepath.Ext(input.ArchiveName), ".zip") {
		issues = append(issues, ValidationIssue{Field: "archive", Message: "archive must be a ZIP file"})
	}
	if input.Mapping == nil {
		issues = append(issues, ValidationIssue{Field: "mapping", Message: "mapping is required"})
	} else if !strings.EqualFold(filepath.Ext(input.MappingName), ".xlsx") {
		issues = append(issues, ValidationIssue{Field: "mapping", Message: "mapping must be an XLSX file"})
	}
	return issues
}

var errUploadTooLarge = errors.New("upload exceeds size limit")

func writeUploadFile(filePath string, reader io.Reader, maxBytes int64) error {
	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	copyReader := reader
	if maxBytes > 0 {
		copyReader = io.LimitReader(reader, maxBytes+1)
	}
	written, copyErr := io.Copy(file, copyReader)
	closeErr := file.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	if maxBytes > 0 && written > maxBytes {
		return errUploadTooLarge
	}
	return nil
}

func extractValidatedVideos(archivePath, tempDir string, rows []ValidatedRow, videoIDs []uint64, maxEntryBytes int64) ([]string, error) {
	archive, err := zip.OpenReader(archivePath)
	if err != nil {
		return nil, err
	}
	defer archive.Close()
	entries := make(map[string]*zip.File, len(archive.File))
	for _, file := range archive.File {
		entries[file.Name] = file
	}
	paths := make([]string, 0, len(rows))
	for index, row := range rows {
		entry := entries[row.ArchiveEntryName]
		if entry == nil {
			return nil, fmt.Errorf("validated archive entry %q is missing", row.ArchiveEntryName)
		}
		input, err := entry.Open()
		if err != nil {
			return nil, err
		}
		outputPath := filepath.Join(tempDir, fmt.Sprintf("source-%d%s", videoIDs[index], filepath.Ext(row.SourceFileName)))
		output, err := os.OpenFile(outputPath, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0o600)
		if err != nil {
			input.Close()
			return nil, err
		}
		limit := int64(row.UncompressedSize) + 1
		if maxEntryBytes > 0 && limit > maxEntryBytes+1 {
			limit = maxEntryBytes + 1
		}
		written, copyErr := io.Copy(output, io.LimitReader(input, limit))
		inputCloseErr := input.Close()
		outputCloseErr := output.Close()
		if copyErr != nil {
			return nil, copyErr
		}
		if inputCloseErr != nil {
			return nil, inputCloseErr
		}
		if outputCloseErr != nil {
			return nil, outputCloseErr
		}
		if uint64(written) != row.UncompressedSize {
			return nil, fmt.Errorf("archive entry %q size changed during extraction", row.ArchiveEntryName)
		}
		paths = append(paths, outputPath)
	}
	return paths, nil
}

func videoContentType(extension string) string {
	switch extension {
	case ".mp4", ".m4v":
		return "video/mp4"
	case ".webm":
		return "video/webm"
	case ".mov":
		return "video/quicktime"
	case ".avi":
		return "video/x-msvideo"
	case ".mkv":
		return "video/x-matroska"
	default:
		return "application/octet-stream"
	}
}
