package service

import (
	"context"
	"io"
	"strings"

	"legacy-video/internal/application/videoapp"
	domainvideo "legacy-video/internal/domain/video"
	"legacy-video/video"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (s *VideoService) UploadVideo(stream video.VideoService_UploadVideoServer) error {
	var meta *video.VideoMeta
	var planCreated bool
	var plan videoapp.UploadPlan

	ctx := stream.Context()
	var writer io.WriteCloser
	var uploadErrCh chan error
	cleanupRaw := func() {
		if plan.RawAbsPath != "" {
			_ = s.App.FS.Remove(plan.RawAbsPath)
		}
	}
	waitUpload := func() error {
		if uploadErrCh == nil {
			return nil
		}
		return <-uploadErrCh
	}

	for {
		req, err := stream.Recv()
		if err == io.EOF {
			if writer != nil {
				_ = writer.Close()
			}
			if !planCreated {
				return status.Error(codes.InvalidArgument, "missing meta")
			}
			if err := waitUpload(); err != nil {
				cleanupRaw()
				return err
			}

			result, err := s.App.FinalizeUpload(ctx, plan, videoapp.UploadMeta{
				Title:       meta.GetTitle(),
				Description: meta.GetDescription(),
			})
			if err != nil {
				cleanupRaw()
				return err
			}

			return stream.SendAndClose(&video.UploadVideoResponse{
				Success:  true,
				Message:  "视频上传成功，正在转码为HLS格式",
				FileName: plan.StoredFileName,
				RawUrl:   result.RawURL,
				HlsUrl:   result.HLSURL,
				TaskId:   result.TaskID,
				VideoId:  result.VideoID,
			})
		}

		if err != nil {
			if writer != nil {
				_ = writer.Close()
			}
			_ = waitUpload()
			cleanupRaw()
			return err
		}

		if req.Data != nil {
			switch data := req.Data.(type) {
			case *video.UploadVideoRequest_Meta:
				if planCreated {
					return status.Error(codes.InvalidArgument, "meta already received")
				}
				meta = data.Meta
				if meta == nil || strings.TrimSpace(meta.FileName) == "" {
					return status.Error(codes.InvalidArgument, "invalid meta")
				}

				p, err := s.App.BuildUploadPlan(meta.FileName)
				if err != nil {
					return err
				}
				pr, pw := io.Pipe()
				writer = pw
				uploadErrCh = make(chan error, 1)
				go func() {
					err := s.App.Store.Put(ctx, p.RawObjectKey, pr, -1, meta.GetContentType())
					_ = pr.CloseWithError(err)
					uploadErrCh <- err
				}()
				p.RawUploaded = true
				p.RawAbsPath = ""
				planCreated = true
				plan = p

			case *video.UploadVideoRequest_ChunkData:
				if !planCreated || writer == nil {
					return status.Error(codes.InvalidArgument, "meta must be sent before chunk_data")
				}
				if _, err := writer.Write(data.ChunkData); err != nil {
					_ = writer.Close()
					_ = waitUpload()
					cleanupRaw()
					return err
				}
			}
		}
	}
}

func (s *VideoService) GetTranscodeStatus(ctx context.Context, req *video.GetTranscodeStatusRequest) (*video.GetTranscodeStatusResponse, error) {
	if req == nil || strings.TrimSpace(req.TaskId) == "" {
		return nil, status.Error(codes.InvalidArgument, "task_id is required")
	}

	info, ok, err := s.App.GetTranscodeStatus(ctx, req.TaskId)
	if err != nil {
		return nil, err
	}
	if !ok {
		return &video.GetTranscodeStatusResponse{
			TaskId:  req.TaskId,
			Status:  video.TranscodeStatus_PENDING,
			Message: "任务不存在或已过期",
			HlsUrl:  "",
		}, nil
	}

	response := &video.GetTranscodeStatusResponse{
		TaskId: req.TaskId,
		Status: toProtoTranscodeStatus(info.Status),
	}

	switch info.Status {
	case domainvideo.StatusUploaded:
		response.Message = "转码任务等待中"
	case domainvideo.StatusProcessing:
		response.Message = "转码任务处理中"
	case domainvideo.StatusDone:
		response.Message = "转码任务成功"
		response.HlsUrl = info.HLSURL
	case domainvideo.StatusFailed:
		response.Message = "转码任务失败"
	default:
		response.Message = "转码任务等待中"
	}

	return response, nil
}

// toProtoTranscodeStatus 把领域状态枚举映射为 protobuf 状态枚举。
func toProtoTranscodeStatus(s domainvideo.Status) video.TranscodeStatus {
	switch s {
	case domainvideo.StatusProcessing:
		return video.TranscodeStatus_PROCESSING
	case domainvideo.StatusDone:
		return video.TranscodeStatus_SUCCESS
	case domainvideo.StatusFailed:
		return video.TranscodeStatus_FAILED
	default:
		return video.TranscodeStatus_PENDING
	}
}
