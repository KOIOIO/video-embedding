package fs

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLocalFileStorageCreatesAndRemovesFiles(t *testing.T) {
	storage := NewLocalFileStorage()
	root := t.TempDir()
	dir := filepath.Join(root, "nested")
	file := filepath.Join(dir, "video.txt")

	if err := storage.MkdirAll(dir); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}

	w, err := storage.Create(file)
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if _, err := w.Write([]byte("content")); err != nil {
		t.Fatalf("Write returned error: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}

	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	if string(data) != "content" {
		t.Fatalf("file content = %q", data)
	}

	if err := storage.Remove(file); err != nil {
		t.Fatalf("Remove returned error: %v", err)
	}
	if _, err := os.Stat(file); !os.IsNotExist(err) {
		t.Fatalf("expected file to be removed, stat err = %v", err)
	}

	if err := storage.RemoveAll(dir); err != nil {
		t.Fatalf("RemoveAll returned error: %v", err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("expected dir to be removed, stat err = %v", err)
	}
}
