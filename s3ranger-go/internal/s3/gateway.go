package s3

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// DownloadProgress tracks byte-level progress for a download operation.
type DownloadProgress struct {
	BytesDownloaded atomic.Int64
	TotalBytes      int64
	StartedAt       time.Time
}

// Percent returns the download completion percentage (0.0 to 1.0).
func (p *DownloadProgress) Percent() float64 {
	if p.TotalBytes <= 0 {
		return 0
	}
	return float64(p.BytesDownloaded.Load()) / float64(p.TotalBytes)
}

// Bandwidth returns a human-readable bandwidth string (e.g. "2.3 MB/s").
func (p *DownloadProgress) Bandwidth() string {
	elapsed := time.Since(p.StartedAt).Seconds()
	if elapsed < 0.1 {
		return "-- B/s"
	}
	bps := float64(p.BytesDownloaded.Load()) / elapsed
	return formatRate(bps)
}

// ETA returns an estimated time remaining string (e.g. "~12s").
func (p *DownloadProgress) ETA() string {
	elapsed := time.Since(p.StartedAt).Seconds()
	if elapsed < 0.1 || p.TotalBytes <= 0 {
		return "~..."
	}
	downloaded := p.BytesDownloaded.Load()
	if downloaded <= 0 {
		return "~..."
	}
	bps := float64(downloaded) / elapsed
	remaining := float64(p.TotalBytes-downloaded) / bps
	if remaining < 1 {
		return "~0s"
	}
	if remaining < 60 {
		return fmt.Sprintf("~%ds", int(remaining))
	}
	return fmt.Sprintf("~%dm%ds", int(remaining)/60, int(remaining)%60)
}

func formatRate(bps float64) string {
	const (
		KB = 1024
		MB = 1024 * KB
		GB = 1024 * MB
	)
	switch {
	case bps < KB:
		return fmt.Sprintf("%.0f B/s", bps)
	case bps < MB:
		return fmt.Sprintf("%.1f KB/s", bps/float64(KB))
	case bps < GB:
		return fmt.Sprintf("%.1f MB/s", bps/float64(MB))
	default:
		return fmt.Sprintf("%.1f GB/s", bps/float64(GB))
	}
}

// progressWriterAt wraps an io.WriterAt and atomically tracks bytes written.
type progressWriterAt struct {
	writer   *os.File
	progress *DownloadProgress
}

func (pw *progressWriterAt) WriteAt(p []byte, off int64) (int, error) {
	n, err := pw.writer.WriteAt(p, off)
	if n > 0 {
		pw.progress.BytesDownloaded.Add(int64(n))
	}
	return n, err
}

type Gateway struct {
	client     *s3.Client
	uploader   *manager.Uploader
	downloader *manager.Downloader
}

func NewGateway(cfg aws.Config, endpointURL string) *Gateway {
	var opts []func(*s3.Options)
	if endpointURL != "" {
		opts = append(opts, func(o *s3.Options) {
			o.BaseEndpoint = aws.String(endpointURL)
			o.UsePathStyle = true
		})
	}

	client := s3.NewFromConfig(cfg, opts...)
	return &Gateway{
		client:     client,
		uploader:   manager.NewUploader(client),
		downloader: manager.NewDownloader(client),
	}
}

// BucketInfo holds bucket metadata.
type BucketInfo struct {
	Name   string
	Region string
}

// BucketPage holds a page of bucket results.
type BucketPage struct {
	Buckets           []BucketInfo
	ContinuationToken string
	HasMore           bool
}

// ObjectInfo holds object metadata.
type ObjectInfo struct {
	Key          string
	Size         int64
	LastModified time.Time
	IsFolder     bool
}

// ObjectPage holds a page of object results.
type ObjectPage struct {
	Files             []ObjectInfo
	Folders           []ObjectInfo
	ContinuationToken string
	HasMore           bool
}

func (g *Gateway) ListBuckets(ctx context.Context, prefix string, maxBuckets int32, continuationToken string) (*BucketPage, error) {
	input := &s3.ListBucketsInput{}
	if prefix != "" {
		input.Prefix = aws.String(prefix)
	}
	if maxBuckets > 0 {
		input.MaxBuckets = aws.Int32(maxBuckets)
	}
	if continuationToken != "" {
		input.ContinuationToken = aws.String(continuationToken)
	}

	output, err := g.client.ListBuckets(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("listing buckets: %w", err)
	}

	page := &BucketPage{}
	for _, b := range output.Buckets {
		page.Buckets = append(page.Buckets, BucketInfo{
			Name: aws.ToString(b.Name),
		})
	}

	if output.ContinuationToken != nil {
		page.ContinuationToken = *output.ContinuationToken
		page.HasMore = true
	}

	return page, nil
}

func (g *Gateway) ListObjectsForPrefix(ctx context.Context, bucket, prefix string, maxKeys int32, continuationToken string) (*ObjectPage, error) {
	input := &s3.ListObjectsV2Input{
		Bucket:    aws.String(bucket),
		Delimiter: aws.String("/"),
	}
	if prefix != "" {
		input.Prefix = aws.String(prefix)
	}
	if maxKeys > 0 {
		input.MaxKeys = aws.Int32(maxKeys)
	}
	if continuationToken != "" {
		input.ContinuationToken = aws.String(continuationToken)
	}

	output, err := g.client.ListObjectsV2(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("listing objects: %w", err)
	}

	page := &ObjectPage{}

	for _, cp := range output.CommonPrefixes {
		page.Folders = append(page.Folders, ObjectInfo{
			Key:      aws.ToString(cp.Prefix),
			IsFolder: true,
		})
	}

	for _, obj := range output.Contents {
		key := aws.ToString(obj.Key)
		// Skip the prefix itself if it appears as an object
		if key == prefix {
			continue
		}
		page.Files = append(page.Files, ObjectInfo{
			Key:          key,
			Size:         aws.ToInt64(obj.Size),
			LastModified: aws.ToTime(obj.LastModified),
		})
	}

	if output.IsTruncated != nil && *output.IsTruncated {
		page.ContinuationToken = aws.ToString(output.NextContinuationToken)
		page.HasMore = true
	}

	return page, nil
}

func (g *Gateway) ListAllObjectsForPrefix(ctx context.Context, bucket, prefix string) ([]ObjectInfo, error) {
	var allObjects []ObjectInfo
	paginator := s3.NewListObjectsV2Paginator(g.client, &s3.ListObjectsV2Input{
		Bucket: aws.String(bucket),
		Prefix: aws.String(prefix),
	})

	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("listing all objects: %w", err)
		}
		for _, obj := range output.Contents {
			allObjects = append(allObjects, ObjectInfo{
				Key:          aws.ToString(obj.Key),
				Size:         aws.ToInt64(obj.Size),
				LastModified: aws.ToTime(obj.LastModified),
			})
		}
	}
	return allObjects, nil
}

func (g *Gateway) UploadFile(ctx context.Context, localPath, bucket, key string) error {
	f, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("opening file: %w", err)
	}
	defer f.Close()

	_, err = g.uploader.Upload(ctx, &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Body:   f,
	})
	if err != nil {
		return fmt.Errorf("uploading file: %w", err)
	}
	return nil
}

func (g *Gateway) UploadDirectory(ctx context.Context, localDir, bucket, prefix string) error {
	return filepath.WalkDir(localDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		relPath, err := filepath.Rel(localDir, path)
		if err != nil {
			return err
		}

		key := prefix + strings.ReplaceAll(relPath, string(os.PathSeparator), "/")
		if err := g.UploadFile(ctx, path, bucket, key); err != nil {
			return fmt.Errorf("uploading %s: %w", relPath, err)
		}
		return nil
	})
}

func (g *Gateway) DownloadFile(ctx context.Context, bucket, key, localPath string) error {
	// If localPath is a directory, append the filename
	if info, err := os.Stat(localPath); err == nil && info.IsDir() {
		localPath = filepath.Join(localPath, filepath.Base(key))
	} else if strings.HasSuffix(localPath, string(os.PathSeparator)) || strings.HasSuffix(localPath, "/") {
		if err := os.MkdirAll(localPath, 0755); err != nil {
			return fmt.Errorf("creating directory: %w", err)
		}
		localPath = filepath.Join(localPath, filepath.Base(key))
	}

	dir := filepath.Dir(localPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}

	f, err := os.Create(localPath)
	if err != nil {
		return fmt.Errorf("creating file: %w", err)
	}
	defer f.Close()

	_, err = g.downloader.Download(ctx, f, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		os.Remove(localPath)
		return fmt.Errorf("downloading file: %w", err)
	}
	return nil
}

func (g *Gateway) DownloadDirectory(ctx context.Context, bucket, prefix, localDir string) error {
	objects, err := g.ListAllObjectsForPrefix(ctx, bucket, prefix)
	if err != nil {
		return err
	}

	for _, obj := range objects {
		relPath := strings.TrimPrefix(obj.Key, prefix)
		if relPath == "" {
			continue
		}
		localPath := filepath.Join(localDir, filepath.FromSlash(relPath))

		dir := filepath.Dir(localPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("creating directory: %w", err)
		}

		if err := g.DownloadFile(ctx, bucket, obj.Key, localPath); err != nil {
			return err
		}
	}
	return nil
}

// DownloadFileWithProgress downloads a file with byte-level progress tracking.
// The passed context can be cancelled to abort the download.
func (g *Gateway) DownloadFileWithProgress(ctx context.Context, bucket, key, localPath string, progress *DownloadProgress) error {
	// If localPath is a directory, append the filename
	if info, err := os.Stat(localPath); err == nil && info.IsDir() {
		localPath = filepath.Join(localPath, filepath.Base(key))
	} else if strings.HasSuffix(localPath, string(os.PathSeparator)) || strings.HasSuffix(localPath, "/") {
		if err := os.MkdirAll(localPath, 0755); err != nil {
			return fmt.Errorf("creating directory: %w", err)
		}
		localPath = filepath.Join(localPath, filepath.Base(key))
	}

	dir := filepath.Dir(localPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}

	// HeadObject to get content length
	head, err := g.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err == nil && head.ContentLength != nil {
		progress.TotalBytes = *head.ContentLength
	}

	progress.StartedAt = time.Now()

	f, err := os.Create(localPath)
	if err != nil {
		return fmt.Errorf("creating file: %w", err)
	}
	defer f.Close()

	pw := &progressWriterAt{writer: f, progress: progress}

	_, err = g.downloader.Download(ctx, pw, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		f.Close()
		os.Remove(localPath)
		return fmt.Errorf("downloading file: %w", err)
	}
	return nil
}

// DownloadDirectoryWithProgress downloads a directory with per-file progress tracking.
// progressFn is called before each file starts downloading.
func (g *Gateway) DownloadDirectoryWithProgress(ctx context.Context, bucket, prefix, localDir string, progressFn func(fileIdx, totalFiles int, currentFile string, progress *DownloadProgress)) error {
	objects, err := g.ListAllObjectsForPrefix(ctx, bucket, prefix)
	if err != nil {
		return err
	}

	for i, obj := range objects {
		relPath := strings.TrimPrefix(obj.Key, prefix)
		if relPath == "" {
			continue
		}
		localPath := filepath.Join(localDir, filepath.FromSlash(relPath))

		dir := filepath.Dir(localPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("creating directory: %w", err)
		}

		progress := &DownloadProgress{}
		if progressFn != nil {
			progressFn(i, len(objects), relPath, progress)
		}

		if err := g.DownloadFileWithProgress(ctx, bucket, obj.Key, localPath, progress); err != nil {
			return err
		}
	}
	return nil
}

func (g *Gateway) DeleteFile(ctx context.Context, bucket, key string) error {
	_, err := g.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("deleting file: %w", err)
	}
	return nil
}

func (g *Gateway) DeleteDirectory(ctx context.Context, bucket, prefix string) error {
	objects, err := g.ListAllObjectsForPrefix(ctx, bucket, prefix)
	if err != nil {
		return err
	}

	// Delete in batches of 1000
	for i := 0; i < len(objects); i += 1000 {
		end := i + 1000
		if end > len(objects) {
			end = len(objects)
		}

		batch := objects[i:end]
		ids := make([]s3types.ObjectIdentifier, len(batch))
		for j, obj := range batch {
			ids[j] = s3types.ObjectIdentifier{
				Key: aws.String(obj.Key),
			}
		}

		_, err := g.client.DeleteObjects(ctx, &s3.DeleteObjectsInput{
			Bucket: aws.String(bucket),
			Delete: &s3types.Delete{
				Objects: ids,
				Quiet:   aws.Bool(true),
			},
		})
		if err != nil {
			return fmt.Errorf("deleting batch: %w", err)
		}
	}
	return nil
}

func (g *Gateway) CopyFile(ctx context.Context, srcBucket, srcKey, dstBucket, dstKey string) error {
	copySource := srcBucket + "/" + srcKey
	_, err := g.client.CopyObject(ctx, &s3.CopyObjectInput{
		Bucket:     aws.String(dstBucket),
		Key:        aws.String(dstKey),
		CopySource: aws.String(copySource),
	})
	if err != nil {
		return fmt.Errorf("copying file: %w", err)
	}
	return nil
}

func (g *Gateway) CopyDirectory(ctx context.Context, srcBucket, srcPrefix, dstBucket, dstPrefix string) error {
	objects, err := g.ListAllObjectsForPrefix(ctx, srcBucket, srcPrefix)
	if err != nil {
		return err
	}

	for _, obj := range objects {
		relKey := strings.TrimPrefix(obj.Key, srcPrefix)
		dstKey := dstPrefix + relKey
		if err := g.CopyFile(ctx, srcBucket, obj.Key, dstBucket, dstKey); err != nil {
			return err
		}
	}
	return nil
}

func (g *Gateway) MoveFile(ctx context.Context, srcBucket, srcKey, dstBucket, dstKey string) error {
	if err := g.CopyFile(ctx, srcBucket, srcKey, dstBucket, dstKey); err != nil {
		return err
	}
	return g.DeleteFile(ctx, srcBucket, srcKey)
}

func (g *Gateway) MoveDirectory(ctx context.Context, srcBucket, srcPrefix, dstBucket, dstPrefix string) error {
	objects, err := g.ListAllObjectsForPrefix(ctx, srcBucket, srcPrefix)
	if err != nil {
		return err
	}

	// Copy all
	for _, obj := range objects {
		relKey := strings.TrimPrefix(obj.Key, srcPrefix)
		dstKey := dstPrefix + relKey
		if err := g.CopyFile(ctx, srcBucket, obj.Key, dstBucket, dstKey); err != nil {
			return err
		}
	}

	// Delete all originals in batches
	for i := 0; i < len(objects); i += 1000 {
		end := i + 1000
		if end > len(objects) {
			end = len(objects)
		}

		batch := objects[i:end]
		ids := make([]s3types.ObjectIdentifier, len(batch))
		for j, obj := range batch {
			ids[j] = s3types.ObjectIdentifier{
				Key: aws.String(obj.Key),
			}
		}

		_, err := g.client.DeleteObjects(ctx, &s3.DeleteObjectsInput{
			Bucket: aws.String(srcBucket),
			Delete: &s3types.Delete{
				Objects: ids,
				Quiet:   aws.Bool(true),
			},
		})
		if err != nil {
			return fmt.Errorf("deleting source batch after move: %w", err)
		}
	}
	return nil
}

// RenameFile renames (moves) a file within the same bucket.
func (g *Gateway) RenameFile(ctx context.Context, bucket, oldKey, newKey string) error {
	return g.MoveFile(ctx, bucket, oldKey, bucket, newKey)
}

// RenameDirectory renames (moves) a directory within the same bucket.
func (g *Gateway) RenameDirectory(ctx context.Context, bucket, oldPrefix, newPrefix string) error {
	return g.MoveDirectory(ctx, bucket, oldPrefix, bucket, newPrefix)
}

