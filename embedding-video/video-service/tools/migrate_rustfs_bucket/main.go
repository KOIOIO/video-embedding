package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

const (
	defaultBucket         = "video-embedding-storage"
	defaultSourceEndpoint = "127.0.0.1:9000"
	defaultTargetEndpoint = "127.0.0.1:9001"
)

type options struct {
	sourceEndpoint  string
	sourceAccessKey string
	sourceSecretKey string
	sourceBucket    string
	sourceSSL       bool
	targetEndpoint  string
	targetAccessKey string
	targetSecretKey string
	targetBucket    string
	targetSSL       bool
	prefix          string
	workers         int
	overwrite       bool
	dryRun          bool
}

type objectClient interface {
	BucketExists(ctx context.Context, bucket string) (bool, error)
	MakeBucket(ctx context.Context, bucket string, opts minio.MakeBucketOptions) error
	ListObjects(ctx context.Context, bucket string, opts minio.ListObjectsOptions) <-chan minio.ObjectInfo
	StatObject(ctx context.Context, bucket string, objectName string, opts minio.StatObjectOptions) (minio.ObjectInfo, error)
	GetObject(ctx context.Context, bucket string, objectName string, opts minio.GetObjectOptions) (io.ReadCloser, error)
	PutObject(ctx context.Context, bucket string, objectName string, reader io.Reader, objectSize int64, opts minio.PutObjectOptions) (minio.UploadInfo, error)
}

type minioObjectClient struct {
	client *minio.Client
}

func (c minioObjectClient) BucketExists(ctx context.Context, bucket string) (bool, error) {
	return c.client.BucketExists(ctx, bucket)
}

func (c minioObjectClient) MakeBucket(ctx context.Context, bucket string, opts minio.MakeBucketOptions) error {
	return c.client.MakeBucket(ctx, bucket, opts)
}

func (c minioObjectClient) ListObjects(ctx context.Context, bucket string, opts minio.ListObjectsOptions) <-chan minio.ObjectInfo {
	return c.client.ListObjects(ctx, bucket, opts)
}

func (c minioObjectClient) StatObject(ctx context.Context, bucket string, objectName string, opts minio.StatObjectOptions) (minio.ObjectInfo, error) {
	return c.client.StatObject(ctx, bucket, objectName, opts)
}

func (c minioObjectClient) GetObject(ctx context.Context, bucket string, objectName string, opts minio.GetObjectOptions) (io.ReadCloser, error) {
	return c.client.GetObject(ctx, bucket, objectName, opts)
}

func (c minioObjectClient) PutObject(ctx context.Context, bucket string, objectName string, reader io.Reader, objectSize int64, opts minio.PutObjectOptions) (minio.UploadInfo, error) {
	return c.client.PutObject(ctx, bucket, objectName, reader, objectSize, opts)
}

type migrationStats struct {
	listed  int64
	copied  int64
	skipped int64
	failed  int64
	bytes   int64
}

type migrateAction int

const (
	actionCopied migrateAction = iota
	actionSkipped
)

func main() {
	if err := run(context.Background(), os.Args[1:], os.Stdout); err != nil {
		log.Fatal(err)
	}
}

func parseOptions(args []string) (options, error) {
	opts := options{
		sourceEndpoint:  defaultSourceEndpoint,
		sourceAccessKey: firstEnv("MIGRATE_SOURCE_ACCESS_KEY", "SOURCE_ACCESS_KEY"),
		sourceSecretKey: firstEnv("MIGRATE_SOURCE_SECRET_KEY", "SOURCE_SECRET_KEY"),
		sourceBucket:    defaultBucket,
		targetEndpoint:  defaultTargetEndpoint,
		targetAccessKey: firstEnv("MIGRATE_TARGET_ACCESS_KEY", "TARGET_ACCESS_KEY"),
		targetSecretKey: firstEnv("MIGRATE_TARGET_SECRET_KEY", "TARGET_SECRET_KEY"),
		targetBucket:    defaultBucket,
		workers:         4,
		dryRun:          true,
	}
	fs := flag.NewFlagSet("migrate_rustfs_bucket", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&opts.sourceEndpoint, "source-endpoint", opts.sourceEndpoint, "source S3 API endpoint host:port")
	fs.StringVar(&opts.sourceAccessKey, "source-access-key", opts.sourceAccessKey, "source access key")
	fs.StringVar(&opts.sourceSecretKey, "source-secret-key", opts.sourceSecretKey, "source secret key")
	fs.StringVar(&opts.sourceBucket, "source-bucket", opts.sourceBucket, "source bucket")
	fs.BoolVar(&opts.sourceSSL, "source-ssl", false, "use HTTPS for source endpoint")
	fs.StringVar(&opts.targetEndpoint, "target-endpoint", opts.targetEndpoint, "target S3 API endpoint host:port")
	fs.StringVar(&opts.targetAccessKey, "target-access-key", opts.targetAccessKey, "target access key")
	fs.StringVar(&opts.targetSecretKey, "target-secret-key", opts.targetSecretKey, "target secret key")
	fs.StringVar(&opts.targetBucket, "target-bucket", opts.targetBucket, "target bucket")
	fs.BoolVar(&opts.targetSSL, "target-ssl", false, "use HTTPS for target endpoint")
	fs.StringVar(&opts.prefix, "prefix", "", "optional object key prefix to migrate")
	fs.IntVar(&opts.workers, "workers", opts.workers, "parallel copy workers")
	fs.BoolVar(&opts.overwrite, "overwrite", false, "overwrite existing target objects")
	fs.BoolVar(&opts.dryRun, "dry-run", opts.dryRun, "list objects and actions without copying")
	if err := fs.Parse(args); err != nil {
		return options{}, err
	}
	opts.sourceEndpoint = strings.TrimSpace(opts.sourceEndpoint)
	opts.sourceAccessKey = strings.TrimSpace(opts.sourceAccessKey)
	opts.sourceSecretKey = strings.TrimSpace(opts.sourceSecretKey)
	opts.sourceBucket = strings.TrimSpace(opts.sourceBucket)
	opts.targetEndpoint = strings.TrimSpace(opts.targetEndpoint)
	opts.targetAccessKey = strings.TrimSpace(opts.targetAccessKey)
	opts.targetSecretKey = strings.TrimSpace(opts.targetSecretKey)
	opts.targetBucket = strings.TrimSpace(opts.targetBucket)
	opts.prefix = strings.TrimLeft(strings.TrimSpace(opts.prefix), "/")
	switch {
	case opts.sourceEndpoint == "":
		return options{}, errors.New("source-endpoint is required")
	case opts.sourceAccessKey == "":
		return options{}, errors.New("source-access-key is required")
	case opts.sourceSecretKey == "":
		return options{}, errors.New("source-secret-key is required")
	case opts.sourceBucket == "":
		return options{}, errors.New("source-bucket is required")
	case opts.targetEndpoint == "":
		return options{}, errors.New("target-endpoint is required")
	case opts.targetAccessKey == "":
		return options{}, errors.New("target-access-key is required")
	case opts.targetSecretKey == "":
		return options{}, errors.New("target-secret-key is required")
	case opts.targetBucket == "":
		return options{}, errors.New("target-bucket is required")
	case opts.workers <= 0:
		return options{}, errors.New("workers must be greater than 0")
	}
	return opts, nil
}

func firstEnv(names ...string) string {
	for _, name := range names {
		if value := strings.TrimSpace(os.Getenv(name)); value != "" {
			return value
		}
	}
	return ""
}

func run(ctx context.Context, args []string, out io.Writer) error {
	opts, err := parseOptions(args)
	if err != nil {
		return err
	}
	src, err := newMinioClient(opts.sourceEndpoint, opts.sourceAccessKey, opts.sourceSecretKey, opts.sourceSSL)
	if err != nil {
		return fmt.Errorf("create source client: %w", err)
	}
	dst, err := newMinioClient(opts.targetEndpoint, opts.targetAccessKey, opts.targetSecretKey, opts.targetSSL)
	if err != nil {
		return fmt.Errorf("create target client: %w", err)
	}
	stats, err := migrateBucket(ctx, minioObjectClient{client: src}, minioObjectClient{client: dst}, opts, out)
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "done listed=%d copied=%d skipped=%d failed=%d bytes=%d dry_run=%v overwrite=%v\n",
		stats.listed,
		stats.copied,
		stats.skipped,
		stats.failed,
		stats.bytes,
		opts.dryRun,
		opts.overwrite,
	)
	return nil
}

func newMinioClient(endpoint string, accessKey string, secretKey string, secure bool) (*minio.Client, error) {
	return minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: secure,
	})
}

func migrateBucket(ctx context.Context, src objectClient, dst objectClient, opts options, out io.Writer) (migrationStats, error) {
	if err := ensureSourceBucket(ctx, src, opts.sourceBucket); err != nil {
		return migrationStats{}, err
	}
	if !opts.dryRun {
		if err := ensureTargetBucket(ctx, dst, opts.targetBucket); err != nil {
			return migrationStats{}, err
		}
	}

	start := time.Now()
	fmt.Fprintf(out, "source=%s/%s target=%s/%s prefix=%q workers=%d dry_run=%v overwrite=%v\n",
		opts.sourceEndpoint,
		opts.sourceBucket,
		opts.targetEndpoint,
		opts.targetBucket,
		opts.prefix,
		opts.workers,
		opts.dryRun,
		opts.overwrite,
	)

	jobs := make(chan minio.ObjectInfo, opts.workers*2)
	errs := make(chan error, 1)
	var stats migrationStats
	var wg sync.WaitGroup
	for i := 0; i < opts.workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for obj := range jobs {
				action, err := migrateObject(ctx, src, dst, opts, obj)
				if err != nil {
					atomic.AddInt64(&stats.failed, 1)
					select {
					case errs <- err:
					default:
					}
					continue
				}
				if action == actionSkipped {
					atomic.AddInt64(&stats.skipped, 1)
					continue
				}
				atomic.AddInt64(&stats.copied, 1)
				atomic.AddInt64(&stats.bytes, obj.Size)
			}
		}()
	}

	for obj := range src.ListObjects(ctx, opts.sourceBucket, minio.ListObjectsOptions{
		Prefix:    opts.prefix,
		Recursive: true,
	}) {
		if obj.Err != nil {
			close(jobs)
			wg.Wait()
			return stats, obj.Err
		}
		atomic.AddInt64(&stats.listed, 1)
		select {
		case jobs <- obj:
		case err := <-errs:
			close(jobs)
			wg.Wait()
			return stats, err
		case <-ctx.Done():
			close(jobs)
			wg.Wait()
			return stats, ctx.Err()
		}
	}
	close(jobs)
	wg.Wait()
	select {
	case err := <-errs:
		return stats, err
	default:
	}
	fmt.Fprintf(out, "elapsed=%s\n", time.Since(start).Round(time.Millisecond))
	return stats, nil
}

func ensureSourceBucket(ctx context.Context, client objectClient, bucket string) error {
	exists, err := client.BucketExists(ctx, bucket)
	if err != nil {
		return fmt.Errorf("check source bucket: %w", err)
	}
	if !exists {
		return fmt.Errorf("source bucket %q does not exist", bucket)
	}
	return nil
}

func ensureTargetBucket(ctx context.Context, client objectClient, bucket string) error {
	exists, err := client.BucketExists(ctx, bucket)
	if err != nil {
		return fmt.Errorf("check target bucket: %w", err)
	}
	if exists {
		return nil
	}
	if err := client.MakeBucket(ctx, bucket, minio.MakeBucketOptions{}); err != nil {
		return fmt.Errorf("create target bucket: %w", err)
	}
	return nil
}

func migrateObject(ctx context.Context, src objectClient, dst objectClient, opts options, obj minio.ObjectInfo) (migrateAction, error) {
	if strings.TrimSpace(obj.Key) == "" {
		return actionSkipped, nil
	}
	if !opts.overwrite {
		if _, err := dst.StatObject(ctx, opts.targetBucket, obj.Key, minio.StatObjectOptions{}); err == nil {
			return actionSkipped, nil
		}
	}
	if opts.dryRun {
		return actionSkipped, nil
	}
	reader, err := src.GetObject(ctx, opts.sourceBucket, obj.Key, minio.GetObjectOptions{})
	if err != nil {
		return actionSkipped, fmt.Errorf("get source object %q: %w", obj.Key, err)
	}
	defer reader.Close()
	_, err = dst.PutObject(ctx, opts.targetBucket, obj.Key, reader, obj.Size, minio.PutObjectOptions{
		ContentType:  obj.ContentType,
		UserMetadata: obj.UserMetadata,
	})
	if err != nil {
		return actionSkipped, fmt.Errorf("put target object %q: %w", obj.Key, err)
	}
	return actionCopied, nil
}
