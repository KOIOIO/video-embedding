package impl

import (
	"os"
	"path/filepath"
	"testing"
)

func TestToContainerPathMapsProjectRelativePaths(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "videos", "raw.mp4")

	got, err := ToContainerPath(root, nested)
	if err != nil {
		t.Fatalf("ToContainerPath returned error: %v", err)
	}
	if got != "/app/videos/raw.mp4" {
		t.Fatalf("ToContainerPath() = %q", got)
	}

	got, err = ToContainerPath(root, root)
	if err != nil {
		t.Fatalf("ToContainerPath root returned error: %v", err)
	}
	if got != "/app" {
		t.Fatalf("ToContainerPath(root) = %q", got)
	}
}

func TestFindProjectRootFindsGoModOrConfig(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "a", "b")
	if err := os.MkdirAll(nested, 0755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example\n"), 0644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	if got := FindProjectRoot(nested); got != root {
		t.Fatalf("FindProjectRoot() = %q, want %q", got, root)
	}
}

func TestFileExistsReportsExistingRegularFile(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "exists.txt")
	if err := os.WriteFile(file, []byte("ok"), 0644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	if !FileExists(file) {
		t.Fatal("expected file to exist")
	}
	if FileExists(filepath.Join(root, "missing.txt")) {
		t.Fatal("expected missing file to not exist")
	}
}
