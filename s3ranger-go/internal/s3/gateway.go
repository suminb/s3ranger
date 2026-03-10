package s3

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
)

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

