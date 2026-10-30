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
