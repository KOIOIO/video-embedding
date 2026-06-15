package transcode

import (
	"context"
	"errors"
	"fmt"
	"nlp-video-analysis/internal/config"
	"nlp-video-analysis/internal/infrastructure/transcode/impl"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// FFmpegTranscoder 统一封装本机 ffmpeg 与 Docker ffmpeg 的调用细节。
type FFmpegTranscoder struct {
	UseDocker   bool
	DockerImage string
	Mode        string
	HLS         config.FFmpegHLSConfig
	Fast        config.FFmpegFastConfig
	Cover       config.FFmpegCoverConfig
	Audio       config.FFmpegAudioConfig
	hlsImpl     *impl.FFmpegHLSImpl
	coverImpl   *impl.FFmpegCoverImpl
	audioImpl   *impl.FFmpegAudioImpl
	dockerImpl  *impl.FFmpegDockerImpl
}

// NewFFmpegTranscoder 创建 FFmpeg 转码器，并装配各类参数构造器。
func NewFFmpegTranscoder(cfg config.FFmpegConfig, mode string) *FFmpegTranscoder {
	hlsImpl := impl.NewFFmpegHLSImpl(cfg.HLS, cfg.Fast)
	coverImpl := impl.NewFFmpegCoverImpl(cfg.Cover)
	audioImpl := impl.NewFFmpegAudioImpl(cfg.Audio)
	dockerImpl := impl.NewFFmpegDockerImpl(cfg.DockerImage, cfg.HLS, cfg.Fast, cfg.Audio)

	return &FFmpegTranscoder{
		UseDocker:   cfg.UseDocker,
		DockerImage: cfg.DockerImage,
		Mode:        strings.ToLower(strings.TrimSpace(mode)),
		HLS:         cfg.HLS,
		Fast:        cfg.Fast,
		Cover:       cfg.Cover,
		Audio:       cfg.Audio,
		hlsImpl:     hlsImpl,
		coverImpl:   coverImpl,
		audioImpl:   audioImpl,
		dockerImpl:  dockerImpl,
	}
}

// ConvertToHLS 把原始视频转成 HLS 目录。
// 优先使用本机 ffmpeg，未安装时按配置回退到 Docker。
func (t *FFmpegTranscoder) ConvertToHLS(ctx context.Context, inputPath string, outputDir string) error {
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return err
	}

	mode := t.Mode
	ffmpegArgs := t.hlsImpl.BuildFFmpegArgs(mode, inputPath, outputDir)
	if _, err := exec.LookPath("ffmpeg"); err == nil {
		cmd := exec.CommandContext(ctx, "ffmpeg", ffmpegArgs...)
		out, err := cmd.CombinedOutput()
		if err == nil {
			return nil
		}
		if mode == "copy" {
			cmd = exec.CommandContext(ctx, "ffmpeg", t.hlsImpl.BuildFFmpegArgs("fast", inputPath, outputDir)...)
			out2, err2 := cmd.CombinedOutput()
			if err2 == nil {
				return nil
			}
			return fmt.Errorf("ffmpeg failed: %w: %s; fallback: %w: %s", err, string(out), err2, string(out2))
		}
		return fmt.Errorf("ffmpeg failed: %w: %s", err, string(out))
	}

	if !t.UseDocker {
		return errors.New("ffmpeg not found and docker is disabled")
	}

	dockerArgs, err := t.dockerImpl.BuildDockerArgs(mode, inputPath, outputDir)
	if err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, "docker", dockerArgs...)
	out, err := cmd.CombinedOutput()
	if err == nil {
		return nil
	}
	if mode == "copy" {
		dockerArgs2, err2 := t.dockerImpl.BuildDockerArgs("fast", inputPath, outputDir)
		if err2 != nil {
			return err2
		}
		cmd = exec.CommandContext(ctx, "docker", dockerArgs2...)
		out2, err2 := cmd.CombinedOutput()
		if err2 == nil {
			return nil
		}
		return fmt.Errorf("docker ffmpeg failed: %w: %s args=%v; fallback: %w: %s args=%v", err, string(out), dockerArgs, err2, string(out2), dockerArgs2)
	}
	return fmt.Errorf("docker ffmpeg failed: %w: %s args=%v", err, string(out), dockerArgs)
}

// GenerateCover 从视频中截取一帧生成封面图，并在首选时间点失败时尝试 fallback 时间点。
func (t *FFmpegTranscoder) GenerateCover(ctx context.Context, inputPath string, outputPath string) error {
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return err
	}

	ss := t.Cover.SeekSec
	if ss <= 0 {
		ss = 2
	}
	fallbackSS := t.Cover.FallbackSeekSec
	q := t.Cover.Quality
	if q <= 0 {
		q = 2
	}
	ffmpegArgs := t.coverImpl.BuildFFmpegCoverArgs(inputPath, outputPath, ss, q)
	if _, err := exec.LookPath("ffmpeg"); err == nil {
		cmd := exec.CommandContext(ctx, "ffmpeg", ffmpegArgs...)
		out, err := cmd.CombinedOutput()
		if err == nil {
			return nil
		}
		cmd = exec.CommandContext(ctx, "ffmpeg", t.coverImpl.BuildFFmpegCoverArgs(inputPath, outputPath, fallbackSS, q)...)
		out2, err2 := cmd.CombinedOutput()
		if err2 != nil {
			return fmt.Errorf("ffmpeg cover failed: %w: %s; fallback: %w: %s", err, string(out), err2, string(out2))
		}
		return nil
	}

	if !t.UseDocker {
		return errors.New("ffmpeg not found and docker is disabled")
	}

	dockerArgs, err := t.dockerImpl.BuildDockerCoverArgs(inputPath, outputPath, ss, q)
	if err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, "docker", dockerArgs...)
	out, err := cmd.CombinedOutput()
	if err == nil {
		return nil
	}
	dockerArgs2, err2 := t.dockerImpl.BuildDockerCoverArgs(inputPath, outputPath, fallbackSS, q)
	if err2 != nil {
		return err2
	}
	cmd = exec.CommandContext(ctx, "docker", dockerArgs2...)
	out2, err2 := cmd.CombinedOutput()
	if err2 != nil {
		return fmt.Errorf("docker ffmpeg cover failed: %w: %s args=%v; fallback: %w: %s args=%v", err, string(out), dockerArgs, err2, string(out2), dockerArgs2)
	}
	return nil
}

// ExtractAudioSegment 从视频中抽取指定时间片段的音频。
func (t *FFmpegTranscoder) ExtractAudioSegment(ctx context.Context, inputPath string, outputPath string, startSec int, durationSec int) error {
	if strings.TrimSpace(inputPath) == "" {
		return errors.New("inputPath is required")
	}
	if strings.TrimSpace(outputPath) == "" {
		return errors.New("outputPath is required")
	}
	if startSec < 0 {
		startSec = 0
	}
	if durationSec <= 0 {
		return errors.New("durationSec must be > 0")
	}
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return err
	}

	sr := t.Audio.SampleRate
	if sr <= 0 {
		sr = 16000
	}
	ch := t.Audio.Channels
	if ch <= 0 {
		ch = 1
	}
	ffmpegArgs := t.audioImpl.BuildFFmpegExtractAudioArgs(inputPath, outputPath, startSec, durationSec, sr, ch)
	if _, err := exec.LookPath("ffmpeg"); err == nil {
		cmd := exec.CommandContext(ctx, "ffmpeg", ffmpegArgs...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("ffmpeg extract audio failed: %w: %s", err, string(out))
		}
		return nil
	}

	if !t.UseDocker {
		return errors.New("ffmpeg not found and docker is disabled")
	}
	dockerArgs, err := t.dockerImpl.BuildDockerExtractAudioArgs(inputPath, outputPath, startSec, durationSec, sr, ch)
	if err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, "docker", dockerArgs...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker ffmpeg extract audio failed: %w: %s args=%v", err, string(out), dockerArgs)
	}
	return nil
}

// ClipVideoSegment 裁剪一个视频片段，优先尝试 copy 模式，失败时回退到快速重编码。
func (t *FFmpegTranscoder) ClipVideoSegment(ctx context.Context, inputPath string, outputPath string, startSec int, durationSec int) error {
	if strings.TrimSpace(inputPath) == "" {
		return errors.New("inputPath is required")
	}
	if strings.TrimSpace(outputPath) == "" {
		return errors.New("outputPath is required")
	}
	if startSec < 0 {
		startSec = 0
	}
	if durationSec <= 0 {
		return errors.New("durationSec must be > 0")
	}
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return err
	}

	copyArgs := []string{
		"-ss", fmt.Sprintf("%d", startSec),
		"-t", fmt.Sprintf("%d", durationSec),
		"-i", inputPath,
		"-map", "0",
		"-c", "copy",
		"-movflags", "+faststart",
		"-y",
		outputPath,
	}

	preset := strings.TrimSpace(t.Fast.Preset)
	if preset == "" {
		preset = "ultrafast"
	}
	crf := t.Fast.Crf
	if crf <= 0 {
		crf = 28
	}
	pixFmt := strings.TrimSpace(t.Fast.PixFmt)
	if pixFmt == "" {
		pixFmt = "yuv420p"
	}
	ab := strings.TrimSpace(t.Fast.AudioBitrate)
	if ab == "" {
		ab = "96k"
	}
	ac := t.Fast.AudioChannels
	if ac <= 0 {
		ac = 2
	}

	fastArgs := []string{
		"-ss", fmt.Sprintf("%d", startSec),
		"-t", fmt.Sprintf("%d", durationSec),
		"-i", inputPath,
		"-map", "0",
		"-c:v", "libx264",
		"-preset", preset,
		"-crf", strconv.Itoa(crf),
		"-pix_fmt", pixFmt,
		"-c:a", "aac",
		"-b:a", ab,
		"-ac", strconv.Itoa(ac),
		"-movflags", "+faststart",
		"-y",
		outputPath,
	}

	if _, err := exec.LookPath("ffmpeg"); err == nil {
		cmd := exec.CommandContext(ctx, "ffmpeg", copyArgs...)
		out, err := cmd.CombinedOutput()
		if err == nil {
			return nil
		}
		cmd = exec.CommandContext(ctx, "ffmpeg", fastArgs...)
		out2, err2 := cmd.CombinedOutput()
		if err2 == nil {
			return nil
		}
		return fmt.Errorf("ffmpeg clip failed: %w: %s; fallback: %w: %s", err, string(out), err2, string(out2))
	}

	if !t.UseDocker {
		return errors.New("ffmpeg not found and docker is disabled")
	}

	dockerArgs, err := t.dockerImpl.BuildDockerClipVideoArgs(inputPath, outputPath, startSec, durationSec, true)
	if err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, "docker", dockerArgs...)
	out, err := cmd.CombinedOutput()
	if err == nil {
		return nil
	}

	dockerArgs2, err2 := t.dockerImpl.BuildDockerClipVideoArgs(inputPath, outputPath, startSec, durationSec, false)
	if err2 != nil {
		return err2
	}
	cmd = exec.CommandContext(ctx, "docker", dockerArgs2...)
	out2, err2 := cmd.CombinedOutput()
	if err2 == nil {
		return nil
	}
	return fmt.Errorf("docker ffmpeg clip failed: %w: %s args=%v; fallback: %w: %s args=%v", err, string(out), dockerArgs, err2, string(out2), dockerArgs2)
}

// ClipVideoSegmentWithAudio 同时导出视频片段和对应音频片段。
func (t *FFmpegTranscoder) ClipVideoSegmentWithAudio(ctx context.Context, inputPath string, outputVideoPath string, outputAudioPath string, startSec int, durationSec int) error {
	if strings.TrimSpace(inputPath) == "" {
		return errors.New("inputPath is required")
	}
	if strings.TrimSpace(outputVideoPath) == "" {
		return errors.New("outputVideoPath is required")
	}
	if strings.TrimSpace(outputAudioPath) == "" {
		return errors.New("outputAudioPath is required")
	}
	if startSec < 0 {
		startSec = 0
	}
	if durationSec <= 0 {
		return errors.New("durationSec must be > 0")
	}
	if err := os.MkdirAll(filepath.Dir(outputVideoPath), 0755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(outputAudioPath), 0755); err != nil {
		return err
	}

	sr := t.Audio.SampleRate
	if sr <= 0 {
		sr = 16000
	}
	ch := t.Audio.Channels
	if ch <= 0 {
		ch = 1
	}

	copyArgs := []string{
		"-ss", fmt.Sprintf("%d", startSec),
		"-t", fmt.Sprintf("%d", durationSec),
		"-i", inputPath,
		"-map", "0",
		"-c", "copy",
		"-movflags", "+faststart",
		"-y",
		outputVideoPath,
		"-map", "0:a:0",
		"-vn",
		"-ac", strconv.Itoa(ch),
		"-ar", strconv.Itoa(sr),
		"-c:a", "pcm_s16le",
		"-y",
		outputAudioPath,
	}

	preset := strings.TrimSpace(t.Fast.Preset)
	if preset == "" {
		preset = "ultrafast"
	}
	crf := t.Fast.Crf
	if crf <= 0 {
		crf = 28
	}
	pixFmt := strings.TrimSpace(t.Fast.PixFmt)
	if pixFmt == "" {
		pixFmt = "yuv420p"
	}
	ab := strings.TrimSpace(t.Fast.AudioBitrate)
	if ab == "" {
		ab = "96k"
	}
	ac := t.Fast.AudioChannels
	if ac <= 0 {
		ac = 2
	}

	fastArgs := []string{
		"-ss", fmt.Sprintf("%d", startSec),
		"-t", fmt.Sprintf("%d", durationSec),
		"-i", inputPath,
		"-map", "0",
		"-c:v", "libx264",
		"-preset", preset,
		"-crf", strconv.Itoa(crf),
		"-pix_fmt", pixFmt,
		"-c:a", "aac",
		"-b:a", ab,
		"-ac", strconv.Itoa(ac),
		"-movflags", "+faststart",
		"-y",
		outputVideoPath,
		"-map", "0:a:0",
		"-vn",
		"-ac", strconv.Itoa(ch),
		"-ar", strconv.Itoa(sr),
		"-c:a", "pcm_s16le",
		"-y",
		outputAudioPath,
	}

	if _, err := exec.LookPath("ffmpeg"); err == nil {
		cmd := exec.CommandContext(ctx, "ffmpeg", copyArgs...)
		out, err := cmd.CombinedOutput()
		if err == nil {
			return nil
		}
		cmd = exec.CommandContext(ctx, "ffmpeg", fastArgs...)
		out2, err2 := cmd.CombinedOutput()
		if err2 == nil {
			return nil
		}
		return fmt.Errorf("ffmpeg clip+audio failed: %w: %s; fallback: %w: %s", err, string(out), err2, string(out2))
	}

	if !t.UseDocker {
		return errors.New("ffmpeg not found and docker is disabled")
	}

	dockerArgs, err := t.dockerImpl.BuildDockerClipVideoWithAudioArgs(inputPath, outputVideoPath, outputAudioPath, startSec, durationSec, sr, ch, true)
	if err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, "docker", dockerArgs...)
	out, err := cmd.CombinedOutput()
	if err == nil {
		return nil
	}

	dockerArgs2, err2 := t.dockerImpl.BuildDockerClipVideoWithAudioArgs(inputPath, outputVideoPath, outputAudioPath, startSec, durationSec, sr, ch, false)
	if err2 != nil {
		return err2
	}
	cmd = exec.CommandContext(ctx, "docker", dockerArgs2...)
	out2, err2 := cmd.CombinedOutput()
	if err2 == nil {
		return nil
	}
	return fmt.Errorf("docker ffmpeg clip+audio failed: %w: %s args=%v; fallback: %w: %s args=%v", err, string(out), dockerArgs, err2, string(out2), dockerArgs2)
}

// ProbeDurationSeconds 使用 ffprobe 探测视频总时长，并向上取整到秒。
func (t *FFmpegTranscoder) ProbeDurationSeconds(ctx context.Context, inputPath string) (int, error) {
	if strings.TrimSpace(inputPath) == "" {
		return 0, errors.New("inputPath is required")
	}
	if _, err := exec.LookPath("ffprobe"); err == nil {
		cmd := exec.CommandContext(ctx, "ffprobe",
			"-v", "error",
			"-show_entries", "format=duration",
			"-of", "default=noprint_wrappers=1:nokey=1",
			inputPath,
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return 0, fmt.Errorf("ffprobe failed: %w: %s", err, string(out))
		}
		s := strings.TrimSpace(string(out))
		if s == "" {
			return 0, nil
		}
		v, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return 0, err
		}
		if v <= 0 {
			return 0, nil
		}
		return int(math.Ceil(v)), nil
	}

	if !t.UseDocker {
		return 0, errors.New("ffprobe not found and docker is disabled")
	}
	dockerArgs, err := impl.BuildDockerProbeDurationArgs(t.DockerImage, inputPath)
	if err != nil {
		return 0, err
	}
	cmd := exec.CommandContext(ctx, "docker", dockerArgs...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return 0, fmt.Errorf("docker ffprobe failed: %w: %s args=%v", err, string(out), dockerArgs)
	}
	s := strings.TrimSpace(string(out))
	if s == "" {
		return 0, nil
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, err
	}
	if v <= 0 {
		return 0, nil
	}
	return int(math.Ceil(v)), nil
}


