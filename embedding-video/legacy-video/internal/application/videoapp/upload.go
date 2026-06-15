package videoapp

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strconv"
	"strings"

	domainvideo "legacy-video/internal/domain/video"
)

// BuildUploadPlan 根据原始文件名预先计算本地落盘路径、对象存储键和未来播放 URL。
func (s *Service) BuildUploadPlan(originalFileName string) (UploadPlan, error) {
	now := s.Now()
	if strings.TrimSpace(originalFileName) == "" {
		return UploadPlan{}, errors.New("originalFileName is required")
	}

	ts := now.UnixNano()
	datePath := now.Format("2006/01/02")
	ext := filepath.Ext(originalFileName)
	storedFileName := fmt.Sprintf("%d%s", ts, ext)
	hlsDirName := fmt.Sprintf("%d", ts)

	rawAbsPath := filepath.Join(s.Paths.RawDir, datePath, storedFileName)
	rawObjectKey := filepath.ToSlash(filepath.Join("raw", datePath, storedFileName))
	relativeRawPath := filepath.ToSlash(filepath.Join(datePath, storedFileName))
	rawURL := strings.TrimRight(s.Paths.RawURLPrefix, "/") + "/" + relativeRawPath

	hlsAbsDir := filepath.Join(s.Paths.HLSDir, datePath, hlsDirName)
	hlsObjectPrefix := filepath.ToSlash(filepath.Join("hls", datePath, hlsDirName))
	relativeHLSPath := filepath.ToSlash(filepath.Join(datePath, hlsDirName, "master.m3u8"))
	hlsURL := strings.TrimRight(s.Paths.HLSURLPrefix, "/") + "/" + relativeHLSPath

	return UploadPlan{
		OriginalFileName: originalFileName,
		StoredFileName:   storedFileName,
		DatePath:         datePath,
		RawAbsPath:       rawAbsPath,
		RawObjectKey:     rawObjectKey,
		RawURL:           rawURL,
		HLSAbsDir:        hlsAbsDir,
		HLSObjectPrefix:  hlsObjectPrefix,
		HLSURL:           hlsURL,
		RawUploaded:      false,
	}, nil
}

// OpenUploadWriter 为原始视频内容打开本地临时写入器。
func (s *Service) OpenUploadWriter(plan UploadPlan) (io.WriteCloser, error) {
	if err := s.FS.MkdirAll(filepath.Dir(plan.RawAbsPath)); err != nil {
		return nil, err
	}
	return s.FS.Create(plan.RawAbsPath)
}

// FinalizeUpload 完成上传收尾工作。
// 这里会完成原视频上云、视频记录入库、转码入队和向量化入队。
func (s *Service) FinalizeUpload(ctx context.Context, plan UploadPlan, meta UploadMeta) (UploadResult, error) {
	now := s.Now()
	title := strings.TrimSpace(meta.Title)
	if title == "" {
		title = plan.OriginalFileName
	}
	v, err := domainvideo.NewUploaded(title, meta.Description, plan.RawURL, now)
	if err != nil {
		return UploadResult{}, err
	}
	v.IsPublished = true
	v.IsRecommend = false

	if !plan.RawUploaded {
		if err := s.Store.PutFile(ctx, plan.RawObjectKey, plan.RawAbsPath, ""); err != nil {
			return UploadResult{}, err
		}
	}

	if err := s.Repo.Create(ctx, v); err != nil {
		return UploadResult{}, err
	}

	taskID := strconv.FormatUint(v.ID, 10)
	if err := s.StatusStore.Set(ctx, taskID, domainvideo.StatusUploaded, plan.HLSURL, s.StatusTTL); err != nil {
		return UploadResult{}, err
	}

	if err := s.Queue.Enqueue(ctx, TranscodeTask{
		VideoID:         v.ID,
		RawKey:          plan.RawObjectKey,
		HLSObjectPrefix: plan.HLSObjectPrefix,
		TaskID:          taskID,
		HLSURL:          plan.HLSURL,
	}); err != nil {
		return UploadResult{}, err
	}

	// 向量化失败不影响主上传链路，因此这里维持尽力而为的入队策略。
	if s.VectorQueue != nil {
		_ = s.VectorQueue.Enqueue(ctx, VectorizeTask{
			VideoID: v.ID,
			RawKey:  plan.RawObjectKey,
			TaskID:  taskID,
		})
	}

	if s.DeleteLocal && strings.TrimSpace(plan.RawAbsPath) != "" {
		_ = s.FS.Remove(plan.RawAbsPath)
	}

	return UploadResult{
		VideoID: v.ID,
		TaskID:  taskID,
		RawURL:  plan.RawURL,
		HLSURL:  plan.HLSURL,
		Name:    plan.StoredFileName,
	}, nil
}
