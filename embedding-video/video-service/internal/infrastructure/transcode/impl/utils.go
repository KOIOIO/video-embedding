package impl

import (
	"os"
	"path/filepath"
	"strings"
)

func ToContainerPath(cwd string, path string) (string, error) {
	absCwd, err := filepath.Abs(cwd)
	if err != nil {
		return "", err
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}

	rel, err := filepath.Rel(absCwd, absPath)
	if err != nil {
		return "", err
	}
	if rel == "." {
		return "/app", nil
	}
	rel = filepath.ToSlash(rel)
	rel = strings.TrimLeft(rel, "/")
	return "/app/" + rel, nil
}

func FindProjectRoot(start string) string {
	dir := start
	for i := 0; i < 8; i++ {
		if FileExists(filepath.Join(dir, "go.mod")) || FileExists(filepath.Join(dir, "configs", "video.yml")) {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return start
}

func FileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
