package util

import (
	"fmt"
	"path"
	"strings"
)

func FormatFileSize(size int64) string {
	if size < 0 {
		return ""
	}
	const (
		KB = 1024
		MB = 1024 * KB
		GB = 1024 * MB
		TB = 1024 * GB
	)
	switch {
	case size < KB:
		return fmt.Sprintf("%d B", size)
	case size < MB:
		return fmt.Sprintf("%.1f KB", float64(size)/float64(KB))
	case size < GB:
		return fmt.Sprintf("%.1f MB", float64(size)/float64(MB))
	case size < TB:
		return fmt.Sprintf("%.1f GB", float64(size)/float64(GB))
	default:
		return fmt.Sprintf("%.1f TB", float64(size)/float64(TB))
	}
}

type S3URI struct {
	Bucket string
	Key    string
}

func ParseS3URI(uri string) (S3URI, error) {
	if !strings.HasPrefix(uri, "s3://") {
		return S3URI{}, fmt.Errorf("invalid S3 URI: must start with s3://")
	}
	rest := uri[5:]
	parts := strings.SplitN(rest, "/", 2)
	result := S3URI{Bucket: parts[0]}
	if len(parts) > 1 {
		result.Key = parts[1]
	}
	return result, nil
}

func BuildS3URI(bucket, key string) string {
	if key == "" {
		return fmt.Sprintf("s3://%s", bucket)
	}
	return fmt.Sprintf("s3://%s/%s", bucket, key)
}

func CompressPath(p string, maxLen int) string {
	if len(p) <= maxLen || maxLen < 5 {
		return p
	}
	return "..." + p[len(p)-(maxLen-3):]
}

func FileExtension(key string) string {
	base := path.Base(key)
	if strings.HasSuffix(key, "/") {
		return "dir"
	}
	idx := strings.LastIndex(base, ".")
	if idx < 0 || idx == len(base)-1 {
		return ""
	}
	return strings.ToLower(base[idx+1:])
}

func IsFolder(key string) bool {
	return strings.HasSuffix(key, "/")
}

func ParentPrefix(prefix string) string {
	if prefix == "" {
		return ""
	}
	trimmed := strings.TrimSuffix(prefix, "/")
	idx := strings.LastIndex(trimmed, "/")
	if idx < 0 {
		return ""
	}
	return trimmed[:idx+1]
}

func ObjectName(key string) string {
	trimmed := strings.TrimSuffix(key, "/")
	return path.Base(trimmed)
}
