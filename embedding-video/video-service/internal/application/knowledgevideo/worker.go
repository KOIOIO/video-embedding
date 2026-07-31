package knowledgevideo

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const maxTranscodeAttempts = 5

type WorkerRepository interface {
	GetVideo(ctx context.Context, id uint64) (Video, bool, error)
	MarkTranscoding(ctx context.Context, id uint64) (bool, error)
	MarkReady(ctx context.Context, id uint64, masterObjectKey string, duration int) (bool, error)
	MarkFailed(ctx context.Context, id uint64, errorMessage string) (bool, error)
	RefreshBatchStatus(ctx context.Context, batchID uint64) (Batch, bool, error)
}

type WorkerQueue interface {
	Dequeue(ctx context.Context) (QueueMessage, error)
	Ack(ctx context.Context, messageID string) error
	Requeue(ctx context.Context, message QueueMessage, delay time.Duration, reason string) error
	MoveToDeadLetter(ctx context.Context, message QueueMessage, reason string) error
}

type Worker struct {
	Repository     WorkerRepository
	Queue          WorkerQueue
	Downloader     ObjectDownloader
	Transcoder     FFmpegConverter
	Uploader       DirectoryUploader
	TempRoot       string
	MasterPlaylist string
	TaskTimeout    time.Duration
}

func (w Worker) RunOnce(ctx context.Context) error {
	message, err := w.Queue.Dequeue(ctx)
	if err != nil {
		return err
	}
	if w.TaskTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, w.TaskTimeout)
		defer cancel()
	}
	video, found, err := w.Repository.GetVideo(ctx, message.Task.KnowledgeVideoID)
	if err != nil {
		return err
	}
	if !found {
		return w.Queue.Ack(ctx, message.MessageID)
	}
	if video.Status == VideoReady {
		if _, _, err := w.Repository.RefreshBatchStatus(ctx, video.BatchID); err != nil {
			return err
		}
		return w.Queue.Ack(ctx, message.MessageID)
	}
	if video.Status == VideoFailed {
		if _, _, err := w.Repository.RefreshBatchStatus(ctx, video.BatchID); err != nil {
			return err
		}
		return w.Queue.MoveToDeadLetter(ctx, message, video.ErrorMessage)
	}
	if video.Status == VideoPending {
		updated, err := w.Repository.MarkTranscoding(ctx, video.ID)
		if err != nil {
			return err
		}
		if !updated {
			return w.Queue.Ack(ctx, message.MessageID)
		}
	}
	if err := w.process(ctx, message, video); err != nil {
		return w.handleFailure(ctx, message, video, err)
	}
	return nil
}

func (w Worker) process(ctx context.Context, message QueueMessage, video Video) error {
	if err := os.MkdirAll(w.TempRoot, 0o750); err != nil {
		return err
	}
	taskDir, err := os.MkdirTemp(w.TempRoot, message.Task.TaskID+"-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(taskDir)
	rawDir := filepath.Join(taskDir, "raw")
	hlsDir := filepath.Join(taskDir, "hls")
	if err := os.MkdirAll(rawDir, 0o750); err != nil {
		return err
	}
	inputPath := filepath.Join(rawDir, message.Task.TaskID+filepath.Ext(video.SourceFileName))
	if err := w.Downloader.DownloadToFile(ctx, message.Task.SourceObjectKey, inputPath); err != nil {
		return fmt.Errorf("download: %w", err)
	}
	if err := w.Transcoder.ConvertToHLS(ctx, inputPath, hlsDir); err != nil {
		return fmt.Errorf("transcode: %w", err)
	}
	masterPlaylist := strings.TrimSpace(w.MasterPlaylist)
	if masterPlaylist == "" {
		masterPlaylist = "master.m3u8"
	}
	masterPath := filepath.Join(hlsDir, masterPlaylist)
	if stat, err := os.Stat(masterPath); err != nil || !stat.Mode().IsRegular() {
		if err == nil {
			err = errors.New("master playlist is not a regular file")
		}
		return fmt.Errorf("verify master playlist: %w", err)
	}
	duration, err := w.Transcoder.ProbeDurationSeconds(ctx, inputPath)
	if err != nil {
		return fmt.Errorf("probe duration: %w", err)
	}
	if err := w.Uploader.UploadDir(ctx, hlsDir, message.Task.HLSObjectPrefix); err != nil {
		return fmt.Errorf("upload HLS: %w", err)
	}
	masterKey := strings.TrimRight(message.Task.HLSObjectPrefix, "/") + "/" + masterPlaylist
	updated, err := w.Repository.MarkReady(ctx, video.ID, masterKey, duration)
	if err != nil {
		return fmt.Errorf("mark ready: %w", err)
	}
	if !updated {
		return errors.New("knowledge video disappeared before ready update")
	}
	if _, _, err := w.Repository.RefreshBatchStatus(ctx, video.BatchID); err != nil {
		return fmt.Errorf("refresh batch: %w", err)
	}
	return w.Queue.Ack(ctx, message.MessageID)
}

func (w Worker) handleFailure(ctx context.Context, message QueueMessage, video Video, taskErr error) error {
	if message.Task.RetryCount+1 < maxTranscodeAttempts {
		retry := message
		retry.Task.RetryCount++
		delay := time.Duration(retry.Task.RetryCount) * 5 * time.Second
		if delay > time.Minute {
			delay = time.Minute
		}
		if err := w.Queue.Requeue(ctx, retry, delay, taskErr.Error()); err != nil {
			return errors.Join(taskErr, err)
		}
		return taskErr
	}
	messageText := terminalErrorMessage(taskErr)
	if _, err := w.Repository.MarkFailed(ctx, video.ID, messageText); err != nil {
		return errors.Join(taskErr, err)
	}
	if _, _, err := w.Repository.RefreshBatchStatus(ctx, video.BatchID); err != nil {
		return errors.Join(taskErr, err)
	}
	if err := w.Queue.MoveToDeadLetter(ctx, message, taskErr.Error()); err != nil {
		return errors.Join(taskErr, err)
	}
	return taskErr
}

func terminalErrorMessage(err error) string {
	message := err.Error()
	if index := strings.Index(message, ": "); index >= 0 && index+2 < len(message) {
		return message[index+2:]
	}
	return message
}
