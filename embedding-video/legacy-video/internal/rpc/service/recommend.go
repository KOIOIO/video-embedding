package service

import (
	"context"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"legacy-video/internal/application/videoapp"
	"legacy-video/video"
)

// RecommendByQuestion 根据题目文本或题目 ID 召回相关视频片段。
func (s *VideoService) RecommendByQuestion(ctx context.Context, req *video.RecommendByQuestionRequest) (*video.RecommendByQuestionResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}

	items, err := s.App.RecommendByQuestion(ctx, videoapp.RecommendByQuestionInput{
		QuestionID:   req.GetQuestionId(),
		QuestionText: req.GetQuestionText(),
		UserID:       req.GetUserId(),
		Limit:        int(req.GetLimit()),
	})
	if err != nil {
		return nil, toRPCError(err)
	}

	return &video.RecommendByQuestionResponse{
		Items: toProtoRecommendItems(items),
		Total: int32(len(items)),
	}, nil
}

// ListRecommendations 返回已保存的推荐记录列表。
func (s *VideoService) ListRecommendations(ctx context.Context, req *video.ListRecommendationsRequest) (*video.ListRecommendationsResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	if req.GetQuestionId() == 0 {
		return nil, status.Error(codes.InvalidArgument, "question_id is required")
	}

	items, err := s.App.ListRecommendations(ctx, videoapp.ListRecommendationsInput{
		QuestionID: req.GetQuestionId(),
		UserID:     req.GetUserId(),
		Limit:      int(req.GetLimit()),
	})
	if err != nil {
		return nil, toRPCError(err)
	}

	return &video.ListRecommendationsResponse{
		Items: toProtoRecommendItems(items),
		Total: int32(len(items)),
	}, nil
}

// toProtoRecommendItems 把推荐结果映射成 protobuf 响应对象。
func toProtoRecommendItems(items []videoapp.RecommendResultItem) []*video.RecommendItem {
	out := make([]*video.RecommendItem, 0, len(items))
	for _, item := range items {
		title := item.Video.Title
		if item.TitleOverride != "" {
			title = item.TitleOverride
		}
		out = append(out, &video.RecommendItem{
			QuestionId:     item.QuestionID,
			VideoId:        item.VideoID,
			VideoSegmentId: item.VideoSegmentID,
			RecommendScore: item.RecommendScore,
			IsWatched:      item.IsWatched,
			WatchDuration:  int32(item.WatchDuration),
			StartTimeSec:   int32(item.StartTimeSec),
			EndTimeSec:     int32(item.EndTimeSec),
			Video: &video.VideoInfo{
				Name:          title,
				RawUrl:        item.Video.VideoURL,
				HlsUrl:        "",
				IsHls:         false,
				DownloadUrl:   item.Video.VideoURL,
				TaskId:        fmt.Sprintf("%d", item.VideoID),
				Id:            item.VideoID,
				ViewCount:     int64(item.Video.ViewCount),
				IsPublished:   item.Video.IsPublished,
				IsRecommend:   item.Video.IsRecommend,
				CreatedAtUnix: toUnixSeconds(item.Video.CreateTime),
				UpdatedAtUnix: toUnixSeconds(item.Video.UpdateTime),
				CoverUrl:      item.Video.CoverURL,
			},
		})
	}
	return out
}

// toRPCError 把 application 层错误收敛到稳定的 gRPC 状态码。
func toRPCError(err error) error {
	if err == nil {
		return nil
	}
	if status.Code(err) != codes.Unknown {
		return err
	}
	switch err {
	case videoapp.ErrVideoSegmentNotFound:
		return status.Error(codes.NotFound, err.Error())
	default:
		return status.Error(codes.Internal, err.Error())
	}
}
