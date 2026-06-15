package impl

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"legacy-video/internal/config"
)

// FFmpegHLSImpl 负责构造不同模式下的 HLS ffmpeg 参数。
type FFmpegHLSImpl struct {
	HLS  config.FFmpegHLSConfig
	Fast config.FFmpegFastConfig
}

// NewFFmpegHLSImpl 创建 HLS 参数构造器。
func NewFFmpegHLSImpl(hls config.FFmpegHLSConfig, fast config.FFmpegFastConfig) *FFmpegHLSImpl {
	return &FFmpegHLSImpl{
		HLS:  hls,
		Fast: fast,
	}
}

// BuildFFmpegArgs 根据 mode 生成不同的 HLS 转码参数。
func (h *FFmpegHLSImpl) BuildFFmpegArgs(mode string, inputPath string, outputDir string) []string {
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode == "" || mode == "default" {
		mode = "standard"
	}
	hlsTime := h.HLS.Time
	if hlsTime <= 0 {
		hlsTime = 6
	}
	listSize := h.HLS.ListSize
	if listSize < 0 {
		listSize = 0
	}
	masterName := strings.TrimSpace(h.HLS.MasterName)
	if masterName == "" {
		masterName = "master.m3u8"
	}
	segPattern := strings.TrimSpace(h.HLS.SegmentPattern)
	if segPattern == "" {
		segPattern = "v0_%03d.ts"
	}
	if mode == "copy" {
		return []string{
			"-i", inputPath,
			"-c", "copy",
			"-f", "hls",
			"-hls_time", strconv.Itoa(hlsTime),
			"-hls_list_size", strconv.Itoa(listSize),
			"-hls_segment_filename", filepath.Join(outputDir, segPattern),
			filepath.Join(outputDir, masterName),
		}
	}
	if mode == "fast" {
		w := h.Fast.ScaleW
		height := h.Fast.ScaleH
		if w <= 0 {
			w = 1280
		}
		if height <= 0 {
			height = 720
		}
		preset := strings.TrimSpace(h.Fast.Preset)
		if preset == "" {
			preset = "ultrafast"
		}
		crf := h.Fast.Crf
		if crf <= 0 {
			crf = 28
		}
		pixFmt := strings.TrimSpace(h.Fast.PixFmt)
		if pixFmt == "" {
			pixFmt = "yuv420p"
		}
		ab := strings.TrimSpace(h.Fast.AudioBitrate)
		if ab == "" {
			ab = "96k"
		}
		ac := h.Fast.AudioChannels
		if ac <= 0 {
			ac = 2
		}
		vf := fmt.Sprintf("scale=w=%d:h=%d:force_original_aspect_ratio=decrease", w, height)
		if h.Fast.PadToFit {
			vf = vf + fmt.Sprintf(",pad=%d:%d:(ow-iw)/2:(oh-ih)/2", w, height)
		}
		return []string{
			"-i", inputPath,
			"-vf", vf,
			"-c:v", "libx264",
			"-preset", preset,
			"-crf", strconv.Itoa(crf),
			"-pix_fmt", pixFmt,
			"-c:a", "aac",
			"-b:a", ab,
			"-ac", strconv.Itoa(ac),
			"-f", "hls",
			"-hls_time", strconv.Itoa(hlsTime),
			"-hls_list_size", strconv.Itoa(listSize),
			"-hls_segment_filename", filepath.Join(outputDir, segPattern),
			filepath.Join(outputDir, masterName),
		}
	}
	return []string{
		"-i", inputPath,
		"-filter_complex", "[0:v]split=3[v1][v2][v3];[v1]scale=w=1920:h=1080[v1out];[v2]scale=w=1280:h=720[v2out];[v3]scale=w=854:h=480[v3out]",
		"-map", "[v1out]", "-map", "0:a",
		"-map", "[v2out]", "-map", "0:a",
		"-map", "[v3out]", "-map", "0:a",
		"-c:v:0", "libx264", "-b:v:0", "5000k", "-maxrate:v:0", "5350k", "-bufsize:v:0", "7500k",
		"-c:v:1", "libx264", "-b:v:1", "2800k", "-maxrate:v:1", "2996k", "-bufsize:v:1", "4200k",
		"-c:v:2", "libx264", "-b:v:2", "1400k", "-maxrate:v:2", "1498k", "-bufsize:v:2", "2100k",
		"-c:a", "aac", "-b:a", "128k", "-ac", "2",
		"-f", "hls",
		"-hls_time", strconv.Itoa(hlsTime),
		"-hls_list_size", strconv.Itoa(listSize),
		"-hls_segment_filename", filepath.Join(outputDir, "v%v_%03d.ts"),
		"-master_pl_name", masterName,
		"-var_stream_map", "v:0,a:0 v:1,a:1 v:2,a:2",
		filepath.Join(outputDir, "v%v_index.m3u8"),
	}
}
