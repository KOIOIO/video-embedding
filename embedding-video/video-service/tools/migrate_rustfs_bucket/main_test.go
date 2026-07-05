package main

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"

	"github.com/minio/minio-go/v7"
)

func TestParseOptionsUsesRustFSDefaults(t *testing.T) {
	t.Setenv("MIGRATE_SOURCE_ACCESS_KEY", "source-ak")
	t.Setenv("MIGRATE_SOURCE_SECRET_KEY", "source-sk")
	t.Setenv("MIGRATE_TARGET_ACCESS_KEY", "target-ak")
	t.Setenv("MIGRATE_TARGET_SECRET_KEY", "target-sk")

	opts, err := parseOptions(nil)
	if err != nil {
		t.Fatalf("parseOptions returned error: %v", err)
	}
	if opts.sourceEndpoint != defaultSourceEndpoint {
		t.Fatalf("sourceEndpoint=%q, want %q", opts.sourceEndpoint, defaultSourceEndpoint)
	}
	if opts.targetEndpoint != defaultTargetEndpoint {
		t.Fatalf("targetEndpoint=%q, want %q", opts.targetEndpoint, defaultTargetEndpoint)
	}
	if opts.sourceBucket != defaultBucket || opts.targetBucket != defaultBucket {
		t.Fatalf("unexpected buckets: source=%q target=%q", opts.sourceBucket, opts.targetBucket)
	}
	if opts.sourceAccessKey != "source-ak" || opts.sourceSecretKey != "source-sk" {
		t.Fatalf("source credentials=%q/%q, want env defaults", opts.sourceAccessKey, opts.sourceSecretKey)
	}
	if opts.targetAccessKey != "target-ak" || opts.targetSecretKey != "target-sk" {
		t.Fatalf("target credentials=%q/%q, want env defaults", opts.targetAccessKey, opts.targetSecretKey)
	}
	if !opts.dryRun {
		t.Fatal("dryRun=false, want true by default")
	}
}

func TestMigrateBucketDryRunListsWithoutCreatingTarget(t *testing.T) {
	src := newFakeObjectClient("video-embedding-storage")
	src.objects["raw/a.mp4"] = fakeObject{body: "a", info: minio.ObjectInfo{Key: "raw/a.mp4", Size: 1, ContentType: "video/mp4"}}
	dst := newFakeObjectClient("")
	opts := options{
		sourceEndpoint: "source",
		sourceBucket:   "video-embedding-storage",
		targetEndpoint: "target",
		targetBucket:   "video-embedding-storage",
		workers:        1,
		dryRun:         true,
	}

	var out bytes.Buffer
	stats, err := migrateBucket(context.Background(), src, dst, opts, &out)

	if err != nil {
		t.Fatalf("migrateBucket returned error: %v", err)
	}
	if stats.listed != 1 || stats.skipped != 1 || stats.copied != 0 {
		t.Fatalf("stats=%+v, want listed=1 skipped=1 copied=0", stats)
	}
	if dst.makeBucketCalled {
		t.Fatal("target bucket was created during dry run")
	}
}

func TestMigrateBucketCopiesMissingObjectsAndSkipsExisting(t *testing.T) {
	src := newFakeObjectClient("source-bucket")
	src.objects["raw/a.mp4"] = fakeObject{body: "a", info: minio.ObjectInfo{Key: "raw/a.mp4", Size: 1, ContentType: "video/mp4"}}
	src.objects["raw/b.mp4"] = fakeObject{body: "b", info: minio.ObjectInfo{Key: "raw/b.mp4", Size: 1, ContentType: "video/mp4"}}
	dst := newFakeObjectClient("target-bucket")
	dst.objects["raw/a.mp4"] = fakeObject{body: "old", info: minio.ObjectInfo{Key: "raw/a.mp4", Size: 3, ContentType: "video/mp4"}}
	opts := options{
		sourceEndpoint: "source",
		sourceBucket:   "source-bucket",
		targetEndpoint: "target",
		targetBucket:   "target-bucket",
		workers:        1,
		dryRun:         false,
		overwrite:      false,
	}

	stats, err := migrateBucket(context.Background(), src, dst, opts, io.Discard)

	if err != nil {
		t.Fatalf("migrateBucket returned error: %v", err)
	}
	if stats.listed != 2 || stats.copied != 1 {
		t.Fatalf("stats=%+v, want listed=2 copied=1", stats)
	}
	if got := dst.objects["raw/a.mp4"].body; got != "old" {
		t.Fatalf("existing object was overwritten: %q", got)
	}
	if got := dst.objects["raw/b.mp4"].body; got != "b" {
		t.Fatalf("missing object was not copied: %q", got)
	}
}

type fakeObject struct {
	body string
	info minio.ObjectInfo
}

type fakeObjectClient struct {
	bucket           string
	objects          map[string]fakeObject
	makeBucketCalled bool
}

func newFakeObjectClient(bucket string) *fakeObjectClient {
	return &fakeObjectClient{bucket: bucket, objects: map[string]fakeObject{}}
}

func (c *fakeObjectClient) BucketExists(_ context.Context, bucket string) (bool, error) {
	return c.bucket == bucket, nil
}

func (c *fakeObjectClient) MakeBucket(_ context.Context, bucket string, _ minio.MakeBucketOptions) error {
	c.bucket = bucket
	c.makeBucketCalled = true
	return nil
}

func (c *fakeObjectClient) ListObjects(_ context.Context, bucket string, opts minio.ListObjectsOptions) <-chan minio.ObjectInfo {
	ch := make(chan minio.ObjectInfo, len(c.objects))
	go func() {
		defer close(ch)
		if c.bucket != bucket {
			ch <- minio.ObjectInfo{Err: minio.ErrorResponse{Code: "NoSuchBucket"}}
			return
		}
		for key, obj := range c.objects {
			if opts.Prefix != "" && !strings.HasPrefix(key, opts.Prefix) {
				continue
			}
			ch <- obj.info
		}
	}()
	return ch
}

func (c *fakeObjectClient) StatObject(_ context.Context, bucket string, objectName string, _ minio.StatObjectOptions) (minio.ObjectInfo, error) {
	if c.bucket != bucket {
		return minio.ObjectInfo{}, minio.ErrorResponse{Code: "NoSuchBucket"}
	}
	obj, ok := c.objects[objectName]
	if !ok {
		return minio.ObjectInfo{}, minio.ErrorResponse{Code: "NoSuchKey"}
	}
	return obj.info, nil
}

func (c *fakeObjectClient) GetObject(_ context.Context, bucket string, objectName string, _ minio.GetObjectOptions) (io.ReadCloser, error) {
	if c.bucket != bucket {
		return nil, minio.ErrorResponse{Code: "NoSuchBucket"}
	}
	obj, ok := c.objects[objectName]
	if !ok {
		return nil, minio.ErrorResponse{Code: "NoSuchKey"}
	}
	return io.NopCloser(strings.NewReader(obj.body)), nil
}

func (c *fakeObjectClient) PutObject(_ context.Context, bucket string, objectName string, reader io.Reader, objectSize int64, opts minio.PutObjectOptions) (minio.UploadInfo, error) {
	if c.bucket != bucket {
		return minio.UploadInfo{}, minio.ErrorResponse{Code: "NoSuchBucket"}
	}
	body, err := io.ReadAll(reader)
	if err != nil {
		return minio.UploadInfo{}, err
	}
	c.objects[objectName] = fakeObject{
		body: string(body),
		info: minio.ObjectInfo{Key: objectName, Size: objectSize, ContentType: opts.ContentType},
	}
	return minio.UploadInfo{Key: objectName, Size: objectSize}, nil
}
