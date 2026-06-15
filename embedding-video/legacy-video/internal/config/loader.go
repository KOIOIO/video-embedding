package config

import (
	"os"
	"path/filepath"
	"runtime"

	"gopkg.in/yaml.v3"
)

const (
	defaultConfigPath = "configs/video.yml"
	prodConfigPath    = "configs/video_prod.yml"
)

// MustLoad 模仿 go-zero：加载配置，失败直接 panic
func MustLoad(path string) Config {
	data, err := os.ReadFile(path)
	if err != nil {
		if !filepath.IsAbs(path) {
			if data2, err2 := tryReadUpward(path, 6); err2 == nil {
				data = data2
				err = nil
			}
		}
		if err != nil {
			panic("load config file failed: " + err.Error())
		}
	}

	var c Config
	if err := yaml.Unmarshal(data, &c); err != nil {
		panic("parse config yaml failed: " + err.Error())
	}

	return c
}

// MustLoadDefault 按运行环境加载默认配置。
// Windows 本地开发默认使用 video.yml，其他环境默认使用 video_prod.yml。
// 可通过 CONFIG_FILE 或 VIDEO_CONFIG_FILE 显式覆盖。
func MustLoadDefault() Config {
	return MustLoad(DefaultConfigPath())
}

// DefaultConfigPath 返回当前环境应使用的配置路径。
func DefaultConfigPath() string {
	if p := os.Getenv("CONFIG_FILE"); p != "" {
		return p
	}
	if p := os.Getenv("VIDEO_CONFIG_FILE"); p != "" {
		return p
	}
	if runtime.GOOS == "windows" {
		return defaultConfigPath
	}
	return prodConfigPath
}

// ResolvePath 解析路径
func ResolvePath(path string) (string, error) {
	if filepath.IsAbs(path) {
		return path, nil
	}
	return findUpward(path, 6)
}

// EnsureProjectRoot 确保项目根目录
func EnsureProjectRoot() {
	root, err := FindProjectRoot()
	if err != nil {
		return
	}
	_ = os.Chdir(root)
}

// FindProjectRoot 查找项目根目录
func FindProjectRoot() (string, error) {
	if p, err := ResolvePath(filepath.Join("legacy-video", DefaultConfigPath())); err == nil {
		return filepath.Dir(filepath.Dir(p)), nil
	}
	if p, err := ResolvePath(filepath.Join("legacy-video", defaultConfigPath)); err == nil {
		return filepath.Dir(filepath.Dir(p)), nil
	}
	if p, err := ResolvePath(filepath.Join("legacy-video", prodConfigPath)); err == nil {
		return filepath.Dir(filepath.Dir(p)), nil
	}
	if p, err := ResolvePath(DefaultConfigPath()); err == nil {
		return filepath.Dir(filepath.Dir(p)), nil
	}
	if p, err := ResolvePath(defaultConfigPath); err == nil {
		return filepath.Dir(filepath.Dir(p)), nil
	}
	if p, err := ResolvePath(prodConfigPath); err == nil {
		return filepath.Dir(filepath.Dir(p)), nil
	}
	if p, err := ResolvePath("go.mod"); err == nil {
		return filepath.Dir(p), nil
	}
	return "", os.ErrNotExist
}

// tryReadUpward 尝试向上读取文件
func tryReadUpward(relPath string, maxLevels int) ([]byte, error) {
	p, err := findUpward(relPath, maxLevels)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(p)
}

// findUpward 向上查找文件
func findUpward(relPath string, maxLevels int) (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	dir := cwd
	for i := 0; i <= maxLevels; i++ {
		p := filepath.Join(dir, relPath)
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", os.ErrNotExist
}
