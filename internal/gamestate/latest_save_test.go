package gamestate

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFindLatestSaveCaseAndTies(t *testing.T) {
	dir := t.TempDir()
	for _, n := range []string{"z.sav", "a.SAV", "ignore.txt"} {
		path := filepath.Join(dir, n)
		if err := os.WriteFile(path, []byte("synthetic"), 0600); err != nil {
			t.Fatal(err)
		}
		stamp := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
		if err := os.Chtimes(path, stamp, stamp); err != nil {
			t.Fatal(err)
		}
	}
	got, err := FindLatestSave(dir)
	if err != nil || filepath.Base(got) != "a.SAV" {
		t.Fatalf("got %q, %v", got, err)
	}
	if _, err := FindLatestSave(filepath.Join(dir, "absent")); err == nil {
		t.Fatal("missing directory was silently ignored")
	}
}

func TestCatalogSelectionUsesOnlyEnumeratedIDs(t *testing.T) {
	dir := t.TempDir()
	nested := filepath.Join(dir, "campaign")
	if err := os.Mkdir(nested, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(nested, "test.sav")
	if err := os.WriteFile(path, []byte("first"), 0600); err != nil {
		t.Fatal(err)
	}
	files, err := ListSaveFiles(dir)
	if err != nil || len(files) != 1 {
		t.Fatalf("catalog: %v %v", files, err)
	}
	if files[0].RelativePath != "campaign/test.sav" {
		t.Fatal(files[0].RelativePath)
	}
	selected, err := FindSaveByID(dir, files[0].ID)
	if err != nil || selected != path {
		t.Fatalf("selected %q: %v", selected, err)
	}
	if _, err := FindSaveByID(dir, "../../outside.sav"); err == nil {
		t.Fatal("arbitrary path accepted as ID")
	}
	if err := os.WriteFile(path, []byte("second version"), 0600); err != nil {
		t.Fatal(err)
	}
	again, err := ListSaveFiles(dir)
	if err != nil || again[0].ID != files[0].ID {
		t.Fatal("path-based ID changed after save update")
	}
}
