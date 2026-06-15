package fs

import (
	"io"
	"os"
)

type LocalFileStorage struct{}

func NewLocalFileStorage() *LocalFileStorage {
	return &LocalFileStorage{}
}

func (s *LocalFileStorage) MkdirAll(path string) error {
	return os.MkdirAll(path, 0755)
}

func (s *LocalFileStorage) Create(path string) (io.WriteCloser, error) {
	return os.Create(path)
}

func (s *LocalFileStorage) RemoveAll(path string) error {
	return os.RemoveAll(path)
}

func (s *LocalFileStorage) Remove(path string) error {
	return os.Remove(path)
}
