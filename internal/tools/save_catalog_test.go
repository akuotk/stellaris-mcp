package tools

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/bmassemin/stellaris-mcp/internal/gamestate"
)

func TestSaveCatalogPaginationRejectsChangedList(t *testing.T) {
	dir := syntheticSaveDir(t, []syntheticEntry{{"gamestate", []byte("a=1")}})
	writeSyntheticSave(t, filepath.Join(dir, "another.sav"), []syntheticEntry{{"gamestate", []byte("b=2")}})
	first := rawCall(t, handleSaveCatalog, map[string]any{"limit": 1})
	if first["count"] != float64(2) || first["next_offset"] != float64(1) {
		t.Fatal(first)
	}
	rev := first["revision"].(string)
	second := rawCall(t, handleSaveCatalog, map[string]any{"revision": rev, "offset": 1, "limit": 1})
	if second["next_offset"] != nil || len(second["saves"].([]any)) != 1 {
		t.Fatal(second)
	}
	if err := os.WriteFile(filepath.Join(dir, "new.sav"), []byte("synthetic"), 0600); err != nil {
		t.Fatal(err)
	}
	r, err := handleSaveCatalog(context.Background(), rawRequest(map[string]any{"revision": rev, "offset": 1}))
	if err != nil || !r.IsError {
		t.Fatal("stale catalog accepted")
	}
	r, err = handleSaveCatalog(context.Background(), rawRequest(map[string]any{"offset": 1}))
	if err != nil || !r.IsError {
		t.Fatal("unversioned continuation accepted")
	}
}

func TestAnalysisSaveSelection(t *testing.T) {
	dir := syntheticSaveDir(t, []syntheticEntry{{"gamestate", []byte(`date="2200.01.01"`)}})
	older := filepath.Join(dir, "synthetic.sav")
	newer := filepath.Join(dir, "newer.sav")
	writeSyntheticSave(t, newer, []syntheticEntry{{"gamestate", []byte(`date="2300.01.01"`)}})
	stamp := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	if err := os.Chtimes(older, stamp, stamp); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(newer, stamp.Add(time.Hour), stamp.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	files, err := gamestate.ListSaveFiles(dir)
	if err != nil || len(files) != 2 {
		t.Fatalf("list: %v %v", files, err)
	}
	latest, err := loadSave(rawRequest(nil))
	if err != nil || latest.Date != "2300.01.01" {
		t.Fatalf("latest: %v %v", latest, err)
	}
	selected, err := loadSave(rawRequest(map[string]any{"save_id": files[1].ID}))
	if err != nil || selected.Date != "2200.01.01" {
		t.Fatalf("selected: %v %v", selected, err)
	}
	if _, err := loadSave(rawRequest(map[string]any{"save_id": "../../outside.sav"})); err == nil {
		t.Fatal("accepted arbitrary path")
	}
	if _, err := loadSave(rawRequest(map[string]any{"save_id": 2})); err == nil {
		t.Fatal("accepted non-string save_id")
	}
}
