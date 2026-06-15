package impl

import (
	"fmt"

	"nlp-video-analysis/internal/config"
)

// FFmpegCoverImpl 负责构造封面截图参数。
type FFmpegCoverImpl struct {
	Cover config.FFmpegCoverConfig
}

// NewFFmpegCoverImpl 创建封面截图参数构造器。
func NewFFmpegCoverImpl(cover config.FFmpegCoverConfig) *FFmpegCoverImpl {
	return &FFmpegCoverImpl{
		Cover: cover,
	}
}

// BuildFFmpegCoverArgs 生成单帧截图所需的 ffmpeg 参数。
func (c *FFmpegCoverImpl) BuildFFmpegCoverArgs(inputPath string, outputPath string, ssSeconds int, q int) []string {
	return []string{
		"-ss", fmt.Sprintf("%d", ssSeconds),
		"-i", inputPath,
		"-vframes", "1",
		"-q:v", fmt.Sprintf("%d", q),
		"-y",
		outputPath,
	}
}
