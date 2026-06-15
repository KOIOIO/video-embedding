package service

import (
	"context"
	"strconv"
	"strings"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"legacy-video/internal/application/videoapp"
	domainvideo "legacy-video/internal/domain/video"
	"legacy-video/video"
)

// ListVideos 处理视频列表查询，并把 application 层的过滤枚举映射到 protobuf 枚举。
func (s *VideoService) ListVideos(ctx context.Context, req *video.ListVideosRequest) (*video.ListVideosResponse, error) {
	filter := videoapp.ListAll
	if req.FilterType == video.VideoType_RAW {
		filter = videoapp.ListRawOnly
	} else if req.FilterType == video.VideoType_HLS {
		filter = videoapp.ListHLSOnly
	}

	videoModels, err := s.App.ListVideos(ctx, filter)
	if err != nil {
		return nil, err
	}

	videos := make([]*video.VideoInfo, 0, len(videoModels))
	for _, m := range videoModels {
		videos = append(videos, toProtoVideoInfo(m))
	}

	return &video.ListVideosResponse{
		Videos: videos,
		Total:  int32(len(videos)),
	}, nil
}

// UpdateVideoMetadata 处理视频标题和描述更新请求。
func (s *VideoService) UpdateVideoMetadata(ctx context.Context, req *video.UpdateVideoMetadataRequest) (*video.UpdateVideoMetadataResponse, error) {
	if req == nil || req.GetVideoId() == 0 {
		return nil, status.Error(codes.InvalidArgument, "video_id is required")
	}
	ok, err := s.App.UpdateVideoMetadata(ctx, req.GetVideoId(), req.GetTitle(), req.GetDescription())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	if !ok {
		return nil, status.Error(codes.NotFound, "video not found")
	}
	return &video.UpdateVideoMetadataResponse{
		Success:     true,
		Message:     "ok",
		VideoId:     req.GetVideoId(),
		Title:       strings.TrimSpace(req.GetTitle()),
		Description: req.GetDescription(),
	}, nil
}

// DeleteVideo 处理视频删除请求。
func (s *VideoService) DeleteVideo(ctx context.Context, req *video.DeleteVideoRequest) (*video.DeleteVideoResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	ok, err := s.App.DeleteVideo(ctx, req.GetVideoId(), req.GetTaskId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	if !ok {
		return &video.DeleteVideoResponse{Success: false, Message: "视频不存在"}, nil
	}
	return &video.DeleteVideoResponse{Success: true, Message: "删除成功"}, nil
}

// PlayVideo 返回播放地址，并复用 application 层的计数与状态更新逻辑。
func (s *VideoService) PlayVideo(ctx context.Context, req *video.PlayVideoRequest) (*video.PlayVideoResponse, error) {
	if req == nil || req.GetVideoId() == 0 {
		return nil, status.Error(codes.InvalidArgument, "video_id is required")
	}
	playURL, v, ok, err := s.App.PlayVideo(ctx, req.GetVideoId())
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, status.Error(codes.NotFound, "video not found")
	}
	if strings.TrimSpace(playURL) == "" {
		return nil, status.Error(codes.FailedPrecondition, "video has no playable url")
	}
	return &video.PlayVideoResponse{
		PlayUrl: playURL,
		Video:   toProtoVideoInfo(v),
	}, nil
}

// GetSimilarVideos 返回与指定视频最接近的若干视频。
func (s *VideoService) GetSimilarVideos(ctx context.Context, req *video.GetSimilarVideosRequest) (*video.GetSimilarVideosResponse, error) {
	if req == nil || req.GetVideoId() == 0 {
		return nil, status.Error(codes.InvalidArgument, "video_id is required")
	}
	limit := int(req.GetLimit())
	videos, err := s.App.GetSimilarVideos(ctx, req.GetVideoId(), limit)
	if err != nil {
		return nil, err
	}
	out := make([]*video.VideoInfo, 0, len(videos))
	for _, v := range videos {
		out = append(out, toProtoVideoInfo(v))
	}
	return &video.GetSimilarVideosResponse{
		Videos: out,
		Total:  int32(len(out)),
	}, nil
}

// GetViewCount 返回视频累计观看次数。
func (s *VideoService) GetViewCount(ctx context.Context, req *video.GetViewCountRequest) (*video.GetViewCountResponse, error) {
	if req == nil || req.GetVideoId() == 0 {
		return nil, status.Error(codes.InvalidArgument, "video_id is required")
	}
	vc, ok, err := s.App.GetViewCount(ctx, req.GetVideoId())
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, status.Error(codes.NotFound, "video not found")
	}
	return &video.GetViewCountResponse{
		VideoId:   req.GetVideoId(),
		ViewCount: vc,
	}, nil
}

// ListRecommendPoolVideos 返回推荐池中的视频列表。
func (s *VideoService) ListRecommendPoolVideos(ctx context.Context, req *video.ListRecommendPoolVideosRequest) (*video.ListVideosResponse, error) {
	_ = req
	videoModels, err := s.App.ListRecommendPoolVideos(ctx)
	if err != nil {
		return nil, err
	}
	videos := make([]*video.VideoInfo, 0, len(videoModels))
	for _, m := range videoModels {
		videos = append(videos, toProtoVideoInfo(m))
	}
	return &video.ListVideosResponse{
		Videos: videos,
		Total:  int32(len(videos)),
	}, nil
}

// SetVideoPublished 修改视频发布状态。
func (s *VideoService) SetVideoPublished(ctx context.Context, req *video.SetVideoPublishedRequest) (*video.SetVideoPublishedResponse, error) {
	if req == nil || req.GetVideoId() == 0 {
		return nil, status.Error(codes.InvalidArgument, "video_id is required")
	}
	ok, err := s.App.SetVideoPublished(ctx, req.GetVideoId(), req.GetIsPublished())
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, status.Error(codes.NotFound, "video not found")
	}
	return &video.SetVideoPublishedResponse{
		Success:     true,
		Message:     "ok",
		VideoId:     req.GetVideoId(),
		IsPublished: req.GetIsPublished(),
	}, nil
}

// SetVideoRecommend 修改视频推荐状态，并触发推荐记录的联动更新。
func (s *VideoService) SetVideoRecommend(ctx context.Context, req *video.SetVideoRecommendRequest) (*video.SetVideoRecommendResponse, error) {
	if req == nil || req.GetVideoId() == 0 {
		return nil, status.Error(codes.InvalidArgument, "video_id is required")
	}
	ok, err := s.App.SetVideoRecommend(ctx, req.GetVideoId(), req.GetIsRecommend(), req.GetUserId(), int16(req.GetRecommendLevel()), req.GetRecommendScore())
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, status.Error(codes.NotFound, "video not found")
	}
	return &video.SetVideoRecommendResponse{
		Success:     true,
		Message:     "ok",
		VideoId:     req.GetVideoId(),
		IsRecommend: req.GetIsRecommend(),
	}, nil
}

// SetVideoCover 更新视频封面地址。
func (s *VideoService) SetVideoCover(ctx context.Context, req *video.SetVideoCoverRequest) (*video.SetVideoCoverResponse, error) {
	if req == nil || req.GetVideoId() == 0 {
		return nil, status.Error(codes.InvalidArgument, "video_id is required")
	}
	if strings.TrimSpace(req.GetCoverUrl()) == "" {
		return nil, status.Error(codes.InvalidArgument, "cover_url is required")
	}
	ok, err := s.App.SetVideoCover(ctx, req.GetVideoId(), req.GetCoverUrl())
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, status.Error(codes.NotFound, "video not found")
	}
	return &video.SetVideoCoverResponse{
		Success:  true,
		Message:  "ok",
		VideoId:  req.GetVideoId(),
		CoverUrl: req.GetCoverUrl(),
	}, nil
}

// toProtoVideoInfo 把领域对象转换成 protobuf 响应对象。
func toProtoVideoInfo(v domainvideo.Video) *video.VideoInfo {
	isHls := v.Status == domainvideo.StatusDone
	return &video.VideoInfo{
		Name:          v.Title,
		RawUrl:        v.VideoURL,
		HlsUrl:        "",
		IsHls:         isHls,
		DownloadUrl:   v.VideoURL,
		TaskId:        strconv.FormatUint(v.ID, 10),
		Id:            v.ID,
		ViewCount:     int64(v.ViewCount),
		IsPublished:   v.IsPublished,
		IsRecommend:   v.IsRecommend,
		CreatedAtUnix: toUnixSeconds(v.CreateTime),
		UpdatedAtUnix: toUnixSeconds(v.UpdateTime),
		CoverUrl:      v.CoverURL,
		Title:         v.Title,
		Description:   v.Description,
	}
}

// toUnixSeconds 把时间安全转换为 Unix 秒级时间戳。
func toUnixSeconds(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	return t.Unix()
}
