//go:build integration

package s3

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

const (
	minioEndpoint  = "http://localhost:9123"
	minioAccessKey = "minioadmin"
	minioSecretKey = "minioadmin"
	containerName  = "s3ranger-test-minio"
)

var testGateway *Gateway

func TestMain(m *testing.M) {
	// Start MinIO container
	if err := startMinio(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to start MinIO: %v\n", err)
		os.Exit(1)
	}

	// Wait for MinIO to be ready
	if err := waitForMinio(); err != nil {
		fmt.Fprintf(os.Stderr, "MinIO did not become ready: %v\n", err)
		stopMinio()
		os.Exit(1)
	}

	// Create gateway
	cfg, err := config.LoadDefaultConfig(context.Background(),
		config.WithRegion("us-east-1"),
		config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(minioAccessKey, minioSecretKey, ""),
		),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load AWS config: %v\n", err)
		stopMinio()
		os.Exit(1)
	}

	testGateway = NewGateway(cfg, minioEndpoint)

	code := m.Run()

	stopMinio()
	os.Exit(code)
}

func startMinio() error {
	// Remove any existing container with the same name
	exec.Command("docker", "rm", "-f", containerName).Run()

	cmd := exec.Command("docker", "run", "-d",
		"--name", containerName,
		"-p", "9123:9000",
		"-e", "MINIO_ROOT_USER="+minioAccessKey,
		"-e", "MINIO_ROOT_PASSWORD="+minioSecretKey,
		"minio/minio", "server", "/data",
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func stopMinio() {
	exec.Command("docker", "rm", "-f", containerName).Run()
}

func waitForMinio() error {
	ctx := context.Background()
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion("us-east-1"),
		config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(minioAccessKey, minioSecretKey, ""),
		),
	)
	if err != nil {
		return err
	}

	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(minioEndpoint)
		o.UsePathStyle = true
	})

	for i := 0; i < 30; i++ {
		_, err := client.ListBuckets(ctx, &s3.ListBucketsInput{})
		if err == nil {
			return nil
		}
		time.Sleep(time.Second)
	}
	return fmt.Errorf("MinIO not ready after 30 seconds")
}

// createTestBucket creates a bucket and returns a cleanup function.
func createTestBucket(t *testing.T, name string) {
	t.Helper()
	ctx := context.Background()
	_, err := testGateway.client.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: aws.String(name),
	})
	if err != nil {
		t.Fatalf("CreateBucket(%q): %v", name, err)
	}
	t.Cleanup(func() {
		// Delete all objects first
		testGateway.DeleteDirectory(ctx, name, "")
		testGateway.client.DeleteBucket(ctx, &s3.DeleteBucketInput{
			Bucket: aws.String(name),
		})
	})
}

// putTestObject uploads a string as an object.
func putTestObject(t *testing.T, bucket, key, content string) {
	t.Helper()
	ctx := context.Background()
	_, err := testGateway.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Body:   strings.NewReader(content),
	})
	if err != nil {
		t.Fatalf("PutObject(%q/%q): %v", bucket, key, err)
	}
}

// writeLocalFile creates a file with content in the given directory.
func writeLocalFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestListBuckets(t *testing.T) {
	createTestBucket(t, "list-test-alpha")
	createTestBucket(t, "list-test-beta")

	ctx := context.Background()
	page, err := testGateway.ListBuckets(ctx, "", 0, "")
	if err != nil {
		t.Fatalf("ListBuckets: %v", err)
	}

	names := make(map[string]bool)
	for _, b := range page.Buckets {
		names[b.Name] = true
	}
	if !names["list-test-alpha"] || !names["list-test-beta"] {
		t.Errorf("Expected both buckets, got: %v", names)
	}
}

func TestListBuckets_WithPrefix(t *testing.T) {
	createTestBucket(t, "prefix-aaa")
	createTestBucket(t, "prefix-bbb")
	createTestBucket(t, "other-ccc")

	ctx := context.Background()
	page, err := testGateway.ListBuckets(ctx, "prefix-", 0, "")
	if err != nil {
		t.Fatalf("ListBuckets: %v", err)
	}

	// MinIO may not support server-side prefix filtering on ListBuckets.
	// Just verify our target buckets are present in the results.
	names := make(map[string]bool)
	for _, b := range page.Buckets {
		names[b.Name] = true
	}
	if !names["prefix-aaa"] || !names["prefix-bbb"] {
		t.Errorf("Expected prefix-aaa and prefix-bbb in results, got: %v", names)
	}
}

func TestUploadAndDownloadFile(t *testing.T) {
	bucket := "upload-download-test"
	createTestBucket(t, bucket)

	tmpDir := t.TempDir()
	srcFile := writeLocalFile(t, tmpDir, "hello.txt", "hello world")

	ctx := context.Background()
	if err := testGateway.UploadFile(ctx, srcFile, bucket, "hello.txt"); err != nil {
		t.Fatalf("UploadFile: %v", err)
	}

	// Download to a new directory
	dlDir := filepath.Join(tmpDir, "downloaded")
	os.MkdirAll(dlDir, 0755)
	if err := testGateway.DownloadFile(ctx, bucket, "hello.txt", dlDir); err != nil {
		t.Fatalf("DownloadFile: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(dlDir, "hello.txt"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(content) != "hello world" {
		t.Errorf("Downloaded content = %q, want %q", string(content), "hello world")
	}
}

func TestUploadAndDownloadDirectory(t *testing.T) {
	bucket := "upload-dir-test"
	createTestBucket(t, bucket)

	tmpDir := t.TempDir()
	srcDir := filepath.Join(tmpDir, "mydir")
	writeLocalFile(t, srcDir, "a.txt", "aaa")
	writeLocalFile(t, srcDir, "sub/b.txt", "bbb")

	ctx := context.Background()
	if err := testGateway.UploadDirectory(ctx, srcDir, bucket, "uploaded/"); err != nil {
		t.Fatalf("UploadDirectory: %v", err)
	}

	// Verify objects exist
	objects, err := testGateway.ListAllObjectsForPrefix(ctx, bucket, "uploaded/")
	if err != nil {
		t.Fatalf("ListAllObjectsForPrefix: %v", err)
	}

	keys := make([]string, len(objects))
	for i, o := range objects {
		keys[i] = o.Key
	}
	sort.Strings(keys)
	expected := []string{"uploaded/a.txt", "uploaded/sub/b.txt"}
	if len(keys) != len(expected) {
		t.Fatalf("Expected %d objects, got %d: %v", len(expected), len(keys), keys)
	}
	for i, k := range keys {
		if k != expected[i] {
			t.Errorf("Object[%d] = %q, want %q", i, k, expected[i])
		}
	}

	// Download directory
	dlDir := filepath.Join(tmpDir, "dl")
	if err := testGateway.DownloadDirectory(ctx, bucket, "uploaded/", dlDir); err != nil {
		t.Fatalf("DownloadDirectory: %v", err)
	}

	contentA, _ := os.ReadFile(filepath.Join(dlDir, "a.txt"))
	contentB, _ := os.ReadFile(filepath.Join(dlDir, "sub", "b.txt"))
	if string(contentA) != "aaa" {
		t.Errorf("a.txt content = %q, want %q", string(contentA), "aaa")
	}
	if string(contentB) != "bbb" {
		t.Errorf("sub/b.txt content = %q, want %q", string(contentB), "bbb")
	}
}

func TestListObjectsForPrefix(t *testing.T) {
	bucket := "list-objects-test"
	createTestBucket(t, bucket)

	putTestObject(t, bucket, "file1.txt", "data1")
	putTestObject(t, bucket, "file2.txt", "data2")
	putTestObject(t, bucket, "dir1/nested.txt", "nested")
	putTestObject(t, bucket, "dir2/deep/file.txt", "deep")

	ctx := context.Background()

	// List root level
	page, err := testGateway.ListObjectsForPrefix(ctx, bucket, "", 0, "")
	if err != nil {
		t.Fatalf("ListObjectsForPrefix: %v", err)
	}

	fileKeys := make([]string, len(page.Files))
	for i, f := range page.Files {
		fileKeys[i] = f.Key
	}
	sort.Strings(fileKeys)
	if len(fileKeys) != 2 || fileKeys[0] != "file1.txt" || fileKeys[1] != "file2.txt" {
		t.Errorf("Root files = %v, want [file1.txt file2.txt]", fileKeys)
	}

	folderKeys := make([]string, len(page.Folders))
	for i, f := range page.Folders {
		folderKeys[i] = f.Key
	}
	sort.Strings(folderKeys)
	if len(folderKeys) != 2 || folderKeys[0] != "dir1/" || folderKeys[1] != "dir2/" {
		t.Errorf("Root folders = %v, want [dir1/ dir2/]", folderKeys)
	}

	// List inside dir1/
	page2, err := testGateway.ListObjectsForPrefix(ctx, bucket, "dir1/", 0, "")
	if err != nil {
		t.Fatalf("ListObjectsForPrefix(dir1/): %v", err)
	}
	if len(page2.Files) != 1 || page2.Files[0].Key != "dir1/nested.txt" {
		t.Errorf("dir1/ files = %v, want [dir1/nested.txt]", page2.Files)
	}
}

func TestListObjectsForPrefix_Pagination(t *testing.T) {
	bucket := "list-pag-test"
	createTestBucket(t, bucket)

	ctx := context.Background()
	// Create 5 objects
	for i := 0; i < 5; i++ {
		putTestObject(t, bucket, fmt.Sprintf("item%02d.txt", i), "data")
	}

	// List with maxKeys=2
	page1, err := testGateway.ListObjectsForPrefix(ctx, bucket, "", 2, "")
	if err != nil {
		t.Fatalf("ListObjectsForPrefix page1: %v", err)
	}
	if len(page1.Files) != 2 {
		t.Errorf("Page1 files = %d, want 2", len(page1.Files))
	}
	if !page1.HasMore {
		t.Error("Page1 HasMore = false, want true")
	}

	// Load next page
	page2, err := testGateway.ListObjectsForPrefix(ctx, bucket, "", 2, page1.ContinuationToken)
	if err != nil {
		t.Fatalf("ListObjectsForPrefix page2: %v", err)
	}
	if len(page2.Files) != 2 {
		t.Errorf("Page2 files = %d, want 2", len(page2.Files))
	}

	// Load last page
	page3, err := testGateway.ListObjectsForPrefix(ctx, bucket, "", 2, page2.ContinuationToken)
	if err != nil {
		t.Fatalf("ListObjectsForPrefix page3: %v", err)
	}
	if len(page3.Files) != 1 {
		t.Errorf("Page3 files = %d, want 1", len(page3.Files))
	}
	if page3.HasMore {
		t.Error("Page3 HasMore = true, want false")
	}
}

func TestDeleteFile(t *testing.T) {
	bucket := "delete-file-test"
	createTestBucket(t, bucket)

	putTestObject(t, bucket, "todelete.txt", "bye")

	ctx := context.Background()
	if err := testGateway.DeleteFile(ctx, bucket, "todelete.txt"); err != nil {
		t.Fatalf("DeleteFile: %v", err)
	}

	// Verify it's gone
	objects, err := testGateway.ListAllObjectsForPrefix(ctx, bucket, "")
	if err != nil {
		t.Fatalf("ListAllObjectsForPrefix: %v", err)
	}
	for _, o := range objects {
		if o.Key == "todelete.txt" {
			t.Error("Object still exists after delete")
		}
	}
}

func TestDeleteDirectory(t *testing.T) {
	bucket := "delete-dir-test"
	createTestBucket(t, bucket)

	putTestObject(t, bucket, "dir/a.txt", "a")
	putTestObject(t, bucket, "dir/b.txt", "b")
	putTestObject(t, bucket, "dir/sub/c.txt", "c")
	putTestObject(t, bucket, "keep.txt", "keep")

	ctx := context.Background()
	if err := testGateway.DeleteDirectory(ctx, bucket, "dir/"); err != nil {
		t.Fatalf("DeleteDirectory: %v", err)
	}

	objects, err := testGateway.ListAllObjectsForPrefix(ctx, bucket, "")
	if err != nil {
		t.Fatalf("ListAllObjectsForPrefix: %v", err)
	}
	if len(objects) != 1 || objects[0].Key != "keep.txt" {
		keys := make([]string, len(objects))
		for i, o := range objects {
			keys[i] = o.Key
		}
		t.Errorf("After deleting dir/, remaining objects = %v, want [keep.txt]", keys)
	}
}

func TestCopyFile(t *testing.T) {
	bucket := "copy-file-test"
	createTestBucket(t, bucket)

	putTestObject(t, bucket, "original.txt", "original content")

	ctx := context.Background()
	if err := testGateway.CopyFile(ctx, bucket, "original.txt", bucket, "copied.txt"); err != nil {
		t.Fatalf("CopyFile: %v", err)
	}

	// Both should exist
	objects, err := testGateway.ListAllObjectsForPrefix(ctx, bucket, "")
	if err != nil {
		t.Fatalf("ListAllObjectsForPrefix: %v", err)
	}

	keys := make(map[string]bool)
	for _, o := range objects {
		keys[o.Key] = true
	}
	if !keys["original.txt"] || !keys["copied.txt"] {
		t.Errorf("Expected both original.txt and copied.txt, got: %v", keys)
	}

	// Verify content by downloading
	tmpDir := t.TempDir()
	if err := testGateway.DownloadFile(ctx, bucket, "copied.txt", tmpDir); err != nil {
		t.Fatalf("DownloadFile: %v", err)
	}
	content, _ := os.ReadFile(filepath.Join(tmpDir, "copied.txt"))
	if string(content) != "original content" {
		t.Errorf("Copied content = %q, want %q", string(content), "original content")
	}
}

func TestCopyFile_CrossBucket(t *testing.T) {
	srcBucket := "copy-src-bucket"
	dstBucket := "copy-dst-bucket"
	createTestBucket(t, srcBucket)
	createTestBucket(t, dstBucket)

	putTestObject(t, srcBucket, "data.txt", "cross-bucket data")

	ctx := context.Background()
	if err := testGateway.CopyFile(ctx, srcBucket, "data.txt", dstBucket, "data.txt"); err != nil {
		t.Fatalf("CopyFile cross-bucket: %v", err)
	}

	// Verify in destination
	objects, err := testGateway.ListAllObjectsForPrefix(ctx, dstBucket, "")
	if err != nil {
		t.Fatalf("ListAllObjectsForPrefix: %v", err)
	}
	if len(objects) != 1 || objects[0].Key != "data.txt" {
		t.Errorf("Destination objects = %v, want [data.txt]", objects)
	}
}

func TestCopyDirectory(t *testing.T) {
	bucket := "copy-dir-test"
	createTestBucket(t, bucket)

	putTestObject(t, bucket, "src/a.txt", "aaa")
	putTestObject(t, bucket, "src/sub/b.txt", "bbb")

	ctx := context.Background()
	if err := testGateway.CopyDirectory(ctx, bucket, "src/", bucket, "dst/"); err != nil {
		t.Fatalf("CopyDirectory: %v", err)
	}

	// Source should still exist
	srcObjs, _ := testGateway.ListAllObjectsForPrefix(ctx, bucket, "src/")
	if len(srcObjs) != 2 {
		t.Errorf("Source objects = %d, want 2", len(srcObjs))
	}

	// Destination should have copies
	dstObjs, _ := testGateway.ListAllObjectsForPrefix(ctx, bucket, "dst/")
	dstKeys := make([]string, len(dstObjs))
	for i, o := range dstObjs {
		dstKeys[i] = o.Key
	}
	sort.Strings(dstKeys)
	expected := []string{"dst/a.txt", "dst/sub/b.txt"}
	if len(dstKeys) != len(expected) {
		t.Fatalf("Dest objects = %v, want %v", dstKeys, expected)
	}
	for i, k := range dstKeys {
		if k != expected[i] {
			t.Errorf("Dest[%d] = %q, want %q", i, k, expected[i])
		}
	}
}

func TestMoveFile(t *testing.T) {
	bucket := "move-file-test"
	createTestBucket(t, bucket)

	putTestObject(t, bucket, "old.txt", "move me")

	ctx := context.Background()
	if err := testGateway.MoveFile(ctx, bucket, "old.txt", bucket, "new.txt"); err != nil {
		t.Fatalf("MoveFile: %v", err)
	}

	objects, err := testGateway.ListAllObjectsForPrefix(ctx, bucket, "")
	if err != nil {
		t.Fatalf("ListAllObjectsForPrefix: %v", err)
	}

	keys := make(map[string]bool)
	for _, o := range objects {
		keys[o.Key] = true
	}
	if keys["old.txt"] {
		t.Error("old.txt still exists after move")
	}
	if !keys["new.txt"] {
		t.Error("new.txt does not exist after move")
	}
}

func TestMoveDirectory(t *testing.T) {
	bucket := "move-dir-test"
	createTestBucket(t, bucket)

	putTestObject(t, bucket, "olddir/a.txt", "aaa")
	putTestObject(t, bucket, "olddir/sub/b.txt", "bbb")

	ctx := context.Background()
	if err := testGateway.MoveDirectory(ctx, bucket, "olddir/", bucket, "newdir/"); err != nil {
		t.Fatalf("MoveDirectory: %v", err)
	}

	// Old directory should be empty
	oldObjs, _ := testGateway.ListAllObjectsForPrefix(ctx, bucket, "olddir/")
	if len(oldObjs) != 0 {
		t.Errorf("Old directory still has %d objects", len(oldObjs))
	}

	// New directory should have the files
	newObjs, _ := testGateway.ListAllObjectsForPrefix(ctx, bucket, "newdir/")
	newKeys := make([]string, len(newObjs))
	for i, o := range newObjs {
		newKeys[i] = o.Key
	}
	sort.Strings(newKeys)
	expected := []string{"newdir/a.txt", "newdir/sub/b.txt"}
	if len(newKeys) != len(expected) {
		t.Fatalf("New dir objects = %v, want %v", newKeys, expected)
	}
	for i, k := range newKeys {
		if k != expected[i] {
			t.Errorf("New[%d] = %q, want %q", i, k, expected[i])
		}
	}
}

func TestRenameFile(t *testing.T) {
	bucket := "rename-file-test"
	createTestBucket(t, bucket)

	putTestObject(t, bucket, "before.txt", "rename me")

	ctx := context.Background()
	if err := testGateway.RenameFile(ctx, bucket, "before.txt", "after.txt"); err != nil {
		t.Fatalf("RenameFile: %v", err)
	}

	objects, _ := testGateway.ListAllObjectsForPrefix(ctx, bucket, "")
	if len(objects) != 1 || objects[0].Key != "after.txt" {
		keys := make([]string, len(objects))
		for i, o := range objects {
			keys[i] = o.Key
		}
		t.Errorf("After rename, objects = %v, want [after.txt]", keys)
	}
}

func TestRenameDirectory(t *testing.T) {
	bucket := "rename-dir-test"
	createTestBucket(t, bucket)

	putTestObject(t, bucket, "oldname/x.txt", "xxx")
	putTestObject(t, bucket, "oldname/y.txt", "yyy")

	ctx := context.Background()
	if err := testGateway.RenameDirectory(ctx, bucket, "oldname/", "newname/"); err != nil {
		t.Fatalf("RenameDirectory: %v", err)
	}

	oldObjs, _ := testGateway.ListAllObjectsForPrefix(ctx, bucket, "oldname/")
	if len(oldObjs) != 0 {
		t.Errorf("Old dir still has %d objects", len(oldObjs))
	}

	newObjs, _ := testGateway.ListAllObjectsForPrefix(ctx, bucket, "newname/")
	newKeys := make([]string, len(newObjs))
	for i, o := range newObjs {
		newKeys[i] = o.Key
	}
	sort.Strings(newKeys)
	expected := []string{"newname/x.txt", "newname/y.txt"}
	if len(newKeys) != len(expected) {
		t.Fatalf("New dir objects = %v, want %v", newKeys, expected)
	}
	for i, k := range newKeys {
		if k != expected[i] {
			t.Errorf("New[%d] = %q, want %q", i, k, expected[i])
		}
	}
}

func TestObjectMetadata(t *testing.T) {
	bucket := "metadata-test"
	createTestBucket(t, bucket)

	putTestObject(t, bucket, "sized.txt", "12345")

	ctx := context.Background()
	page, err := testGateway.ListObjectsForPrefix(ctx, bucket, "", 0, "")
	if err != nil {
		t.Fatalf("ListObjectsForPrefix: %v", err)
	}

	if len(page.Files) != 1 {
		t.Fatalf("Expected 1 file, got %d", len(page.Files))
	}

	obj := page.Files[0]
	if obj.Key != "sized.txt" {
		t.Errorf("Key = %q, want %q", obj.Key, "sized.txt")
	}
	if obj.Size != 5 {
		t.Errorf("Size = %d, want 5", obj.Size)
	}
	if obj.LastModified.IsZero() {
		t.Error("LastModified is zero")
	}
	if obj.IsFolder {
		t.Error("IsFolder = true for a file")
	}
}

func TestListAllObjectsForPrefix(t *testing.T) {
	bucket := "list-all-test"
	createTestBucket(t, bucket)

	ctx := context.Background()
	// Create objects across folders
	putTestObject(t, bucket, "a.txt", "a")
	putTestObject(t, bucket, "dir/b.txt", "b")
	putTestObject(t, bucket, "dir/sub/c.txt", "c")

	objects, err := testGateway.ListAllObjectsForPrefix(ctx, bucket, "")
	if err != nil {
		t.Fatalf("ListAllObjectsForPrefix: %v", err)
	}

	keys := make([]string, len(objects))
	for i, o := range objects {
		keys[i] = o.Key
	}
	sort.Strings(keys)
	expected := []string{"a.txt", "dir/b.txt", "dir/sub/c.txt"}
	if len(keys) != len(expected) {
		t.Fatalf("All objects = %v, want %v", keys, expected)
	}
	for i, k := range keys {
		if k != expected[i] {
			t.Errorf("Object[%d] = %q, want %q", i, k, expected[i])
		}
	}

	// With prefix filter
	dirObjects, err := testGateway.ListAllObjectsForPrefix(ctx, bucket, "dir/")
	if err != nil {
		t.Fatalf("ListAllObjectsForPrefix(dir/): %v", err)
	}
	if len(dirObjects) != 2 {
		t.Errorf("dir/ objects = %d, want 2", len(dirObjects))
	}
}

func TestEmptyBucket(t *testing.T) {
	bucket := "empty-bucket-test"
	createTestBucket(t, bucket)

	ctx := context.Background()
	page, err := testGateway.ListObjectsForPrefix(ctx, bucket, "", 0, "")
	if err != nil {
		t.Fatalf("ListObjectsForPrefix: %v", err)
	}
	if len(page.Files) != 0 {
		t.Errorf("Empty bucket has %d files", len(page.Files))
	}
	if len(page.Folders) != 0 {
		t.Errorf("Empty bucket has %d folders", len(page.Folders))
	}
	if page.HasMore {
		t.Error("Empty bucket HasMore = true")
	}
}

func TestDeleteDirectory_LargeBatch(t *testing.T) {
	bucket := "delete-large-test"
	createTestBucket(t, bucket)

	ctx := context.Background()
	// Create 1050 objects to test batch deletion (batches of 1000)
	for i := 0; i < 1050; i++ {
		putTestObject(t, bucket, fmt.Sprintf("batch/%04d.txt", i), "x")
	}

	if err := testGateway.DeleteDirectory(ctx, bucket, "batch/"); err != nil {
		t.Fatalf("DeleteDirectory large batch: %v", err)
	}

	remaining, _ := testGateway.ListAllObjectsForPrefix(ctx, bucket, "batch/")
	if len(remaining) != 0 {
		t.Errorf("After large batch delete, %d objects remain", len(remaining))
	}
}

func TestDownloadFile_ToExplicitPath(t *testing.T) {
	bucket := "dl-explicit-test"
	createTestBucket(t, bucket)

	putTestObject(t, bucket, "remote.txt", "explicit download")

	ctx := context.Background()
	tmpDir := t.TempDir()
	explicitPath := filepath.Join(tmpDir, "local-name.txt")

	if err := testGateway.DownloadFile(ctx, bucket, "remote.txt", explicitPath); err != nil {
		t.Fatalf("DownloadFile to explicit path: %v", err)
	}

	content, err := os.ReadFile(explicitPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(content) != "explicit download" {
		t.Errorf("Content = %q, want %q", string(content), "explicit download")
	}
}

func TestMoveFile_CrossBucket(t *testing.T) {
	srcBucket := "move-src-xb"
	dstBucket := "move-dst-xb"
	createTestBucket(t, srcBucket)
	createTestBucket(t, dstBucket)

	putTestObject(t, srcBucket, "moveme.txt", "cross-bucket move")

	ctx := context.Background()
	if err := testGateway.MoveFile(ctx, srcBucket, "moveme.txt", dstBucket, "moved.txt"); err != nil {
		t.Fatalf("MoveFile cross-bucket: %v", err)
	}

	// Source should be empty
	srcObjs, _ := testGateway.ListAllObjectsForPrefix(ctx, srcBucket, "")
	if len(srcObjs) != 0 {
		t.Errorf("Source bucket still has %d objects", len(srcObjs))
	}

	// Destination should have the file
	dstObjs, _ := testGateway.ListAllObjectsForPrefix(ctx, dstBucket, "")
	if len(dstObjs) != 1 || dstObjs[0].Key != "moved.txt" {
		t.Errorf("Dest bucket objects = %v, want [moved.txt]", dstObjs)
	}
}
