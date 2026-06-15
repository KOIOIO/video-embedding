package impl

import (
	"fmt"

	"nlp-video-analysis/internal/config"
)

// FFmpegAudioImpl 负责构造音频抽取参数。
type FFmpegAudioImpl struct {
	Audio config.FFmpegAudioConfig
}

// NewFFmpegAudioImpl 创建音频抽取参数构造器。
func NewFFmpegAudioImpl(audio config.FFmpegAudioConfig) *FFmpegAudioImpl {
	return &FFmpegAudioImpl{
		Audio: audio,
	}
}

// BuildFFmpegExtractAudioArgs 生成抽取音频片段所需的 ffmpeg 参数。
func (a *FFmpegAudioImpl) BuildFFmpegExtractAudioArgs(inputPath string, outputPath string, startSec int, durationSec int, sampleRate int, channels int) []string {
	return []string{
		"-ss", fmt.Sprintf("%d", startSec),
		"-t", fmt.Sprintf("%d", durationSec),
		"-i", inputPath,
		"-vn",
		"-ac", fmt.Sprintf("%d", channels),
		"-ar", fmt.Sprintf("%d", sampleRate),
		"-c:a", "pcm_s16le",
		"-y",
		outputPath,
	}
}
