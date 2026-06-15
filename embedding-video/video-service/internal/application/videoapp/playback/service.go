package playback

import (
	"context"
	"path/filepath"
	"strconv"
	"strings"

	domainvideo "nlp-video-analysis/internal/domain/video"
)

type Status struct {
	Status domainvideo.Status
	HLSURL string
}

type VideoRepository interface {
	GetByID(ctx context.Context, id uint64) (domainvideo.Video, bool, error)
	IncrementViewCount(ctx context.Context, id uint64) (int, bool, error)
	UpdateStatusByID(ctx context.Context, id uint64, status domainvideo.Status, errMsg string) error
	GetViewCount(ctx context.Context, id uint64) (int, bool, error)
}

type StatusStore interface {
	Get(ctx context.Context, taskID string) (Status, bool, error)
}

type Service struct {
	Repo          VideoRepository
	StatusStore   StatusStore
	RawURLPrefix  string
	HLSURLPrefix  string
	HLSMasterName string
}

func (s Service) PlayVideo(ctx context.Context, videoID uint64) (string, domainvideo.Video, bool, error) {
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
		return deriveHLSURLFromRaw(v.VideoURL, s.RawURLPrefix, s.HLSURLPrefix, s.HLSMasterName), v, true, nil
	}
	return v.VideoURL, v, true, nil
}

func (s Service) ResolvePlaybackURL(ctx context.Context, v domainvideo.Video) string {
	if info, ok, err := s.StatusStore.Get(ctx, strconv.FormatUint(v.ID, 10)); err == nil && ok {
		if info.Status == domainvideo.StatusDone && strings.TrimSpace(info.HLSURL) != "" {
			return info.HLSURL
		}
	}
	if v.Status == domainvideo.StatusDone {
		return deriveHLSURLFromRaw(v.VideoURL, s.RawURLPrefix, s.HLSURLPrefix, s.HLSMasterName)
	}
	return v.VideoURL
}

func (s Service) GetViewCount(ctx context.Context, videoID uint64) (int64, bool, error) {
	vc, ok, err := s.Repo.GetViewCount(ctx, videoID)
	return int64(vc), ok, err
}

func (s Service) GetTranscodeStatus(ctx context.Context, taskID string) (Status, bool, error) {
	return s.StatusStore.Get(ctx, taskID)
}

func deriveHLSURLFromRaw(rawURL string, rawPrefix string, hlsPrefix string, masterName string) string {
	value := strings.TrimSpace(rawURL)
	if value == "" {
		return ""
	}
	rawPrefix = strings.TrimRight(strings.TrimSpace(rawPrefix), "/")
	prefix := strings.TrimRight(hlsPrefix, "/")
	if rawPrefix == "" {
		rawPrefix = "/videos/raw"
	}
	if prefix == "" {
		prefix = "/videos/hls"
	}
	masterName = strings.TrimSpace(masterName)
	if masterName == "" {
		masterName = "master.m3u8"
	}
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
