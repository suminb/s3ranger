package components

import (
	"testing"

	s3gw "github.com/s3ranger/s3ranger-go/internal/s3"
)

func makeBuckets(names ...string) []s3gw.BucketInfo {
	buckets := make([]s3gw.BucketInfo, len(names))
	for i, n := range names {
		buckets[i] = s3gw.BucketInfo{Name: n}
	}
	return buckets
}

func TestApplyFilter_NoFilter(t *testing.T) {
	m := &BucketListModel{
		buckets: makeBuckets("alpha", "beta", "gamma"),
	}
	m.applyFilter()

	if len(m.filteredBuckets) != 3 {
		t.Errorf("No filter: filteredBuckets = %d, want 3", len(m.filteredBuckets))
	}
}

func TestApplyFilter_Substring(t *testing.T) {
	m := &BucketListModel{
		buckets:     makeBuckets("prod-logs", "staging-logs", "prod-data", "dev-tools"),
		filterValue: "prod",
	}
	m.applyFilter()

	if len(m.filteredBuckets) != 2 {
		t.Errorf("Filter 'prod': got %d buckets, want 2", len(m.filteredBuckets))
	}
	for _, b := range m.filteredBuckets {
		if b.Name != "prod-logs" && b.Name != "prod-data" {
			t.Errorf("Unexpected bucket in filtered results: %q", b.Name)
		}
	}
}

func TestApplyFilter_CaseInsensitive(t *testing.T) {
	m := &BucketListModel{
		buckets:     makeBuckets("MyBucket", "mybucket", "OTHER"),
		filterValue: "mybucket",
	}
	m.applyFilter()

	if len(m.filteredBuckets) != 2 {
		t.Errorf("Case-insensitive filter: got %d buckets, want 2", len(m.filteredBuckets))
	}
}

func TestApplyFilter_NoMatch(t *testing.T) {
	m := &BucketListModel{
		buckets:     makeBuckets("alpha", "beta"),
		filterValue: "zzz",
		cursor:      1,
	}
	m.applyFilter()

	if len(m.filteredBuckets) != 0 {
		t.Errorf("No match: got %d buckets, want 0", len(m.filteredBuckets))
	}
	if m.cursor != 0 {
		t.Errorf("Cursor should reset to 0 when no results, got %d", m.cursor)
	}
}

func TestApplyFilter_CursorClamp(t *testing.T) {
	m := &BucketListModel{
		buckets:     makeBuckets("aaa", "bbb", "ccc", "ddd"),
		filterValue: "aaa",
		cursor:      3, // was pointing at "ddd"
	}
	m.applyFilter()

	// Only 1 result, cursor should clamp to 0
	if m.cursor != 0 {
		t.Errorf("Cursor should clamp to 0, got %d", m.cursor)
	}
}

func TestApplyFilter_CursorStaysValid(t *testing.T) {
	m := &BucketListModel{
		buckets:     makeBuckets("ab", "ac", "ad", "bc"),
		filterValue: "a",
		cursor:      2, // within range of 3 filtered results
	}
	m.applyFilter()

	if len(m.filteredBuckets) != 3 {
		t.Fatalf("Expected 3 filtered, got %d", len(m.filteredBuckets))
	}
	// Cursor 2 is valid (3 results: indices 0,1,2)
	if m.cursor != 2 {
		t.Errorf("Cursor should stay at 2, got %d", m.cursor)
	}
}

func TestApplyFilter_EmptyBucketList(t *testing.T) {
	m := &BucketListModel{
		buckets:     nil,
		filterValue: "test",
	}
	m.applyFilter()

	if len(m.filteredBuckets) != 0 {
		t.Errorf("Empty bucket list: got %d filtered, want 0", len(m.filteredBuckets))
	}
}

func TestApplyFilter_ClearFilter(t *testing.T) {
	m := &BucketListModel{
		buckets:     makeBuckets("alpha", "beta"),
		filterValue: "alpha",
	}
	m.applyFilter()
	if len(m.filteredBuckets) != 1 {
		t.Fatalf("Filtered: got %d, want 1", len(m.filteredBuckets))
	}

	// Clear filter
	m.filterValue = ""
	m.applyFilter()
	if len(m.filteredBuckets) != 2 {
		t.Errorf("After clearing filter: got %d, want 2", len(m.filteredBuckets))
	}
}

func TestSelectedBucket(t *testing.T) {
	m := BucketListModel{
		filteredBuckets: makeBuckets("alpha", "beta", "gamma"),
		cursor:          1,
	}
	if got := m.SelectedBucket(); got != "beta" {
		t.Errorf("SelectedBucket = %q, want %q", got, "beta")
	}
}

func TestSelectedBucket_Empty(t *testing.T) {
	m := BucketListModel{
		filteredBuckets: nil,
		cursor:          0,
	}
	if got := m.SelectedBucket(); got != "" {
		t.Errorf("SelectedBucket on empty list = %q, want empty", got)
	}
}

func TestSelectedBucket_OutOfBounds(t *testing.T) {
	m := BucketListModel{
		filteredBuckets: makeBuckets("only"),
		cursor:          5,
	}
	if got := m.SelectedBucket(); got != "" {
		t.Errorf("SelectedBucket out of bounds = %q, want empty", got)
	}
}
