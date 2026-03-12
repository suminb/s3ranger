package components

import (
	"testing"
	"time"
)

func makeTestItems() []ObjectItem {
	return []ObjectItem{
		{Key: "..", Name: "..", IsFolder: true, IsParent: true},
		{Key: "docs/", Name: "docs", Type: "dir", IsFolder: true},
		{Key: "images/", Name: "images", Type: "dir", IsFolder: true},
		{Key: "readme.md", Name: "readme.md", Type: "md", Size: 1024, LastModified: time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC)},
		{Key: "app.go", Name: "app.go", Type: "go", Size: 4096, LastModified: time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)},
		{Key: "Makefile", Name: "Makefile", Type: "", Size: 512, LastModified: time.Date(2024, 6, 10, 0, 0, 0, 0, time.UTC)},
	}
}

func itemNames(items []ObjectItem) []string {
	names := make([]string, len(items))
	for i, it := range items {
		names[i] = it.Name
	}
	return names
}

func TestSortItems_ByNameAscending(t *testing.T) {
	m := &ObjectListModel{
		items:         makeTestItems(),
		sortColumn:    0,
		sortAscending: true,
	}
	m.sortItems()
	names := itemNames(m.items)

	// Parent always first, then folders (sorted), then files (sorted)
	expected := []string{"..", "docs", "images", "app.go", "Makefile", "readme.md"}
	for i, want := range expected {
		if names[i] != want {
			t.Errorf("Position %d: got %q, want %q (full order: %v)", i, names[i], want, names)
			break
		}
	}
}

func TestSortItems_ByNameDescending(t *testing.T) {
	m := &ObjectListModel{
		items:         makeTestItems(),
		sortColumn:    0,
		sortAscending: false,
	}
	m.sortItems()
	names := itemNames(m.items)

	// Parent first, then folders (desc), then files (desc)
	expected := []string{"..", "images", "docs", "readme.md", "Makefile", "app.go"}
	for i, want := range expected {
		if names[i] != want {
			t.Errorf("Position %d: got %q, want %q (full order: %v)", i, names[i], want, names)
			break
		}
	}
}

func TestSortItems_BySize(t *testing.T) {
	m := &ObjectListModel{
		items:         makeTestItems(),
		sortColumn:    3,
		sortAscending: true,
	}
	m.sortItems()
	names := itemNames(m.items)

	// Parent first, folders (stable order), files by size ascending: Makefile(512), readme.md(1024), app.go(4096)
	if names[0] != ".." {
		t.Errorf("Parent not first: %v", names)
	}
	// Files portion (last 3)
	fileNames := names[3:]
	expectedFiles := []string{"Makefile", "readme.md", "app.go"}
	for i, want := range expectedFiles {
		if fileNames[i] != want {
			t.Errorf("File position %d: got %q, want %q (files: %v)", i, fileNames[i], want, fileNames)
			break
		}
	}
}

func TestSortItems_BySizeDescending(t *testing.T) {
	m := &ObjectListModel{
		items:         makeTestItems(),
		sortColumn:    3,
		sortAscending: false,
	}
	m.sortItems()
	names := itemNames(m.items)

	fileNames := names[3:]
	expectedFiles := []string{"app.go", "readme.md", "Makefile"}
	for i, want := range expectedFiles {
		if fileNames[i] != want {
			t.Errorf("File position %d: got %q, want %q (files: %v)", i, fileNames[i], want, fileNames)
			break
		}
	}
}

func TestSortItems_ByModified(t *testing.T) {
	m := &ObjectListModel{
		items:         makeTestItems(),
		sortColumn:    2,
		sortAscending: true,
	}
	m.sortItems()
	names := itemNames(m.items)

	// Files by date ascending: app.go(Jan), readme.md(Mar), Makefile(Jun)
	fileNames := names[3:]
	expectedFiles := []string{"app.go", "readme.md", "Makefile"}
	for i, want := range expectedFiles {
		if fileNames[i] != want {
			t.Errorf("File position %d: got %q, want %q (files: %v)", i, fileNames[i], want, fileNames)
			break
		}
	}
}

func TestSortItems_ByType(t *testing.T) {
	m := &ObjectListModel{
		items:         makeTestItems(),
		sortColumn:    1,
		sortAscending: true,
	}
	m.sortItems()
	names := itemNames(m.items)

	// Files by type ascending: ""(Makefile), "go"(app.go), "md"(readme.md)
	fileNames := names[3:]
	expectedFiles := []string{"Makefile", "app.go", "readme.md"}
	for i, want := range expectedFiles {
		if fileNames[i] != want {
			t.Errorf("File position %d: got %q, want %q (files: %v)", i, fileNames[i], want, fileNames)
			break
		}
	}
}

func TestSortItems_ParentAlwaysFirst(t *testing.T) {
	items := []ObjectItem{
		{Key: "z.txt", Name: "z.txt", Size: 100},
		{Key: "..", Name: "..", IsFolder: true, IsParent: true},
		{Key: "a.txt", Name: "a.txt", Size: 200},
	}
	m := &ObjectListModel{items: items, sortColumn: 0, sortAscending: true}
	m.sortItems()
	if m.items[0].Name != ".." {
		t.Errorf("Parent not first after sort: %v", itemNames(m.items))
	}
}

func TestSortItems_FoldersBeforeFiles(t *testing.T) {
	items := []ObjectItem{
		{Key: "file.txt", Name: "file.txt"},
		{Key: "aaa/", Name: "aaa", Type: "dir", IsFolder: true},
		{Key: "zzz.go", Name: "zzz.go", Type: "go"},
	}
	m := &ObjectListModel{items: items, sortColumn: 0, sortAscending: true}
	m.sortItems()
	if !m.items[0].IsFolder {
		t.Errorf("First item should be a folder: %v", itemNames(m.items))
	}
	if m.items[1].IsFolder {
		t.Errorf("Second item should be a file: %v", itemNames(m.items))
	}
}

func TestSortItems_CaseInsensitive(t *testing.T) {
	items := []ObjectItem{
		{Key: "Banana.txt", Name: "Banana.txt"},
		{Key: "apple.txt", Name: "apple.txt"},
		{Key: "cherry.txt", Name: "cherry.txt"},
	}
	m := &ObjectListModel{items: items, sortColumn: 0, sortAscending: true}
	m.sortItems()
	expected := []string{"apple.txt", "Banana.txt", "cherry.txt"}
	names := itemNames(m.items)
	for i, want := range expected {
		if names[i] != want {
			t.Errorf("Position %d: got %q, want %q (full: %v)", i, names[i], want, names)
			break
		}
	}
}

func TestSortItems_EmptyAndSingle(t *testing.T) {
	// Empty
	m := &ObjectListModel{items: nil, sortColumn: 0, sortAscending: true}
	m.sortItems() // should not panic

	// Single item
	m.items = []ObjectItem{{Key: "only.txt", Name: "only.txt"}}
	m.sortItems() // should not panic
	if m.items[0].Name != "only.txt" {
		t.Error("Single item changed after sort")
	}
}

func TestSetSort_ToggleDirection(t *testing.T) {
	m := &ObjectListModel{
		items:         makeTestItems(),
		sortColumn:    0,
		sortAscending: true,
	}

	// Same column → toggle to descending
	m.SetSort(0)
	if m.sortAscending {
		t.Error("SetSort same column should toggle to descending")
	}

	// Same column again → toggle back to ascending
	m.SetSort(0)
	if !m.sortAscending {
		t.Error("SetSort same column should toggle back to ascending")
	}
}

func TestSetSort_ChangeColumn(t *testing.T) {
	m := &ObjectListModel{
		items:         makeTestItems(),
		sortColumn:    0,
		sortAscending: true,
	}

	// Different column → set to descending
	m.SetSort(3)
	if m.sortColumn != 3 {
		t.Errorf("sortColumn = %d, want 3", m.sortColumn)
	}
	if m.sortAscending {
		t.Error("New column should start descending")
	}

	// Verify items are actually reordered by size descending
	fileNames := itemNames(m.items)[3:] // skip parent + 2 folders
	expectedFiles := []string{"app.go", "readme.md", "Makefile"} // 4096, 1024, 512
	for i, want := range expectedFiles {
		if fileNames[i] != want {
			t.Errorf("SetSort size desc: position %d: got %q, want %q (all: %v)", i, fileNames[i], want, fileNames)
		}
	}
}

func TestSetSort_SizeFromNameSorted(t *testing.T) {
	// Simulate: items loaded and sorted by name, then user sorts by size
	m := &ObjectListModel{
		items:         makeTestItems(),
		sortColumn:    0,
		sortAscending: true,
	}
	m.sortItems() // sort by name first

	// Verify name order: Makefile, app.go, readme.md
	filesBefore := itemNames(m.items)[3:]
	if filesBefore[0] != "app.go" {
		t.Errorf("Expected name sort: got %v", filesBefore)
	}

	// Now sort by size
	m.SetSort(3)

	// Should be size descending: app.go(4096), readme.md(1024), Makefile(512)
	filesAfter := itemNames(m.items)[3:]
	expected := []string{"app.go", "readme.md", "Makefile"}
	for i, want := range expected {
		if filesAfter[i] != want {
			t.Errorf("Size desc position %d: got %q, want %q (all: %v)", i, filesAfter[i], want, filesAfter)
		}
	}
}

func TestSelectedItems(t *testing.T) {
	m := ObjectListModel{
		items: []ObjectItem{
			{Key: "a.txt", Name: "a.txt"},
			{Key: "b.txt", Name: "b.txt"},
			{Key: "c.txt", Name: "c.txt"},
		},
		selectedKeys: map[string]bool{
			"a.txt": true,
			"c.txt": true,
		},
	}

	selected := m.SelectedItems()
	if len(selected) != 2 {
		t.Fatalf("SelectedItems count = %d, want 2", len(selected))
	}

	keys := make(map[string]bool)
	for _, s := range selected {
		keys[s.Key] = true
	}
	if !keys["a.txt"] || !keys["c.txt"] {
		t.Errorf("Selected keys = %v, want a.txt and c.txt", keys)
	}
}

func TestSelectedItems_Empty(t *testing.T) {
	m := ObjectListModel{
		items: []ObjectItem{
			{Key: "a.txt", Name: "a.txt"},
		},
		selectedKeys: make(map[string]bool),
	}
	if len(m.SelectedItems()) != 0 {
		t.Error("SelectedItems should be empty")
	}
}

func TestSelectedCount(t *testing.T) {
	m := ObjectListModel{
		selectedKeys: map[string]bool{"a": true, "b": true},
	}
	if m.SelectedCount() != 2 {
		t.Errorf("SelectedCount = %d, want 2", m.SelectedCount())
	}
}

func TestCursorItem(t *testing.T) {
	m := ObjectListModel{
		items: []ObjectItem{
			{Key: "..", Name: "..", IsParent: true},
			{Key: "file.txt", Name: "file.txt"},
		},
		cursor: 0,
	}

	// Parent entry → nil
	if m.CursorItem() != nil {
		t.Error("CursorItem on parent should return nil")
	}

	// Regular item
	m.cursor = 1
	item := m.CursorItem()
	if item == nil {
		t.Fatal("CursorItem on file should not be nil")
	}
	if item.Key != "file.txt" {
		t.Errorf("CursorItem.Key = %q, want %q", item.Key, "file.txt")
	}
}

func TestCursorItem_OutOfBounds(t *testing.T) {
	m := ObjectListModel{items: nil, cursor: 0}
	if m.CursorItem() != nil {
		t.Error("CursorItem on empty list should return nil")
	}

	m = ObjectListModel{
		items:  []ObjectItem{{Key: "a.txt"}},
		cursor: 5,
	}
	if m.CursorItem() != nil {
		t.Error("CursorItem with cursor out of bounds should return nil")
	}
}

func TestAllItems(t *testing.T) {
	items := []ObjectItem{
		{Key: "a.txt"},
		{Key: "b.txt"},
	}
	m := ObjectListModel{items: items}
	got := m.AllItems()
	if len(got) != 2 {
		t.Errorf("AllItems len = %d, want 2", len(got))
	}
}

func TestCursorItemRaw_IncludesParent(t *testing.T) {
	m := ObjectListModel{
		items: []ObjectItem{
			{Key: "..", Name: "..", IsParent: true},
			{Key: "file.txt", Name: "file.txt"},
		},
		cursor: 0,
	}

	// CursorItem filters out parent
	if m.CursorItem() != nil {
		t.Error("CursorItem should return nil for parent")
	}

	// CursorItemRaw includes parent
	raw := m.CursorItemRaw()
	if raw == nil {
		t.Fatal("CursorItemRaw should return parent item")
	}
	if !raw.IsParent {
		t.Error("CursorItemRaw should return the parent item")
	}
}

func TestCursorItemRaw_OutOfBounds(t *testing.T) {
	m := ObjectListModel{items: nil, cursor: 0}
	if m.CursorItemRaw() != nil {
		t.Error("CursorItemRaw on empty list should return nil")
	}
}
