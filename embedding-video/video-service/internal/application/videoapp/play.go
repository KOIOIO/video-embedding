package videoapp

import (
	"context"
	"path/filepath"
	"strings"

	playbackapp "nlp-video-analysis/internal/application/videoapp/playback"
	domainvideo "nlp-video-analysis/internal/domain/video"
)

// PlayVideo 返回播放地址，并在读取前后更新视频播放次数与状态。
func (s *Service) PlayVideo(ctx context.Context, videoID uint64) (string, domainvideo.Video, bool, error) {
	return newPlaybackService(s).PlayVideo(ctx, videoID)
}

// ResolvePlaybackURL 在不产生副作用的前提下为视频推导可播放地址。
func (s *Service) ResolvePlaybackURL(ctx context.Context, v domainvideo.Video) string {
	return newPlaybackService(s).ResolvePlaybackURL(ctx, v)
}

// GetViewCount 返回指定视频的累计观看次数。
func (s *Service) GetViewCount(ctx context.Context, videoID uint64) (int64, bool, error) {
	return newPlaybackService(s).GetViewCount(ctx, videoID)
}

// GetTranscodeStatus 读取缓存中的转码状态。
func (s *Service) GetTranscodeStatus(ctx context.Context, taskID string) (TranscodeStatus, bool, error) {
	status, ok, err := newPlaybackService(s).GetTranscodeStatus(ctx, taskID)
	if err != nil || !ok {
		return TranscodeStatus{}, ok, err
	}
	return TranscodeStatus{Status: status.Status, HLSURL: status.HLSURL}, true, nil
}

func newPlaybackService(s *Service) playbackapp.Service {
	return playbackapp.Service{
		Repo:          playbackRepositoryAdapter{repo: s.Repo},
		StatusStore:   playbackStatusStoreAdapter{store: s.StatusStore},
		RawURLPrefix:  s.Paths.RawURLPrefix,
		HLSURLPrefix:  s.Paths.HLSURLPrefix,
		HLSMasterName: s.Paths.HLSMasterName,
	}
}

func deriveHLSURLFromRaw(rawURL string, rawPrefix string, hlsPrefix string, masterName string) string {
	value := strings.TrimSpace(rawURL)
	if value == "" {
		return ""
	}
	rawPrefix = strings.TrimRight(strings.TrimSpace(rawPrefix), "/")
	prefix := strings.TrimRight(hlsPrefix, "/")
	if strings.HasPrefix(value, rawPrefix+"/") {
		value = strings.TrimPrefix(value, rawPrefix+"/")
	} else {
		value = strings.TrimPrefix(value, "/videos/")
		value = strings.TrimPrefix(value, "raw/")
	}
	value = strings.TrimPrefix(value, "/")
	parts := strings.Split(value, "/")
	if len(parts) < 4 {
		return ""
	}
	datePath := strings.Join(parts[:3], "/")
	fileName := parts[3]
	base := strings.TrimSuffix(fileName, filepath.Ext(fileName))
	return prefix + "/" + datePath + "/" + base + "/" + masterName
}

type playbackRepositoryAdapter struct {
	repo VideoRepository
}

func (a playbackRepositoryAdapter) GetByID(ctx context.Context, id uint64) (domainvideo.Video, bool, error) {
	return a.repo.GetByID(ctx, id)
}

func (a playbackRepositoryAdapter) IncrementViewCount(ctx context.Context, id uint64) (int, bool, error) {
	return a.repo.IncrementViewCount(ctx, id)
}

func (a playbackRepositoryAdapter) UpdateStatusByID(ctx context.Context, id uint64, status domainvideo.Status, errMsg string) error {
	return a.repo.UpdateStatusByID(ctx, id, status, errMsg)
}

func (a playbackRepositoryAdapter) GetViewCount(ctx context.Context, id uint64) (int, bool, error) {
	return a.repo.GetViewCount(ctx, id)
}

type playbackStatusStoreAdapter struct {
	store TranscodeStatusStore
}

func (a playbackStatusStoreAdapter) Get(ctx context.Context, taskID string) (playbackapp.Status, bool, error) {
	status, ok, err := a.store.Get(ctx, taskID)
	if err != nil || !ok {
		return playbackapp.Status{}, ok, err
	}
	return playbackapp.Status{Status: status.Status, HLSURL: status.HLSURL}, true, nil
}
