package gamestate

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func syntheticFlagGame(t *testing.T) (root, locale string) {
	t.Helper()
	root = t.TempDir()
	locale = filepath.Join(root, "localisation", "english")
	for _, dir := range []string{locale, filepath.Join(root, "events"), filepath.Join(root, "common"), filepath.Join(root, "dlc")} {
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	for _, marker := range []string{"launcher-settings.json", "stellaris.exe"} {
		writeFlagFixture(t, filepath.Join(root, marker), "")
	}
	return root, locale
}

func writeFlagFixture(t *testing.T, path, text string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
}

func writeFlagZipFixture(t *testing.T, filename string, entries [][2]string) {
	t.Helper()
	f, err := os.Create(filename)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	z := zip.NewWriter(f)
	for _, entry := range entries {
		w, err := z.Create(entry[0])
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(entry[1])); err != nil {
			t.Fatal(err)
		}
	}
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestFlagScriptIndexLiteralSitesAndFiles(t *testing.T) {
	root, locale := syntheticFlagGame(t)
	common := filepath.Join(root, "common", "synthetic.txt")
	source := NewFlagScriptSource(locale)
	writeFlagFixture(t, common, `example={
		set_global_flag=sample
		set_global_flag="sample"
		set_global_flag="quoted\"name"
		set_timed_global_flag={days=8 flag=sample}
		has_global_flag=sample
		has_global_flag=sample
		has_global_flag=Sample
		set_country_flag=sample
		# remove_global_flag=sample
		text="remove_global_flag=sample"
	}`)
	writeFlagFixture(t, filepath.Join(root, "events", "synthetic.TXT"), `x={set_global_flag=sample remove_global_flag=sample}`)
	writeFlagFixture(t, filepath.Join(root, "events", "ignored.log"), `x={set_global_flag=sample}`)
	writeFlagZipFixture(t, filepath.Join(root, "dlc", "synthetic.zip"), [][2]string{
		{"events/synthetic.txt", `x={set_timed_global_flag={flag="sample" years=2}}`},
		{"ignored.log", `x={set_global_flag=sample}`},
	})
	var wg sync.WaitGroup
	indexes := make([]FlagScriptIndex, 8)
	for i := range indexes {
		wg.Go(func() { indexes[i] = source() })
	}
	wg.Wait()
	expected := [...]FlagScriptReference{
		{"set_global_flag", 3, 2}, {"set_timed_global_flag", 2, 2},
		{"has_global_flag", 2, 1}, {"remove_global_flag", 1, 1},
	}
	for _, index := range indexes {
		if index.Status != "complete" || len(index.Fingerprint) != 64 || index.Fingerprint != indexes[0].Fingerprint ||
			index.References("sample") != expected || index.References("Sample")[2].Sites != 1 ||
			index.References(`quoted"name`)[0].Sites != 1 {
			t.Fatalf("unexpected literal index: status=%s refs=%+v", index.Status, index.References("sample"))
		}
	}
	writeFlagFixture(t, common, "x={set_global_flag=sample}")
	if source().References("sample") != expected {
		t.Fatal("source snapshot changed")
	}
	changed := NewFlagScriptSource(locale)()
	if changed.Fingerprint == indexes[0].Fingerprint || changed.References("sample")[0].Sites != 2 {
		t.Fatal("new source failed to reflect script edit")
	}
}

func TestFlagScriptIndexPartialSources(t *testing.T) {
	for _, fixture := range []struct {
		name  string
		setup func(*testing.T, string)
	}{
		{"dynamic and unsupported", func(t *testing.T, root string) {
			writeFlagFixture(t, filepath.Join(root, "events", "unsupported.txt"), `x={
				set_global_flag="$dynamic$"
				has_global_flag=event_target:dynamic
				set_timed_global_flag={flag=sample flag=sample}
				set_timed_global_flag={flag=sample anonymous}
				remove_global_flag={flag=sample set_global_flag=sample}
			}`)
		}},
		{"parse failure", func(t *testing.T, root string) {
			writeFlagFixture(t, filepath.Join(root, "events", "broken.txt"), `x={set_global_flag=sample`)
		}},
		{"oversized script", func(t *testing.T, root string) {
			writeFlagFixture(t, filepath.Join(root, "events", "large.txt"), strings.Repeat(" ", maxFlagScriptBytes+1))
		}},
		{"archive traversal", func(t *testing.T, root string) {
			writeFlagZipFixture(t, filepath.Join(root, "dlc", "unsafe.zip"), [][2]string{{"../escaped.txt", "set_global_flag=sample"}})
		}},
		{"archive duplicate entry", func(t *testing.T, root string) {
			writeFlagZipFixture(t, filepath.Join(root, "dlc", "duplicate.zip"), [][2]string{
				{"events/duplicate.txt", "set_global_flag=sample"}, {"events/duplicate.txt", "set_global_flag=sample"},
			})
		}},
		{"oversized archive member", func(t *testing.T, root string) {
			writeFlagZipFixture(t, filepath.Join(root, "dlc", "large.zip"), [][2]string{
				{"events/large.txt", strings.Repeat(" ", maxFlagScriptBytes+1)},
			})
		}},
		{"broken archive", func(t *testing.T, root string) {
			writeFlagFixture(t, filepath.Join(root, "dlc", "broken.zip"), "invalid ZIP")
		}},
		{"file link", func(t *testing.T, root string) {
			outside := filepath.Join(t.TempDir(), "outside.txt")
			writeFlagFixture(t, outside, "set_global_flag=sample")
			if err := os.Symlink(outside, filepath.Join(root, "events", "link.txt")); err != nil {
				t.Skip("OS symlink creation unavailable")
			}
		}},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			root, locale := syntheticFlagGame(t)
			writeFlagFixture(t, filepath.Join(root, "common", "valid.txt"), "set_global_flag=sample")
			fixture.setup(t, root)
			index := NewFlagScriptSource(locale)()
			if index.Status != "partial" || index.References("sample")[0].Sites != 1 ||
				index.References("sample")[1].Sites != 0 || index.References("sample")[3].Sites != 0 {
				t.Fatalf("expected a partial index with one usable site, got %s %+v", index.Status, index.References("sample"))
			}
		})
	}
}

func TestFlagScriptIndexUnavailable(t *testing.T) {
	root, locale := syntheticFlagGame(t)
	for _, path := range []string{"", root, filepath.Join(root, "common"), locale} {
		if index := NewFlagScriptSource(path)(); index.Status != "unavailable" {
			t.Fatalf("expected unavailable, got %s", index.Status)
		}
	}
	writeFlagFixture(t, filepath.Join(root, "events", "synthetic.txt"), "set_global_flag=sample")
	if err := os.Remove(filepath.Join(root, "launcher-settings.json")); err != nil {
		t.Fatal(err)
	}
	if index := NewFlagScriptSource(locale)(); index.Status != "unavailable" {
		t.Fatalf("unexpected status without installation marker: %s", index.Status)
	}
}
