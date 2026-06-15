package objectstorage

import (
	"context"
	"errors"
	"io"
	"mime"
	"os"
	"path/filepath"
	"strings"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// RustFS 封装 S3 兼容对象存储客户端。
type RustFS struct {
	client *minio.Client
	bucket string
}

// Config 定义对象存储连接配置。
type Config struct {
	Endpoint  string
	AccessKey string
	SecretKey string
	Bucket    string
	UseSSL    bool
}

// NewRustFS 创建一个基于 MinIO SDK 的对象存储客户端。
func NewRustFS(cfg Config) (*RustFS, error) {
	if strings.TrimSpace(cfg.Endpoint) == "" {
		return nil, errors.New("endpoint is required")
	}
	if strings.TrimSpace(cfg.Bucket) == "" {
		return nil, errors.New("bucket is required")
	}
	if strings.TrimSpace(cfg.AccessKey) == "" {
		return nil, errors.New("accessKey is required")
	}
	if strings.TrimSpace(cfg.SecretKey) == "" {
		return nil, errors.New("secretKey is required")
	}

	cli, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.UseSSL,
	})
	if err != nil {
		return nil, err
	}

	return &RustFS{
		client: cli,
		bucket: cfg.Bucket,
	}, nil
}

// EnsureBucket 确保存储桶存在，不存在则尝试创建。
func (s *RustFS) EnsureBucket(ctx context.Context) error {
	exists, err := s.client.BucketExists(ctx, s.bucket)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	return s.client.MakeBucket(ctx, s.bucket, minio.MakeBucketOptions{})
}

// PutFile 把本地文件上传到对象存储。
func (s *RustFS) PutFile(ctx context.Context, objectKey string, filePath string, contentType string) error {
	if strings.TrimSpace(contentType) == "" {
		contentType = guessContentType(filePath)
	}
	_, err := s.client.FPutObject(ctx, s.bucket, cleanKey(objectKey), filePath, minio.PutObjectOptions{
		ContentType: contentType,
	})
	return err
}

// Put 把任意 Reader 内容流式写入对象存储。
func (s *RustFS) Put(ctx context.Context, objectKey string, r io.Reader, size int64, contentType string) error {
	if strings.TrimSpace(contentType) == "" {
		contentType = "application/octet-stream"
	}
	_, err := s.client.PutObject(ctx, s.bucket, cleanKey(objectKey), r, size, minio.PutObjectOptions{
		ContentType: contentType,
	})
	return err
}

// DownloadToFile 把对象存储中的文件下载到本地路径。
func (s *RustFS) DownloadToFile(ctx context.Context, objectKey string, filePath string) error {
	return s.client.FGetObject(ctx, s.bucket, cleanKey(objectKey), filePath, minio.GetObjectOptions{})
}

// ObjectInfo 描述对象的最小元信息集合。
type ObjectInfo struct {
	Size        int64
	ContentType string
	ETag        string
}

// Stat 读取对象元信息。
func (s *RustFS) Stat(ctx context.Context, objectKey string) (ObjectInfo, error) {
	st, err := s.client.StatObject(ctx, s.bucket, cleanKey(objectKey), minio.StatObjectOptions{})
	if err != nil {
		return ObjectInfo{}, err
	}
	return ObjectInfo{
		Size:        st.Size,
		ContentType: st.ContentType,
		ETag:        st.ETag,
	}, nil
}

// Get 获取对象读取句柄。
func (s *RustFS) Get(ctx context.Context, objectKey string, opts minio.GetObjectOptions) (*minio.Object, error) {
	return s.client.GetObject(ctx, s.bucket, cleanKey(objectKey), opts)
}

// DirUploader 负责把本地目录整体上传到对象存储前缀下。
type DirUploader struct {
	store *RustFS
}

// NewDirUploader 创建目录上传器。
func NewDirUploader(store *RustFS) *DirUploader {
	return &DirUploader{store: store}
}

// UploadDir 遍历本地目录，并按相对路径上传每个文件。
func (u *DirUploader) UploadDir(ctx context.Context, localDir string, objectPrefix string) error {
	localDir = filepath.Clean(localDir)
	objectPrefix = strings.Trim(cleanKey(objectPrefix), "/")
	if objectPrefix != "" {
		objectPrefix = objectPrefix + "/"
	}

	return filepath.WalkDir(localDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(localDir, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		key := objectPrefix + rel
		return u.store.PutFile(ctx, key, path, guessContentType(path))
	})
}

// guessContentType 根据扩展名推断上传时的 Content-Type。
func guessContentType(path string) string {
	ext := filepath.Ext(path)
	ct := mime.TypeByExtension(ext)
	if ct != "" {
		return ct
	}
	switch strings.ToLower(ext) {
	case ".m3u8":
		return "application/vnd.apple.mpegurl"
	case ".ts":
		return "video/mp2t"
	case ".mp4":
		return "video/mp4"
	default:
		return "application/octet-stream"
	}
}

// cleanKey 统一清洗对象键格式，避免 Windows 分隔符与前导斜杠问题。
func cleanKey(key string) string {
	key = strings.TrimSpace(key)
	key = strings.TrimPrefix(key, "/")
	key = strings.ReplaceAll(key, "\\", "/")
	return key
}

// Range 表示一个字节范围请求。
type Range struct {
	Start int64
	End   int64
}

// ParseRangeHeader 解析单个 HTTP Range 请求头。
func ParseRangeHeader(value string, size int64) (Range, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return Range{}, false
	}
	if !strings.HasPrefix(value, "bytes=") {
		return Range{}, false
	}
	spec := strings.TrimPrefix(value, "bytes=")
	parts := strings.SplitN(spec, "-", 2)
	if len(parts) != 2 {
		return Range{}, false
	}
	if parts[0] == "" {
		return Range{}, false
	}
	start, err := parseInt64(parts[0])
	if err != nil || start < 0 {
		return Range{}, false
	}
	end := size - 1
	if parts[1] != "" {
		e, err := parseInt64(parts[1])
		if err != nil || e < start {
			return Range{}, false
		}
		end = e
	}
	if size > 0 && end >= size {
		end = size - 1
	}
	return Range{Start: start, End: end}, true
}

// parseInt64 解析十进制非负整数。
func parseInt64(s string) (int64, error) {
	var v int64
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0, errors.New("invalid integer")
		}
		v = v*10 + int64(r-'0')
	}
	return v, nil
}

// CopyAndClose 把源对象复制到目标 Writer，并在结束后关闭源 Reader。
func CopyAndClose(dst io.Writer, src io.ReadCloser) error {
	defer src.Close()
	_, err := io.Copy(dst, src)
	return err
}
