package tasks

import "context"

type StageRecord struct {
	TaskID       string
	VideoID      uint64
	Stage        string
	SegmentIndex int
	SegmentID    uint64
	ObjectKey    string
	Text         string
	RetryCount   int
	StartSec     int
	EndSec       int
}

type StageRecorder interface {
	PendingTask(context.Context, StageRecord)
	CompleteTask(context.Context, StageRecord)
	FailTask(context.Context, StageRecord, error)
}

func recordStagePending(ctx context.Context, recorder StageRecorder, rec StageRecord) {
	if recorder == nil {
		return
	}
	recorder.PendingTask(ctx, rec)
}

func recordStageComplete(ctx context.Context, recorder StageRecorder, rec StageRecord) {
	if recorder == nil {
		return
	}
	recorder.CompleteTask(ctx, rec)
}

func recordStageFailed(ctx context.Context, recorder StageRecorder, rec StageRecord, err error) {
	if recorder == nil || err == nil {
		return
	}
	recorder.FailTask(ctx, rec, err)
}
