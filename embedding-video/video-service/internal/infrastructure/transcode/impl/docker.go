package impl

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"nlp-video-analysis/internal/config"
)

type FFmpegDockerImpl struct {
	DockerImage string
	HLS         config.FFmpegHLSConfig
	Fast        config.FFmpegFastConfig
	Audio       config.FFmpegAudioConfig
}

func NewFFmpegDockerImpl(dockerImage string, hls config.FFmpegHLSConfig, fast config.FFmpegFastConfig, audio config.FFmpegAudioConfig) *FFmpegDockerImpl {
	return &FFmpegDockerImpl{
		DockerImage: dockerImage,
		HLS:         hls,
		Fast:        fast,
		Audio:       audio,
	}
}

func (d *FFmpegDockerImpl) BuildDockerArgs(mode string, inputPath string, outputDir string) ([]string, error) {
	absInput, err := filepath.Abs(inputPath)
	if err != nil {
		return nil, err
	}
	absOutput, err := filepath.Abs(outputDir)
	if err != nil {
		return nil, err
	}
	inputDir := filepath.Dir(absInput)
	inputName := filepath.Base(absInput)

	inMount, err := filepath.Abs(inputDir)
	if err != nil {
		return nil, err
	}
	outMount, err := filepath.Abs(absOutput)
	if err != nil {
		return nil, err
	}

	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode == "" || mode == "default" {
		mode = "standard"
	}
	hlsTime := d.HLS.Time
	if hlsTime <= 0 {
		hlsTime = 6
	}
	listSize := d.HLS.ListSize
	if listSize < 0 {
		listSize = 0
	}
	masterName := strings.TrimSpace(d.HLS.MasterName)
	if masterName == "" {
		masterName = "master.m3u8"
	}
	segPattern := strings.TrimSpace(d.HLS.SegmentPattern)
	if segPattern == "" {
		segPattern = "v0_%03d.ts"
	}
	if mode == "copy" {
		return []string{
			"run", "--rm",
			"--entrypoint", "ffmpeg",
			"-v", inMount + ":/in:ro",
			"-v", outMount + ":/out",
			d.DockerImage,
			"-i", "/in/" + inputName,
			"-c", "copy",
			"-f", "hls",
			"-hls_time", strconv.Itoa(hlsTime),
			"-hls_list_size", strconv.Itoa(listSize),
			"-hls_segment_filename", "/out/" + segPattern,
			"/out/" + masterName,
		}, nil
	}
	if mode == "fast" {
		w := d.Fast.ScaleW
		h := d.Fast.ScaleH
		if w <= 0 {
			w = 1280
		}
		if h <= 0 {
			h = 720
		}
		preset := strings.TrimSpace(d.Fast.Preset)
		if preset == "" {
			preset = "ultrafast"
		}
		crf := d.Fast.Crf
		if crf <= 0 {
			crf = 28
		}
		pixFmt := strings.TrimSpace(d.Fast.PixFmt)
		if pixFmt == "" {
			pixFmt = "yuv420p"
		}
		ab := strings.TrimSpace(d.Fast.AudioBitrate)
		if ab == "" {
			ab = "96k"
		}
		ac := d.Fast.AudioChannels
		if ac <= 0 {
			ac = 2
		}
		vf := fmt.Sprintf("scale=w=%d:h=%d:force_original_aspect_ratio=decrease", w, h)
		if d.Fast.PadToFit {
			vf = vf + fmt.Sprintf(",pad=%d:%d:(ow-iw)/2:(oh-ih)/2", w, h)
		}
		return []string{
			"run", "--rm",
			"--entrypoint", "ffmpeg",
			"-v", inMount + ":/in:ro",
			"-v", outMount + ":/out",
			d.DockerImage,
			"-i", "/in/" + inputName,
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
			"-hls_segment_filename", "/out/" + segPattern,
			"/out/" + masterName,
		}, nil
	}

	return []string{
		"run", "--rm",
		"--entrypoint", "ffmpeg",
		"-v", inMount + ":/in:ro",
		"-v", outMount + ":/out",
		d.DockerImage,
		"-i", "/in/" + inputName,
		"-filter_complex", "[0:v]split=3[v1][v2][v3];[v1]scale=w=1920:h=1080[v1out];[v2]scale=w=1280:h=720[v2out];[v3]scale=w=854:h=480[v3out]",
		"-map", "[v1out]", "-map", "0:a",
		"-map", "[v2out]", "-map", "0:a",
		"-map", "[v3out]", "-map", "0:a",
		"-c:v:0", "libx264", "-b:v:0", "5000k",
		"-c:v:1", "libx264", "-b:v:1", "2800k",
		"-c:v:2", "libx264", "-b:v:2", "1400k",
		"-c:a", "aac", "-b:a", "128k",
		"-f", "hls",
		"-hls_time", strconv.Itoa(hlsTime),
		"-hls_list_size", strconv.Itoa(listSize),
		"-hls_segment_filename", "/out/v%v_%03d.ts",
		"-master_pl_name", masterName,
		"-var_stream_map", "v:0,a:0 v:1,a:1 v:2,a:2",
		"/out/v%v_index.m3u8",
	}, nil
}

func (d *FFmpegDockerImpl) BuildDockerCoverArgs(inputPath string, outputPath string, ssSeconds int, q int) ([]string, error) {
	absInput, err := filepath.Abs(inputPath)
	if err != nil {
		return nil, err
	}
	absOutput, err := filepath.Abs(outputPath)
	if err != nil {
		return nil, err
	}
	inputDir := filepath.Dir(absInput)
	inputName := filepath.Base(absInput)
	outDir := filepath.Dir(absOutput)
	outName := filepath.Base(absOutput)

	inMount, err := filepath.Abs(inputDir)
	if err != nil {
		return nil, err
	}
	outMount, err := filepath.Abs(outDir)
	if err != nil {
		return nil, err
	}

	return []string{
		"run", "--rm",
		"--entrypoint", "ffmpeg",
		"-v", inMount + ":/in:ro",
		"-v", outMount + ":/out",
		d.DockerImage,
		"-ss", fmt.Sprintf("%d", ssSeconds),
		"-i", "/in/" + inputName,
		"-vframes", "1",
		"-q:v", fmt.Sprintf("%d", q),
		"-y",
		"/out/" + outName,
	}, nil
}

func (d *FFmpegDockerImpl) BuildDockerExtractAudioArgs(inputPath string, outputPath string, startSec int, durationSec int, sampleRate int, channels int) ([]string, error) {
	absInput, err := filepath.Abs(inputPath)
	if err != nil {
		return nil, err
	}
	absOutput, err := filepath.Abs(outputPath)
	if err != nil {
		return nil, err
	}
	inputDir := filepath.Dir(absInput)
	inputName := filepath.Base(absInput)
	outDir := filepath.Dir(absOutput)
	outName := filepath.Base(absOutput)

	inMount, err := filepath.Abs(inputDir)
	if err != nil {
		return nil, err
	}
	outMount, err := filepath.Abs(outDir)
	if err != nil {
		return nil, err
	}
	return []string{
		"run", "--rm",
		"--entrypoint", "ffmpeg",
		"-v", inMount + ":/in:ro",
		"-v", outMount + ":/out",
		d.DockerImage,
		"-ss", fmt.Sprintf("%d", startSec),
		"-t", fmt.Sprintf("%d", durationSec),
		"-i", "/in/" + inputName,
		"-vn",
		"-ac", fmt.Sprintf("%d", channels),
		"-ar", fmt.Sprintf("%d", sampleRate),
		"-c:a", "pcm_s16le",
		"-y",
		"/out/" + outName,
	}, nil
}

func (d *FFmpegDockerImpl) BuildDockerClipVideoArgs(inputPath string, outputPath string, startSec int, durationSec int, tryCopy bool) ([]string, error) {
	absInput, err := filepath.Abs(inputPath)
	if err != nil {
		return nil, err
	}
	absOutput, err := filepath.Abs(outputPath)
	if err != nil {
		return nil, err
	}
	inputDir := filepath.Dir(absInput)
	inputName := filepath.Base(absInput)
	outDir := filepath.Dir(absOutput)
	outName := filepath.Base(absOutput)

	inMount, err := filepath.Abs(inputDir)
	if err != nil {
		return nil, err
	}
	outMount, err := filepath.Abs(outDir)
	if err != nil {
		return nil, err
	}

	if tryCopy {
		return []string{
			"run", "--rm",
			"--entrypoint", "ffmpeg",
			"-v", inMount + ":/in:ro",
			"-v", outMount + ":/out",
			d.DockerImage,
			"-ss", fmt.Sprintf("%d", startSec),
			"-t", fmt.Sprintf("%d", durationSec),
			"-i", "/in/" + inputName,
			"-map", "0",
			"-c", "copy",
			"-movflags", "+faststart",
			"-y",
			"/out/" + outName,
		}, nil
	}

	preset := strings.TrimSpace(d.Fast.Preset)
	if preset == "" {
		preset = "ultrafast"
	}
	crf := d.Fast.Crf
	if crf <= 0 {
		crf = 28
	}
	pixFmt := strings.TrimSpace(d.Fast.PixFmt)
	if pixFmt == "" {
		pixFmt = "yuv420p"
	}
	ab := strings.TrimSpace(d.Fast.AudioBitrate)
	if ab == "" {
		ab = "96k"
	}
	ac := d.Fast.AudioChannels
	if ac <= 0 {
		ac = 2
	}

	return []string{
		"run", "--rm",
		"--entrypoint", "ffmpeg",
		"-v", inMount + ":/in:ro",
		"-v", outMount + ":/out",
		d.DockerImage,
		"-ss", fmt.Sprintf("%d", startSec),
		"-t", fmt.Sprintf("%d", durationSec),
		"-i", "/in/" + inputName,
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
		"/out/" + outName,
	}, nil
}

func (d *FFmpegDockerImpl) BuildDockerClipVideoWithAudioArgs(inputPath string, outputVideoPath string, outputAudioPath string, startSec int, durationSec int, sampleRate int, channels int, tryCopy bool) ([]string, error) {
	absInput, err := filepath.Abs(inputPath)
	if err != nil {
		return nil, err
	}
	absVideo, err := filepath.Abs(outputVideoPath)
	if err != nil {
		return nil, err
	}
	absAudio, err := filepath.Abs(outputAudioPath)
	if err != nil {
		return nil, err
	}

	inputDir := filepath.Dir(absInput)
	inputName := filepath.Base(absInput)
	outDir := filepath.Dir(absVideo)
	if filepath.Dir(absAudio) != outDir {
		return nil, fmt.Errorf("outputVideoPath and outputAudioPath must be in the same directory when using docker")
	}
	outVideoName := filepath.Base(absVideo)
	outAudioName := filepath.Base(absAudio)

	inMount, err := filepath.Abs(inputDir)
	if err != nil {
		return nil, err
	}
	outMount, err := filepath.Abs(outDir)
	if err != nil {
		return nil, err
	}

	if sampleRate <= 0 {
		sampleRate = 16000
	}
	if channels <= 0 {
		channels = 1
	}

	if tryCopy {
		return []string{
			"run", "--rm",
			"--entrypoint", "ffmpeg",
			"-v", inMount + ":/in:ro",
			"-v", outMount + ":/out",
			d.DockerImage,
			"-ss", fmt.Sprintf("%d", startSec),
			"-t", fmt.Sprintf("%d", durationSec),
			"-i", "/in/" + inputName,
			"-map", "0",
			"-c", "copy",
			"-movflags", "+faststart",
			"-y",
			"/out/" + outVideoName,
			"-map", "0:a:0",
			"-vn",
			"-ac", fmt.Sprintf("%d", channels),
			"-ar", fmt.Sprintf("%d", sampleRate),
			"-c:a", "pcm_s16le",
			"-y",
			"/out/" + outAudioName,
		}, nil
	}

	preset := strings.TrimSpace(d.Fast.Preset)
	if preset == "" {
		preset = "ultrafast"
	}
	crf := d.Fast.Crf
	if crf <= 0 {
		crf = 28
	}
	pixFmt := strings.TrimSpace(d.Fast.PixFmt)
	if pixFmt == "" {
		pixFmt = "yuv420p"
	}
	ab := strings.TrimSpace(d.Fast.AudioBitrate)
	if ab == "" {
		ab = "96k"
	}
	ac := d.Fast.AudioChannels
	if ac <= 0 {
		ac = 2
	}

	return []string{
		"run", "--rm",
		"--entrypoint", "ffmpeg",
		"-v", inMount + ":/in:ro",
		"-v", outMount + ":/out",
		d.DockerImage,
		"-ss", fmt.Sprintf("%d", startSec),
		"-t", fmt.Sprintf("%d", durationSec),
		"-i", "/in/" + inputName,
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
		"/out/" + outVideoName,
		"-map", "0:a:0",
		"-vn",
		"-ac", fmt.Sprintf("%d", channels),
		"-ar", fmt.Sprintf("%d", sampleRate),
		"-c:a", "pcm_s16le",
		"-y",
		"/out/" + outAudioName,
	}, nil
}

func BuildDockerProbeDurationArgs(dockerImage string, inputPath string) ([]string, error) {
	absInput, err := filepath.Abs(inputPath)
	if err != nil {
		return nil, err
	}
	inputDir := filepath.Dir(absInput)
	inputName := filepath.Base(absInput)

	inMount, err := filepath.Abs(inputDir)
	if err != nil {
		return nil, err
	}
	return []string{
		"run", "--rm",
		"--entrypoint", "ffprobe",
		"-v", inMount + ":/in:ro",
		dockerImage,
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		"/in/" + inputName,
	}, nil
}
