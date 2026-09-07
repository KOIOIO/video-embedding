package knowledgevideo

import (
	"context"
	"path/filepath"
	"strings"
	"time"
)

type PlaybackRepository interface {
	DictionaryLookup
	ReadyVideosResolver
	GetVideo(ctx context.Context, id uint64) (Video, bool, error)
	PlaybackRecorder
}

type PlaybackService struct {
	Repo             PlaybackRepository
	MediaRoutePrefix string
	Now              func() time.Time
}

func (s PlaybackService) List(ctx context.Context, knowledgePointID uint64) (PlaybackResolution, error) {
	if knowledgePointID == 0 {
		return PlaybackResolution{}, &InvalidArgument{Field: "knowledge_point_id"}
	}
	points, err := s.Repo.LookupKnowledgePoints(ctx, []uint64{knowledgePointID})
	if err != nil {
		return PlaybackResolution{}, err
	}
	if len(points) == 0 {
		return PlaybackResolution{}, &NotFound{Resource: "knowledge point"}
	}
	videos, err := s.Repo.ListReadyByKnowledgePoint(ctx, knowledgePointID)
	if err != nil {
		return PlaybackResolution{}, err
	}
	prefix := "/" + strings.Trim(s.MediaRoutePrefix, "/")
	if prefix == "/" {
		prefix = "/knowledge-video-media"
	}
	result := PlaybackResolution{
		KnowledgePointID:   knowledgePointID,
		KnowledgePointName: points[0].Name,
		Videos:             make([]PlaybackVideo, 0, len(videos)),
	}
	for _, video := range videos {
		result.Videos = append(result.Videos, PlaybackVideo{
			KnowledgeVideoID: video.ID,
			SourceFileName:   video.SourceFileName,
			DisplayName:      strings.TrimSuffix(video.SourceFileName, filepath.Ext(video.SourceFileName)),
			Duration:         video.Duration,
			PlaybackURL:      prefix + "/" + strings.TrimLeft(video.HLSMasterObjectKey, "/"),
		})
	}
	return result, nil
}

// Resolve preserves the old application method while making lookup read-only.
func (s PlaybackService) Resolve(ctx context.Context, _ uint64, knowledgePointID uint64) (PlaybackResolution, error) {
	return s.List(ctx, knowledgePointID)
}

func (s PlaybackService) Record(ctx context.Context, userID, knowledgeVideoID uint64) error {
	if userID == 0 {
		return &InvalidArgument{Field: "user_id"}
	}
	if knowledgeVideoID == 0 {
		return &InvalidArgument{Field: "knowledge_video_id"}
	}
	video, found, err := s.Repo.GetVideo(ctx, knowledgeVideoID)
	if err != nil {
		return err
	}
	if !found {
		return &NotFound{Resource: "knowledge video"}
	}
	if video.Status != VideoReady {
		return &NotReady{Status: video.Status}
	}
	now := time.Now()
	if s.Now != nil {
		now = s.Now()
	}
	return s.Repo.RecordPlayback(ctx, PlayRecord{UserID: userID, KnowledgePointID: video.KnowledgePointID, KnowledgeVideoID: video.ID, CreateTime: now})
}
