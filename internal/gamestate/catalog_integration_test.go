package gamestate

import (
	"archive/zip"
	"io"
	"os"
	"testing"

	"github.com/bmassemin/stellaris-mcp/internal/clausewitz"
)

// Optional local compatibility survey; never logs save names or contents.
func TestLocalSaveCatalogCompatibility(t *testing.T) {
	dir := os.Getenv("STELLARIS_TEST_SAVE_DIR")
	if dir == "" {
		t.Skip("set STELLARIS_TEST_SAVE_DIR for a read-only local survey")
	}
	files, err := ListSaveFiles(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("no local saves found")
	}
	passed, failed, compared, mismatches := 0, 0, 0, 0
	for _, file := range files {
		path, err := FindSaveByID(dir, file.ID)
		if err != nil {
			failed++
			continue
		}
		gs, err := LoadFromZip(path)
		if err != nil {
			failed++
		} else {
			passed++
			zr, err := zip.OpenReader(path)
			if err != nil {
				t.Fatal(err)
			}
			for _, f := range zr.File {
				if f.Name != "meta" {
					continue
				}
				stream, err := f.Open()
				if err != nil {
					zr.Close()
					t.Fatal(err)
				}
				data, err := io.ReadAll(io.LimitReader(stream, 1<<20))
				stream.Close()
				if err != nil {
					zr.Close()
					t.Fatal(err)
				}
				var meta struct {
					Version string `clausewitz:"version"`
					Date    string `clausewitz:"date"`
				}
				if clausewitz.Unmarshal(data, &meta) == nil && meta.Version != "" && meta.Date != "" {
					compared++
					if meta.Version != gs.Version || meta.Date != gs.Date {
						mismatches++
					}
				}
			}
			zr.Close()
		}
	}
	t.Logf("local save compatibility: %d total, %d typed parse succeeded, %d failed; metadata version/date compared=%d mismatches=%d", len(files), passed, failed, compared, mismatches)
	if passed == 0 {
		t.Fatal("typed parser could not load any discovered save")
	}
	if mismatches > 0 {
		t.Fatal("metadata and gamestate version/date differed")
	}
}
