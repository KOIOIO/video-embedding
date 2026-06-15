package service

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"legacy-video/internal/application/videoapp"
	"legacy-video/video"
)

// ReportWatch 记录用户对推荐片段的观看行为。
func (s *VideoService) ReportWatch(ctx context.Context, req *video.ReportWatchRequest) (*video.ReportWatchResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	if req.GetQuestionId() == 0 {
		return nil, status.Error(codes.InvalidArgument, "question_id is required")
	}
	if req.GetVideoSegmentId() == 0 {
		return nil, status.Error(codes.InvalidArgument, "video_segment_id is required")
	}

	err := s.App.ReportWatch(ctx, videoapp.ReportWatchInput{
		QuestionID:     req.GetQuestionId(),
		UserID:         req.GetUserId(),
		VideoSegmentID: req.GetVideoSegmentId(),
		IsWatched:      req.GetIsWatched(),
		WatchDuration:  int(req.GetWatchDuration()),
	})
	if err != nil {
		return nil, toRPCError(err)
	}

	return &video.ReportWatchResponse{
		Success: true,
		Message: "ok",
	}, nil
}
