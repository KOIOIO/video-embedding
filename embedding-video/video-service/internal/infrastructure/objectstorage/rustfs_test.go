package objectstorage

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/minio/minio-go/v7"
)

func TestMinioOptionsForCOSUsesRegionAndDNSLookup(t *testing.T) {
	opts, err := minioOptions(Config{
		Endpoint:     "cos.ap-beijing.myqcloud.com",
		AccessKey:    "ak",
		SecretKey:    "sk",
		Bucket:       "video-object-storage",
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

func TestDeletePrefixRejectsEmptyPrefix(t *testing.T) {
	client := &prefixTestClient{}
	err := deletePrefix(context.Background(), client, "bucket", " / ")
	if err == nil {
		t.Fatal("deletePrefix() error = nil")
	}
	if client.listPrefix != "" {
		t.Fatalf("ListObjects called with %q", client.listPrefix)
	}
}

func TestDeletePrefixListsAndDeletesExactPrefix(t *testing.T) {
	client := &prefixTestClient{objects: []minio.ObjectInfo{{Key: "hls/41/master.m3u8"}, {Key: "hls/41/segment.ts"}}}
	if err := deletePrefix(context.Background(), client, "bucket", "/hls/41/"); err != nil {
		t.Fatalf("deletePrefix() error = %v", err)
	}
	if client.listPrefix != "hls/41/" {
		t.Fatalf("listed prefix = %q, want hls/41/", client.listPrefix)
	}
	want := []string{"hls/41/master.m3u8", "hls/41/segment.ts"}
	if !reflect.DeepEqual(client.removedKeys, want) {
		t.Fatalf("removed keys = %v, want %v", client.removedKeys, want)
	}
}

func TestDeletePrefixCollectsListAndRemovalErrors(t *testing.T) {
	listErr := errors.New("list failed")
	removeErr := errors.New("remove failed")
	client := &prefixTestClient{
		objects:   []minio.ObjectInfo{{Key: "hls/41/master.m3u8"}, {Err: listErr}},
		removeErr: removeErr,
	}
	err := deletePrefix(context.Background(), client, "bucket", "hls/41")
	if !errors.Is(err, listErr) || !errors.Is(err, removeErr) {
		t.Fatalf("deletePrefix() error = %v, want list and removal errors", err)
	}
}

type prefixTestClient struct {
	objects     []minio.ObjectInfo
	listPrefix  string
	removedKeys []string
	removeErr   error
}

func (c *prefixTestClient) ListObjects(_ context.Context, _ string, opts minio.ListObjectsOptions) <-chan minio.ObjectInfo {
	c.listPrefix = opts.Prefix
	result := make(chan minio.ObjectInfo, len(c.objects))
	for _, object := range c.objects {
		result <- object
	}
	close(result)
	return result
}

func (c *prefixTestClient) RemoveObjects(_ context.Context, _ string, objects <-chan minio.ObjectInfo, _ minio.RemoveObjectsOptions) <-chan minio.RemoveObjectError {
	result := make(chan minio.RemoveObjectError, 1)
	for object := range objects {
		c.removedKeys = append(c.removedKeys, object.Key)
	}
	if c.removeErr != nil {
		result <- minio.RemoveObjectError{ObjectName: "hls/41/master.m3u8", Err: c.removeErr}
	}
	close(result)
	return result
}

func TestNormalizeEndpointStripsScheme(t *testing.T) {
	got := normalizeEndpoint(" https://cos.ap-beijing.myqcloud.com/ ")

	if got != "cos.ap-beijing.myqcloud.com" {
		t.Fatalf("normalizeEndpoint() = %q, want cos.ap-beijing.myqcloud.com", got)
	}
}

func TestNormalizeEndpointConvertsCOSBucketEndpointToServiceEndpoint(t *testing.T) {
	cfg := normalizedConfig(Config{
		Endpoint:     "https://video-object-storage.cos.ap-beijing.myqcloud.com",
		Bucket:       "video-object-storage",
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
