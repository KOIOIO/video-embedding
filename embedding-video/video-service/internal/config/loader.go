package config

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	defaultConfigPath = "configs/video.yml"
	prodConfigPath    = "configs/video_prod.yml"
)

var runtimeGOOS = runtime.GOOS

// MustLoad 模仿 go-zero：加载配置，失败直接 panic
func MustLoad(path string) Config {
	loadDotEnv()

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
	applyEnvOverrides(&c)

	return c
}

func applyEnvOverrides(c *Config) {
	if value := firstEnv("POSTGRES_DSN"); value != "" {
		c.Postgres.DSN = value
	}
	if value := firstEnv("REDIS_PASSWORD"); value != "" {
		c.Redis.Password = value
	}
	if value := firstEnv("COS_SECRET_ID", "RUSTFS_ACCESS_KEY"); value != "" {
		c.RustFS.AccessKey = value
	}
	if value := firstEnv("COS_SECRET_KEY", "RUSTFS_SECRET_KEY"); value != "" {
		c.RustFS.SecretKey = value
	}
	if value := firstEnv("GORSE_API_KEY"); value != "" {
		c.Gorse.APIKey = value
	}
	if value := firstEnv("GORSE_DASHBOARD_USERNAME"); value != "" {
		c.Gorse.DashboardUsername = value
	}
	if value := firstEnv("GORSE_DASHBOARD_PASSWORD"); value != "" {
		c.Gorse.DashboardPassword = value
	}
	if value := firstEnv("ASR_API_KEY", "DASHSCOPE_API_KEY", "OPENAI_API_KEY"); value != "" {
		c.ASR.APIKey = value
	}
	if value := firstEnv("EMBEDDING_API_KEY", "DASHSCOPE_API_KEY", "OPENAI_API_KEY"); value != "" {
		c.Embedding.APIKey = value
	}
}

func loadDotEnv() {
	if path, err := findUpward(".env", 6); err == nil {
		loadEnvFile(path)
	}
	envFile := firstEnv("VIDEO_ENV_FILE")
	if envFile == "" {
		return
	}
	if !filepath.IsAbs(envFile) {
		if path, err := findUpward(envFile, 6); err == nil {
			envFile = path
		}
	}
	loadEnvFile(envFile)
}

func loadEnvFile(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(data), "\n") {
		key, value, ok := parseEnvLine(line)
		if !ok {
			continue
		}
		if _, exists := os.LookupEnv(key); exists {
			continue
		}
		_ = os.Setenv(key, value)
	}
}

func parseEnvLine(line string) (string, string, bool) {
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, "#") {
		return "", "", false
	}
	line = strings.TrimPrefix(line, "export ")
	key, value, ok := strings.Cut(line, "=")
	if !ok {
		return "", "", false
	}
	key = strings.TrimSpace(key)
	if key == "" || strings.ContainsAny(key, " \t") {
		return "", "", false
	}
	value = strings.TrimSpace(value)
	if len(value) >= 2 {
		quote := value[0]
		if (quote == '"' || quote == '\'') && value[len(value)-1] == quote {
			value = value[1 : len(value)-1]
		}
	}
	return key, value, true
}

// MustLoadDefault 按运行环境加载默认配置。
// Windows、macOS 本地开发默认使用 video.yml，其他环境默认使用 video_prod.yml。
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
	if runtimeGOOS == "windows" || runtimeGOOS == "darwin" {
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
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	configCandidates := []string{DefaultConfigPath(), defaultConfigPath, prodConfigPath}
	projectDirs := []string{"", "video-service", "embedding-video"}

	dir := cwd
	for {
		for _, projectDir := range projectDirs {
			root := dir
			if projectDir != "" {
				root = filepath.Join(dir, projectDir)
			}
			for _, cfgPath := range configCandidates {
				if _, statErr := os.Stat(filepath.Join(root, filepath.FromSlash(cfgPath))); statErr == nil {
					return root, nil
				}
			}
			if moduleMatches(root) {
				return root, nil
			}
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return "", os.ErrNotExist
}

func moduleMatches(dir string) bool {
	data, err := os.ReadFile(filepath.Join(dir, "go.mod"))
	if err != nil {
		return false
	}
	moduleLine := strings.TrimSpace(strings.SplitN(string(data), "\n", 2)[0])
	return moduleLine == "module nlp-video-analysis"
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
