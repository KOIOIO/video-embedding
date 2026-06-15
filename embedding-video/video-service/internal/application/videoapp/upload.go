package videoapp

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	domainvideo "nlp-video-analysis/internal/domain/video"
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
	rawObjectPrefix := cleanObjectPrefix(s.Paths.RawObjectPrefix)
	hlsObjectPrefixRoot := cleanObjectPrefix(s.Paths.HLSObjectPrefix)
	hlsMasterName := s.Paths.HLSMasterName

	rawAbsPath := filepath.Join(s.Paths.RawDir, datePath, storedFileName)
	rawObjectKey := filepath.ToSlash(filepath.Join(rawObjectPrefix, datePath, storedFileName))
	relativeRawPath := filepath.ToSlash(filepath.Join(datePath, storedFileName))
	rawURL := strings.TrimRight(s.Paths.RawURLPrefix, "/") + "/" + relativeRawPath

	hlsAbsDir := filepath.Join(s.Paths.HLSDir, datePath, hlsDirName)
	hlsObjectPrefix := filepath.ToSlash(filepath.Join(hlsObjectPrefixRoot, datePath, hlsDirName))
	relativeHLSPath := filepath.ToSlash(filepath.Join(datePath, hlsDirName, hlsMasterName))
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
		baseName := filepath.Base(plan.OriginalFileName)
		title = strings.TrimSuffix(baseName, filepath.Ext(baseName))
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
	rawUploadedByFinalize := !plan.RawUploaded

	if err := s.Repo.Create(ctx, v); err != nil {
		if rawUploadedByFinalize {
			rollbackObject(ctx, s.Store, plan.RawObjectKey)
		}
		return UploadResult{}, err
	}

	taskID := strconv.FormatUint(v.ID, 10)
	if err := s.StatusStore.Set(ctx, taskID, domainvideo.StatusUploaded, plan.HLSURL, s.StatusTTL); err != nil {
		s.markUploadBootstrapFailed(ctx, v.ID, err)
		if rawUploadedByFinalize {
			rollbackObject(ctx, s.Store, plan.RawObjectKey)
		}
		return UploadResult{}, err
	}

	if err := s.Queue.Enqueue(ctx, TranscodeTask{
		VideoID:         v.ID,
		RawKey:          plan.RawObjectKey,
		HLSObjectPrefix: plan.HLSObjectPrefix,
		TaskID:          taskID,
		HLSURL:          plan.HLSURL,
	}); err != nil {
		s.markUploadBootstrapFailed(ctx, v.ID, err)
		if rawUploadedByFinalize {
			rollbackObject(ctx, s.Store, plan.RawObjectKey)
		}
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

	if s.DeleteLocal && s.FS != nil && strings.TrimSpace(plan.RawAbsPath) != "" {
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

func (s *Service) markUploadBootstrapFailed(ctx context.Context, videoID uint64, cause error) {
	if s == nil || s.Repo == nil || videoID == 0 {
		return
	}
	msg := "upload bootstrap failed"
	if cause != nil {
		msg = cause.Error()
	}
	rollbackCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	_ = s.Repo.UpdateStatusByID(rollbackCtx, videoID, domainvideo.StatusFailed, msg)
}

func cleanObjectPrefix(value string) string {
	return strings.Trim(strings.TrimSpace(value), "/")
}
