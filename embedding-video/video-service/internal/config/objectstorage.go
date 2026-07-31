package config

import (
	"os"
	"strings"

	"video-service/internal/infrastructure/objectstorage"
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

// KnowledgeVideoObjectStorageConfig returns the isolated object storage configuration for knowledge videos.
func KnowledgeVideoObjectStorageConfig(cfg Config) objectstorage.Config {
	fallback := ObjectStorageConfig(cfg)
	storage := cfg.KnowledgeVideoStorage
	if strings.TrimSpace(storage.Endpoint) == "" {
		storage.Endpoint = objectstorage.NormalizeEndpoint(fallback.Endpoint, fallback.Bucket)
		storage.UseSSL = fallback.UseSSL
		storage.Region = fallback.Region
		storage.BucketLookup = fallback.BucketLookup
	}
	return objectstorage.Config{
		Endpoint:     firstConfigValue(storage.Endpoint, fallback.Endpoint),
		AccessKey:    firstConfigValue(storage.AccessKey, fallback.AccessKey),
		SecretKey:    firstConfigValue(storage.SecretKey, fallback.SecretKey),
		Bucket:       firstConfigValue(storage.Bucket, defaultKnowledgeVideoBucket),
		UseSSL:       storage.UseSSL,
		Region:       storage.Region,
		BucketLookup: storage.BucketLookup,
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
