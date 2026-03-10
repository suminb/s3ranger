package util

import (
	"testing"
)

func TestFormatFileSize(t *testing.T) {
	tests := []struct {
		size int64
		want string
	}{
		{-1, ""},
		{0, "0 B"},
		{1, "1 B"},
		{512, "512 B"},
		{1023, "1023 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1048576, "1.0 MB"},
		{1572864, "1.5 MB"},
		{1073741824, "1.0 GB"},
		{1610612736, "1.5 GB"},
		{1099511627776, "1.0 TB"},
		{1649267441664, "1.5 TB"},
	}

	for _, tt := range tests {
		got := FormatFileSize(tt.size)
		if got != tt.want {
			t.Errorf("FormatFileSize(%d) = %q, want %q", tt.size, got, tt.want)
		}
	}
}

func TestParseS3URI(t *testing.T) {
	tests := []struct {
		uri     string
		want    S3URI
		wantErr bool
	}{
		{"s3://mybucket", S3URI{Bucket: "mybucket"}, false},
		{"s3://mybucket/", S3URI{Bucket: "mybucket", Key: ""}, false},
		{"s3://mybucket/path/to/file.txt", S3URI{Bucket: "mybucket", Key: "path/to/file.txt"}, false},
		{"s3://mybucket/folder/", S3URI{Bucket: "mybucket", Key: "folder/"}, false},
		{"s3://", S3URI{Bucket: ""}, false},
		{"http://mybucket/key", S3URI{}, true},
		{"mybucket/key", S3URI{}, true},
		{"", S3URI{}, true},
	}

	for _, tt := range tests {
		got, err := ParseS3URI(tt.uri)
		if (err != nil) != tt.wantErr {
			t.Errorf("ParseS3URI(%q) error = %v, wantErr %v", tt.uri, err, tt.wantErr)
			continue
		}
		if !tt.wantErr && got != tt.want {
			t.Errorf("ParseS3URI(%q) = %+v, want %+v", tt.uri, got, tt.want)
		}
	}
}

func TestBuildS3URI(t *testing.T) {
	tests := []struct {
		bucket string
		key    string
		want   string
	}{
		{"mybucket", "", "s3://mybucket"},
		{"mybucket", "file.txt", "s3://mybucket/file.txt"},
		{"mybucket", "path/to/file.txt", "s3://mybucket/path/to/file.txt"},
		{"mybucket", "folder/", "s3://mybucket/folder/"},
	}

	for _, tt := range tests {
		got := BuildS3URI(tt.bucket, tt.key)
		if got != tt.want {
			t.Errorf("BuildS3URI(%q, %q) = %q, want %q", tt.bucket, tt.key, got, tt.want)
		}
	}
}

func TestParseAndBuildRoundTrip(t *testing.T) {
	uris := []string{
		"s3://mybucket/path/to/file.txt",
		"s3://mybucket/folder/",
	}
	for _, uri := range uris {
		parsed, err := ParseS3URI(uri)
		if err != nil {
			t.Fatalf("ParseS3URI(%q) unexpected error: %v", uri, err)
		}
		rebuilt := BuildS3URI(parsed.Bucket, parsed.Key)
		if rebuilt != uri {
			t.Errorf("Round-trip failed: %q -> %+v -> %q", uri, parsed, rebuilt)
		}
	}
}

func TestCompressPath(t *testing.T) {
	tests := []struct {
		path   string
		maxLen int
		want   string
	}{
		{"/home/user/documents/file.txt", 30, "/home/user/documents/file.txt"},
		{"/home/user/documents/file.txt", 20, "...ocuments/file.txt"},
		{"/home/user/documents/file.txt", 10, "...ile.txt"},
		{"short", 10, "short"},
		{"short", 3, "short"}, // maxLen < 5, return as-is
		{"short", 4, "short"}, // maxLen < 5, return as-is
		{"abcde", 5, "abcde"},
		{"abcdef", 5, "...ef"},
	}

	for _, tt := range tests {
		got := CompressPath(tt.path, tt.maxLen)
		if got != tt.want {
			t.Errorf("CompressPath(%q, %d) = %q, want %q", tt.path, tt.maxLen, got, tt.want)
		}
	}
}

func TestFileExtension(t *testing.T) {
	tests := []struct {
		key  string
		want string
	}{
		{"file.txt", "txt"},
		{"file.TXT", "txt"},
		{"archive.tar.gz", "gz"},
		{"noext", ""},
		{"folder/", "dir"},
		{"path/to/file.jpg", "jpg"},
		{"path/to/folder/", "dir"},
		{".hidden", "hidden"},
		{"file.", ""},
		{"path/to/.gitignore", "gitignore"},
	}

	for _, tt := range tests {
		got := FileExtension(tt.key)
		if got != tt.want {
			t.Errorf("FileExtension(%q) = %q, want %q", tt.key, got, tt.want)
		}
	}
}

func TestIsFolder(t *testing.T) {
	tests := []struct {
		key  string
		want bool
	}{
		{"folder/", true},
		{"path/to/folder/", true},
		{"file.txt", false},
		{"path/to/file.txt", false},
		{"", false},
	}

	for _, tt := range tests {
		got := IsFolder(tt.key)
		if got != tt.want {
			t.Errorf("IsFolder(%q) = %v, want %v", tt.key, got, tt.want)
		}
	}
}

func TestParentPrefix(t *testing.T) {
	tests := []struct {
		prefix string
		want   string
	}{
		{"", ""},
		{"folder/", ""},
		{"path/to/folder/", "path/to/"},
		{"a/b/c/", "a/b/"},
		{"a/b/c", "a/b/"},
		{"toplevel", ""},
	}

	for _, tt := range tests {
		got := ParentPrefix(tt.prefix)
		if got != tt.want {
			t.Errorf("ParentPrefix(%q) = %q, want %q", tt.prefix, got, tt.want)
		}
	}
}

func TestObjectName(t *testing.T) {
	tests := []struct {
		key  string
		want string
	}{
		{"file.txt", "file.txt"},
		{"path/to/file.txt", "file.txt"},
		{"folder/", "folder"},
		{"path/to/folder/", "folder"},
		{"toplevel", "toplevel"},
	}

	for _, tt := range tests {
		got := ObjectName(tt.key)
		if got != tt.want {
			t.Errorf("ObjectName(%q) = %q, want %q", tt.key, got, tt.want)
		}
	}
}
