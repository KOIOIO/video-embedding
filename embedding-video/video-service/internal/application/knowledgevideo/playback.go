package knowledgevideo

import (
	"context"
	"math"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

type PlaybackRepository interface {
	DictionaryLookup
	ReadyVideosResolver
	GetVideo(ctx context.Context, id uint64) (Video, bool, error)
	PlaybackRecorder
	UserExists(ctx context.Context, userID uint64) (bool, error)
	UpsertWatchSession(ctx context.Context, report WatchSessionReport, duration int) (WatchSessionAggregate, error)
}

var watchSessionPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{16,64}$`)

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

func (s PlaybackService) ReportWatchSession(ctx context.Context, input WatchSessionInput) (WatchSessionResult, error) {
	if input.UserID == 0 {
		return WatchSessionResult{}, &InvalidArgument{Field: "user_id"}
	}
	if input.KnowledgeVideoID == 0 {
		return WatchSessionResult{}, &InvalidArgument{Field: "knowledge_video_id"}
	}
	if !watchSessionPattern.MatchString(input.SessionID) {
		return WatchSessionResult{}, &InvalidArgument{Field: "session_id"}
	}
	if input.WatchedSeconds < 0 {
		return WatchSessionResult{}, &InvalidArgument{Field: "watched_seconds"}
	}
	userExists, err := s.Repo.UserExists(ctx, input.UserID)
	if err != nil {
		return WatchSessionResult{}, err
	}
	if !userExists {
		return WatchSessionResult{}, &NotFound{Resource: "user"}
	}
	video, found, err := s.Repo.GetVideo(ctx, input.KnowledgeVideoID)
	if err != nil {
		return WatchSessionResult{}, err
	}
	if !found {
		return WatchSessionResult{}, &NotFound{Resource: "knowledge video"}
	}
	if video.Status != VideoReady || video.Duration <= 0 {
		return WatchSessionResult{}, &NotReady{Status: video.Status}
	}
	watchedSeconds := input.WatchedSeconds
	if watchedSeconds > video.Duration {
		watchedSeconds = video.Duration
	}
	now := time.Now()
	if s.Now != nil {
		now = s.Now()
	}
	aggregate, err := s.Repo.UpsertWatchSession(ctx, WatchSessionReport{
		UserID: input.UserID, KnowledgePointID: video.KnowledgePointID,
		KnowledgeVideoID: video.ID, SessionID: input.SessionID,
		WatchedSeconds: watchedSeconds, UpdatedAt: now,
	}, video.Duration)
	if err != nil {
		return WatchSessionResult{}, err
	}
	return WatchSessionResult{
		SessionID: input.SessionID, SessionWatchedSeconds: aggregate.SessionWatchedSeconds,
		TotalWatchedSeconds: aggregate.TotalWatchedSeconds, DurationSeconds: video.Duration,
		ProgressRatio:  math.Min(1, float64(aggregate.TotalWatchedSeconds)/float64(video.Duration)),
		EffectiveWatch: aggregate.TotalWatchedSeconds*100 >= video.Duration*60,
	}, nil
}
