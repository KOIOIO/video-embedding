package config

import (
	"os"
	"strings"

	"nlp-video-analysis/internal/infrastructure/objectstorage"
)

// ObjectStorageConfig 转换运行配置为 S3 兼容对象存储客户端配置。
func ObjectStorageConfig(cfg Config) objectstorage.Config {
	accessKey := cfg.RustFS.AccessKey
	if strings.TrimSpace(accessKey) == "" {
		accessKey = firstEnv("COS_SECRET_ID", "RUSTFS_ACCESS_KEY")
	}
	secretKey := cfg.RustFS.SecretKey
	if strings.TrimSpace(secretKey) == "" {
		secretKey = firstEnv("COS_SECRET_KEY", "RUSTFS_SECRET_KEY")
	}
	return objectstorage.Config{
		Endpoint:     cfg.RustFS.Endpoint,
		AccessKey:    accessKey,
		SecretKey:    secretKey,
		Bucket:       cfg.RustFS.Bucket,
		UseSSL:       cfg.RustFS.UseSSL,
		Region:       cfg.RustFS.Region,
		BucketLookup: cfg.RustFS.BucketLookup,
	}
}

func firstEnv(names ...string) string {
	for _, name := range names {
		if value := strings.TrimSpace(os.Getenv(name)); value != "" {
			return value
		}
	}
	return ""
}
