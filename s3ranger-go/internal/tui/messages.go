package tui

import (
	"time"

	s3gw "github.com/s3ranger/s3ranger-go/internal/s3"
)

// --- Bucket messages ---

type BucketsLoadedMsg struct {
	Buckets           []s3gw.BucketInfo
	ContinuationToken string
	HasMore           bool
	Append            bool
	Err               error
}

type BucketSelectedMsg struct {
	BucketName string
}

type BucketFilterTickMsg struct {
	FilterValue string
}

// --- Object messages ---

type ObjectsLoadedMsg struct {
	Files             []s3gw.ObjectInfo
	Folders           []s3gw.ObjectInfo
	ContinuationToken string
	HasMore           bool
	Append            bool
	Err               error
}

type NavigateIntoFolderMsg struct {
	Prefix string
}

type NavigateUpMsg struct{}

type RefreshObjectsMsg struct{}

// --- Modal result messages ---

type DeleteResultMsg struct {
	Err error
}

type RenameResultMsg struct {
	Err error
}

type DownloadResultMsg struct {
	Err error
}

type UploadResultMsg struct {
	Err error
}

type MultiDeleteProgressMsg struct {
	Current int
	Total   int
	Name    string
}

type MultiDeleteDoneMsg struct {
	Succeeded int
	Failed    int
	Err       error
}

type MultiDownloadProgressMsg struct {
	Current int
	Total   int
	Name    string
}

type MultiDownloadDoneMsg struct {
	Succeeded int
	Failed    int
	Err       error
}

type MoveCopyProgressMsg struct {
	Current int
	Total   int
	Name    string
}

type MoveCopyDoneMsg struct {
	Err error
}

// --- UI messages ---

type NotificationMsg struct {
	Text     string
	Duration time.Duration
	IsError  bool
}

type ClearNotificationMsg struct{}

type CloseModalMsg struct{}
