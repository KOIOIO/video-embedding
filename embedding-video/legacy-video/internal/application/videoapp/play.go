package videoapp

import (
	"context"
	"path/filepath"
	"strconv"
	"strings"

	domainvideo "legacy-video/internal/domain/video"
)

// PlayVideo 返回播放地址，并在读取前后更新视频播放次数与状态。
func (s *Service) PlayVideo(ctx context.Context, videoID uint64) (string, domainvideo.Video, bool, error) {
	v, ok, err := s.Repo.GetByID(ctx, videoID)
	if err != nil || !ok {
		return "", domainvideo.Video{}, ok, err
	}
	if _, _, err := s.Repo.IncrementViewCount(ctx, videoID); err != nil {
		return "", domainvideo.Video{}, false, err
	}
	v, ok, err = s.Repo.GetByID(ctx, videoID)
	if err != nil || !ok {
		return "", domainvideo.Video{}, ok, err
	}

	taskID := strconv.FormatUint(videoID, 10)
	if info, ok, err := s.StatusStore.Get(ctx, taskID); err == nil && ok {
		if info.Status == domainvideo.StatusDone && strings.TrimSpace(info.HLSURL) != "" {
			if v.Status != domainvideo.StatusDone {
				_ = s.Repo.UpdateStatusByID(ctx, videoID, domainvideo.StatusDone, "")
				v.Status = domainvideo.StatusDone
			}
			return info.HLSURL, v, true, nil
		}
	}

	if v.Status == domainvideo.StatusDone {
		return deriveHLSURLFromRaw(v.VideoURL, s.Paths.HLSURLPrefix), v, true, nil
	}
	return v.VideoURL, v, true, nil
}

// GetViewCount 返回指定视频的累计观看次数。
func (s *Service) GetViewCount(ctx context.Context, videoID uint64) (int64, bool, error) {
	vc, ok, err := s.Repo.GetViewCount(ctx, videoID)
	return int64(vc), ok, err
}

// GetTranscodeStatus 读取缓存中的转码状态。
func (s *Service) GetTranscodeStatus(ctx context.Context, taskID string) (TranscodeStatus, bool, error) {
	return s.StatusStore.Get(ctx, taskID)
}

// deriveHLSURLFromRaw 在缓存缺失但数据库状态已完成时，按约定路径推导 HLS URL。
func deriveHLSURLFromRaw(rawURL string, hlsPrefix string) string {
	s := strings.TrimSpace(rawURL)
	if s == "" {
		return ""
	}
	prefix := strings.TrimRight(hlsPrefix, "/")
	s = strings.TrimPrefix(s, "/videos/")
	s = strings.TrimPrefix(s, "raw/")
	s = strings.TrimPrefix(s, "/")
	parts := strings.Split(s, "/")
	if len(parts) < 4 {
		return ""
	}
	datePath := strings.Join(parts[:3], "/")
	fileName := parts[3]
	base := strings.TrimSuffix(fileName, filepath.Ext(fileName))
	return prefix + "/" + datePath + "/" + base + "/master.m3u8"
}
