package videoapp

import (
	"archive/zip"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

type chunkedUploadSession struct {
	UploadID    string     `json:"upload_id"`
	FileName    string     `json:"file_name"`
	ContentType string     `json:"content_type,omitempty"`
	Title       string     `json:"title,omitempty"`
	Description string     `json:"description,omitempty"`
	UserID      uint64     `json:"user_id,omitempty"`
	FileSize    int64      `json:"file_size"`
	ChunkSize   int64      `json:"chunk_size"`
	TotalChunks int        `json:"total_chunks"`
	Plan        UploadPlan `json:"plan"`
	CreatedAt   time.Time  `json:"created_at"`
}

type archiveBatchSession struct {
	BatchID   string    `json:"batch_id"`
	VideoIDs  []uint64  `json:"video_ids"`
	CreatedAt time.Time `json:"created_at"`
}

func (s *Service) InitiateChunkedUpload(ctx context.Context, input InitiateChunkedUploadInput) (ChunkedUploadStatus, error) {
	if err := validateChunkedUploadInit(input); err != nil {
		return ChunkedUploadStatus{}, err
	}
	userID := input.UserID
	if userID == 0 {
		userID = DefaultUploadUserID
	}
	if err := s.ensureUploadAllowed(ctx, userID); err != nil {
		return ChunkedUploadStatus{}, err
	}

	plan, err := s.BuildUploadPlan(input.FileName)
	if err != nil {
		return ChunkedUploadStatus{}, InvalidArgumentError("file is required")
	}

	uploadID, err := newChunkedUploadID()
	if err != nil {
		return ChunkedUploadStatus{}, err
	}
	session := chunkedUploadSession{
		UploadID:    uploadID,
		FileName:    input.FileName,
		ContentType: strings.TrimSpace(input.ContentType),
		Title:       input.Title,
		Description: input.Description,
		UserID:      userID,
		FileSize:    input.FileSize,
		ChunkSize:   input.ChunkSize,
		TotalChunks: input.TotalChunks,
		Plan:        plan,
		CreatedAt:   s.Now(),
	}
	if err := s.writeChunkedUploadSession(session); err != nil {
		return ChunkedUploadStatus{}, err
	}
	return s.chunkedUploadStatus(session)
}

func (s *Service) InitiateChunkedArchiveUpload(ctx context.Context, input InitiateChunkedUploadInput) (ChunkedUploadStatus, error) {
	if !isZipFileName(input.FileName) {
		return ChunkedUploadStatus{}, InvalidArgumentError("zip archive is required")
	}
	if err := validateChunkedUploadInit(input); err != nil {
		return ChunkedUploadStatus{}, err
	}
	userID := input.UserID
	if userID == 0 {
		userID = DefaultUploadUserID
	}
	if err := s.ensureUploadAllowed(ctx, userID); err != nil {
		return ChunkedUploadStatus{}, err
	}

	uploadID, err := newChunkedUploadID()
	if err != nil {
		return ChunkedUploadStatus{}, err
	}
	session := chunkedUploadSession{
		UploadID:    uploadID,
		FileName:    input.FileName,
		ContentType: strings.TrimSpace(input.ContentType),
		Description: input.Description,
		UserID:      userID,
		FileSize:    input.FileSize,
		ChunkSize:   input.ChunkSize,
		TotalChunks: input.TotalChunks,
		Plan: UploadPlan{
			OriginalFileName: input.FileName,
			RawAbsPath:       filepath.Join(s.chunkedUploadSessionDir(uploadID), "archive.zip"),
		},
		CreatedAt: s.Now(),
	}
	if err := s.writeChunkedUploadSession(session); err != nil {
		return ChunkedUploadStatus{}, err
	}
	return s.chunkedUploadStatus(session)
}

func (s *Service) UploadVideoChunk(_ context.Context, input UploadVideoChunkInput) (ChunkedUploadStatus, error) {
	if input.Reader == nil {
		return ChunkedUploadStatus{}, InvalidArgumentError("chunk is required")
	}
	session, err := s.loadChunkedUploadSession(input.UploadID)
	if err != nil {
		return ChunkedUploadStatus{}, err
	}
	if input.ChunkIndex < 0 || input.ChunkIndex >= session.TotalChunks {
		return ChunkedUploadStatus{}, InvalidArgumentError("chunk_index is invalid")
	}

	chunkPath := s.chunkedUploadChunkPath(session.UploadID, input.ChunkIndex)
	if err := os.MkdirAll(filepath.Dir(chunkPath), 0755); err != nil {
		return ChunkedUploadStatus{}, err
	}
	tempPath := chunkPath + ".tmp"
	file, err := os.Create(tempPath)
	if err != nil {
		return ChunkedUploadStatus{}, err
	}
	written, copyErr := io.Copy(file, input.Reader)
	closeErr := file.Close()
	if copyErr != nil {
		_ = os.Remove(tempPath)
		return ChunkedUploadStatus{}, copyErr
	}
	if closeErr != nil {
		_ = os.Remove(tempPath)
		return ChunkedUploadStatus{}, closeErr
	}
	if written != expectedChunkSize(session, input.ChunkIndex) {
		_ = os.Remove(tempPath)
		return ChunkedUploadStatus{}, InvalidArgumentError("chunk size is invalid")
	}
	if err := os.Rename(tempPath, chunkPath); err != nil {
		_ = os.Remove(tempPath)
		return ChunkedUploadStatus{}, err
	}
	return s.chunkedUploadStatus(session)
}

func (s *Service) GetChunkedUploadStatus(_ context.Context, uploadID string) (ChunkedUploadStatus, error) {
	session, err := s.loadChunkedUploadSession(uploadID)
	if err != nil {
		return ChunkedUploadStatus{}, err
	}
	return s.chunkedUploadStatus(session)
}

func (s *Service) CompleteChunkedUpload(ctx context.Context, input CompleteChunkedUploadInput) (UploadResult, error) {
	session, err := s.loadChunkedUploadSession(input.UploadID)
	if err != nil {
		return UploadResult{}, err
	}
	if err := s.assembleChunkedUploadFile(session); err != nil {
		return UploadResult{}, err
	}
	result, err := s.FinalizeUpload(ctx, session.Plan, UploadMeta{
		Title:       session.Title,
		Description: session.Description,
		UserID:      session.UserID,
	})
	if err != nil {
		return UploadResult{}, err
	}
	_ = os.RemoveAll(s.chunkedUploadSessionDir(session.UploadID))
	return result, nil
}

func (s *Service) CompleteChunkedArchiveUpload(ctx context.Context, input CompleteChunkedUploadInput) (ArchiveUploadResult, error) {
	session, err := s.loadChunkedUploadSession(input.UploadID)
	if err != nil {
		return ArchiveUploadResult{}, err
	}
	if !isZipFileName(session.FileName) {
		return ArchiveUploadResult{}, InvalidArgumentError("zip archive is required")
	}
	if err := s.assembleChunkedUploadFile(session); err != nil {
		return ArchiveUploadResult{}, err
	}

	result, err := s.importVideoArchiveFile(ctx, session.Plan.RawAbsPath, session.Description, session.UserID)
	if err != nil {
		return ArchiveUploadResult{}, err
	}
	_ = os.RemoveAll(s.chunkedUploadSessionDir(session.UploadID))
	return result, nil
}

func validateChunkedUploadInit(input InitiateChunkedUploadInput) error {
	fileName := strings.TrimSpace(input.FileName)
	if fileName == "" {
		return InvalidArgumentError("file is required")
	}
	if isArchiveMetadataEntryName(fileName) {
		return InvalidArgumentError("unsupported metadata file")
	}
	if input.FileSize <= 0 {
		return InvalidArgumentError("file_size is required")
	}
	if input.ChunkSize <= 0 {
		return InvalidArgumentError("chunk_size is required")
	}
	if input.TotalChunks <= 0 {
		return InvalidArgumentError("total_chunks is required")
	}
	expectedChunks := int((input.FileSize + input.ChunkSize - 1) / input.ChunkSize)
	if input.TotalChunks != expectedChunks {
		return InvalidArgumentError("total_chunks does not match file_size and chunk_size")
	}
	return nil
}

func (s *Service) assembleChunkedUploadFile(session chunkedUploadSession) error {
	status, err := s.chunkedUploadStatus(session)
	if err != nil {
		return err
	}
	if !status.Completed {
		return InvalidArgumentError("upload is incomplete")
	}
	if err := os.MkdirAll(filepath.Dir(session.Plan.RawAbsPath), 0755); err != nil {
		return err
	}
	output, err := os.Create(session.Plan.RawAbsPath)
	if err != nil {
		return err
	}
	for index := 0; index < session.TotalChunks; index++ {
		chunk, err := os.Open(s.chunkedUploadChunkPath(session.UploadID, index))
		if err != nil {
			_ = output.Close()
			return err
		}
		_, copyErr := io.Copy(output, chunk)
		closeErr := chunk.Close()
		if copyErr != nil {
			_ = output.Close()
			return copyErr
		}
		if closeErr != nil {
			_ = output.Close()
			return closeErr
		}
	}
	return output.Close()
}

func (s *Service) importVideoArchiveFile(ctx context.Context, archivePath string, description string, userID uint64) (ArchiveUploadResult, error) {
	zr, err := zip.OpenReader(archivePath)
	if err != nil {
		return ArchiveUploadResult{}, InvalidArgumentError("invalid zip archive")
	}
	defer zr.Close()

	result := ArchiveUploadResult{Total: len(zr.File)}
	for _, entry := range zr.File {
		name := strings.TrimSpace(entry.Name)
		if entry.FileInfo().IsDir() || !isSafeArchiveEntryName(name) || isArchiveMetadataEntryName(name) || !isSupportedVideoFileName(name) {
			result.Skipped = append(result.Skipped, name)
			continue
		}
		rc, err := entry.Open()
		if err != nil {
			result.Failed = append(result.Failed, ArchiveUploadFailure{FileName: name, Error: err.Error()})
			continue
		}
		uploadResult, err := s.UploadVideo(ctx, UploadVideoInput{
			FileName:    filepath.Base(name),
			ContentType: contentTypeFromVideoExtension(filepath.Ext(name)),
			Title:       strings.TrimSuffix(filepath.Base(name), filepath.Ext(name)),
			Description: description,
			UserID:      userID,
			Reader:      rc,
		})
		_ = rc.Close()
		if err != nil {
			result.Failed = append(result.Failed, ArchiveUploadFailure{FileName: name, Error: err.Error()})
			continue
		}
		uploadResult.Name = filepath.Base(name)
		result.Uploaded = append(result.Uploaded, uploadResult)
	}
	if len(result.Uploaded) > 0 {
		batchID, err := newChunkedUploadID()
		if err != nil {
			return ArchiveUploadResult{}, err
		}
		result.BatchID = batchID
		if err := s.writeArchiveBatchSession(archiveBatchSession{
			BatchID:   batchID,
			VideoIDs:  archiveVideoIDs(result.Uploaded),
			CreatedAt: s.Now(),
		}); err != nil {
			return ArchiveUploadResult{}, err
		}
	}
	return result, nil
}

func (s *Service) GetArchiveProcessingProgress(ctx context.Context, batchID string) (ArchiveProcessingProgress, error) {
	batchID = strings.TrimSpace(batchID)
	if !isSafeChunkedUploadID(batchID) {
		return ArchiveProcessingProgress{}, InvalidArgumentError("batch_id is invalid")
	}
	session, err := s.loadArchiveBatchSession(batchID)
	if err != nil {
		return ArchiveProcessingProgress{}, err
	}
	repo, ok := s.Repo.(ArchiveProcessingProgressRepository)
	if !ok {
		return ArchiveProcessingProgress{}, fmt.Errorf("archive processing progress is not supported")
	}
	return repo.GetArchiveProcessingProgress(ctx, session.VideoIDs)
}

func (s *Service) writeChunkedUploadSession(session chunkedUploadSession) error {
	root := s.chunkedUploadSessionDir(session.UploadID)
	if err := os.MkdirAll(filepath.Join(root, "chunks"), 0755); err != nil {
		return err
	}
	payload, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(root, "session.json"), payload, 0644)
}

func (s *Service) writeArchiveBatchSession(session archiveBatchSession) error {
	root := s.archiveBatchRoot()
	if err := os.MkdirAll(root, 0755); err != nil {
		return err
	}
	payload, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(root, session.BatchID+".json"), payload, 0644)
}

func (s *Service) loadChunkedUploadSession(uploadID string) (chunkedUploadSession, error) {
	if !isSafeChunkedUploadID(uploadID) {
		return chunkedUploadSession{}, InvalidArgumentError("upload_id is invalid")
	}
	payload, err := os.ReadFile(filepath.Join(s.chunkedUploadSessionDir(uploadID), "session.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return chunkedUploadSession{}, InvalidArgumentError("upload not found")
		}
		return chunkedUploadSession{}, err
	}
	var session chunkedUploadSession
	if err := json.Unmarshal(payload, &session); err != nil {
		return chunkedUploadSession{}, err
	}
	if session.UploadID != uploadID || !isSafeChunkedUploadID(session.UploadID) {
		return chunkedUploadSession{}, InvalidArgumentError("upload_id is invalid")
	}
	return session, nil
}

func (s *Service) loadArchiveBatchSession(batchID string) (archiveBatchSession, error) {
	payload, err := os.ReadFile(filepath.Join(s.archiveBatchRoot(), batchID+".json"))
	if err != nil {
		if os.IsNotExist(err) {
			return archiveBatchSession{}, InvalidArgumentError("archive batch not found")
		}
		return archiveBatchSession{}, err
	}
	var session archiveBatchSession
	if err := json.Unmarshal(payload, &session); err != nil {
		return archiveBatchSession{}, err
	}
	if session.BatchID != batchID || !isSafeChunkedUploadID(session.BatchID) {
		return archiveBatchSession{}, InvalidArgumentError("batch_id is invalid")
	}
	return session, nil
}

func (s *Service) chunkedUploadStatus(session chunkedUploadSession) (ChunkedUploadStatus, error) {
	uploaded, err := s.listUploadedChunks(session)
	if err != nil {
		return ChunkedUploadStatus{}, err
	}
	return ChunkedUploadStatus{
		UploadID:       session.UploadID,
		FileName:       session.FileName,
		FileSize:       session.FileSize,
		ChunkSize:      session.ChunkSize,
		TotalChunks:    session.TotalChunks,
		UploadedChunks: uploaded,
		Completed:      len(uploaded) == session.TotalChunks,
	}, nil
}

func (s *Service) listUploadedChunks(session chunkedUploadSession) ([]int, error) {
	entries, err := os.ReadDir(filepath.Join(s.chunkedUploadSessionDir(session.UploadID), "chunks"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	uploaded := make([]int, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".part") {
			continue
		}
		indexText := strings.TrimSuffix(entry.Name(), ".part")
		index, err := strconv.Atoi(indexText)
		if err != nil || index < 0 || index >= session.TotalChunks {
			continue
		}
		info, err := entry.Info()
		if err != nil || info.Size() != expectedChunkSize(session, index) {
			continue
		}
		uploaded = append(uploaded, index)
	}
	sort.Ints(uploaded)
	return uploaded, nil
}

func (s *Service) chunkedUploadRoot() string {
	return filepath.Join(s.Paths.RawDir, ".uploads")
}

func (s *Service) archiveBatchRoot() string {
	return filepath.Join(s.chunkedUploadRoot(), "archive_batches")
}

func (s *Service) chunkedUploadSessionDir(uploadID string) string {
	return filepath.Join(s.chunkedUploadRoot(), uploadID)
}

func (s *Service) chunkedUploadChunkPath(uploadID string, chunkIndex int) string {
	return filepath.Join(s.chunkedUploadSessionDir(uploadID), "chunks", fmt.Sprintf("%d.part", chunkIndex))
}

func expectedChunkSize(session chunkedUploadSession, chunkIndex int) int64 {
	if chunkIndex == session.TotalChunks-1 {
		remainder := session.FileSize - int64(chunkIndex)*session.ChunkSize
		if remainder > 0 {
			return remainder
		}
	}
	return session.ChunkSize
}

func newChunkedUploadID() (string, error) {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf[:]), nil
}

func archiveVideoIDs(videos []UploadResult) []uint64 {
	ids := make([]uint64, 0, len(videos))
	for _, video := range videos {
		if video.VideoID > 0 {
			ids = append(ids, video.VideoID)
		}
	}
	return ids
}

func isSafeChunkedUploadID(uploadID string) bool {
	uploadID = strings.TrimSpace(uploadID)
	if uploadID == "" {
		return false
	}
	for _, r := range uploadID {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			continue
		}
		return false
	}
	return true
}
