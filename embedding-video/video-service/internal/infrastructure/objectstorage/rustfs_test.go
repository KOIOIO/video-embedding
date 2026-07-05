package objectstorage

import (
	"testing"

	"github.com/minio/minio-go/v7"
)

func TestMinioOptionsForCOSUsesRegionAndDNSLookup(t *testing.T) {
	opts, err := minioOptions(Config{
		Endpoint:     "cos.ap-beijing.myqcloud.com",
		AccessKey:    "ak",
		SecretKey:    "sk",
		Bucket:       "video-embedding-storage-0000000000",
		UseSSL:       true,
		Region:       "ap-beijing",
		BucketLookup: "dns",
	})
	if err != nil {
		t.Fatalf("minioOptions returned error: %v", err)
	}

	if opts.Region != "ap-beijing" {
		t.Fatalf("Region = %q, want ap-beijing", opts.Region)
	}
	if opts.BucketLookup != minio.BucketLookupDNS {
		t.Fatalf("BucketLookup = %v, want %v", opts.BucketLookup, minio.BucketLookupDNS)
	}
}

func TestNormalizeEndpointStripsScheme(t *testing.T) {
	got := normalizeEndpoint(" https://cos.ap-beijing.myqcloud.com/ ")

	if got != "cos.ap-beijing.myqcloud.com" {
		t.Fatalf("normalizeEndpoint() = %q, want cos.ap-beijing.myqcloud.com", got)
	}
}

func TestNormalizeEndpointConvertsCOSBucketEndpointToServiceEndpoint(t *testing.T) {
	cfg := normalizedConfig(Config{
		Endpoint:     "https://video-embedding-storage-0000000000.cos.ap-beijing.myqcloud.com",
		Bucket:       "video-embedding-storage-0000000000",
		BucketLookup: "dns",
	})

	if cfg.Endpoint != "cos.ap-beijing.myqcloud.com" {
		t.Fatalf("Endpoint = %q, want cos.ap-beijing.myqcloud.com", cfg.Endpoint)
	}
	if cfg.BucketLookup != "dns" {
		t.Fatalf("BucketLookup = %q, want dns", cfg.BucketLookup)
	}
}

func TestBucketLookupOptionRejectsInvalidValue(t *testing.T) {
	_, err := bucketLookupOption("virtual")

	if err == nil {
		t.Fatal("bucketLookupOption returned nil error for invalid value")
	}
}
