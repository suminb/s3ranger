package modals

import (
	"testing"

	"github.com/s3ranger/s3ranger-go/internal/tui/theme"
)

func newTestRenameModel(objKey string, isFolder bool, existingNames []string) RenameModel {
	return NewRename(theme.GithubDark(), nil, "test-bucket", objKey, "prefix/", isFolder, existingNames)
}

func TestRenameValidate_EmptyName(t *testing.T) {
	m := newTestRenameModel("prefix/oldfile.txt", false, nil)
	m.input.SetValue("")
	if m.validate() {
		t.Error("Empty name should fail validation")
	}
	if m.validationErr != "Name cannot be empty" {
		t.Errorf("validationErr = %q, want 'Name cannot be empty'", m.validationErr)
	}
}

func TestRenameValidate_WhitespaceOnly(t *testing.T) {
	m := newTestRenameModel("prefix/oldfile.txt", false, nil)
	m.input.SetValue("   ")
	if m.validate() {
		t.Error("Whitespace-only name should fail validation")
	}
	if m.validationErr != "Name cannot be empty" {
		t.Errorf("validationErr = %q, want 'Name cannot be empty'", m.validationErr)
	}
}

func TestRenameValidate_Unchanged(t *testing.T) {
	m := newTestRenameModel("prefix/oldfile.txt", false, nil)
	m.input.SetValue("oldfile.txt")
	if m.validate() {
		t.Error("Unchanged name should fail validation")
	}
	if m.validationErr != "Name is unchanged" {
		t.Errorf("validationErr = %q, want 'Name is unchanged'", m.validationErr)
	}
}

func TestRenameValidate_DuplicateFile(t *testing.T) {
	m := newTestRenameModel("prefix/oldfile.txt", false, []string{"existing.txt", "other.txt"})
	m.input.SetValue("existing.txt")
	if m.validate() {
		t.Error("Duplicate name should fail validation")
	}
	if m.validationErr != "An item with this name already exists" {
		t.Errorf("validationErr = %q", m.validationErr)
	}
}

func TestRenameValidate_DuplicateFolder(t *testing.T) {
	// When renaming a folder, it checks both "newname/" and "newname"
	m := newTestRenameModel("prefix/oldfolder/", true, []string{"existing/"})
	m.input.SetValue("existing")
	if m.validate() {
		t.Error("Duplicate folder name should fail validation")
	}
	if m.validationErr != "An item with this name already exists" {
		t.Errorf("validationErr = %q", m.validationErr)
	}
}

func TestRenameValidate_DuplicateFolderWithoutSlash(t *testing.T) {
	// existingNames might have the name without trailing slash
	m := newTestRenameModel("prefix/oldfolder/", true, []string{"existing"})
	m.input.SetValue("existing")
	if m.validate() {
		t.Error("Duplicate folder name (no slash) should fail validation")
	}
}

func TestRenameValidate_ValidNewName(t *testing.T) {
	m := newTestRenameModel("prefix/oldfile.txt", false, []string{"other.txt"})
	m.input.SetValue("newfile.txt")
	if !m.validate() {
		t.Errorf("Valid new name should pass, got error: %q", m.validationErr)
	}
	if m.validationErr != "" {
		t.Errorf("validationErr should be empty, got %q", m.validationErr)
	}
}

func TestRenameValidate_ValidFolderRename(t *testing.T) {
	m := newTestRenameModel("prefix/oldfolder/", true, []string{"other/"})
	m.input.SetValue("newfolder")
	if !m.validate() {
		t.Errorf("Valid folder rename should pass, got error: %q", m.validationErr)
	}
}

func TestRenameValidate_NoExistingNames(t *testing.T) {
	m := newTestRenameModel("prefix/file.txt", false, nil)
	m.input.SetValue("renamed.txt")
	if !m.validate() {
		t.Errorf("Should pass with no existing names, got error: %q", m.validationErr)
	}
}

func TestRenameIsDone(t *testing.T) {
	m := newTestRenameModel("prefix/file.txt", false, nil)
	if m.IsDone() {
		t.Error("New model should not be done")
	}
	m.done = true
	if !m.IsDone() {
		t.Error("Model with done=true should be done")
	}
}
