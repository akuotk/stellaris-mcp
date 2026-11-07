package gamestate

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func syntheticZipForLoad(t *testing.T, names []string, values []string) []byte {
	t.Helper()
	var data bytes.Buffer
	z := zip.NewWriter(&data)
	for i, name := range names {
		w, err := z.CreateHeader(&zip.FileHeader{Name: name, Method: zip.Store})
		if err != nil {
			t.Fatal(err)
		}
		if _, err = w.Write([]byte(values[i])); err != nil {
			t.Fatal(err)
		}
	}
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	return data.Bytes()
}
func TestLoadFromZipRejectsAmbiguousAndCorruptEntries(t *testing.T) {
	for _, tc := range []struct {
		name          string
		names, values []string
		corrupt       bool
		wantErr       string
	}{
		{"single", []string{"gamestate"}, []string{`date="2201.01.01"`}, false, ""},
		{"duplicate", []string{"gamestate", "gamestate"}, []string{`date="2201.01.01"`, `date="2202.01.01"`}, false, "multiple gamestate entries"},
		{"missing", []string{"meta"}, []string{"version=example"}, false, "no gamestate entry"},
		{"corrupt", []string{"gamestate"}, []string{`date="2201.01.01"`}, true, "checksum"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data := syntheticZipForLoad(t, tc.names, tc.values)
			if tc.corrupt {
				i := bytes.Index(data, []byte(tc.values[0]))
				if i < 0 {
					t.Fatal("payload missing")
				}
				data[i] ^= 1
			}
			path := filepath.Join(t.TempDir(), "synthetic.sav")
			if err := os.WriteFile(path, data, 0600); err != nil {
				t.Fatal(err)
			}
			gs, err := LoadFromZip(path)
			if tc.wantErr == "" {
				if err != nil || gs.Date != "2201.01.01" {
					t.Fatalf("valid archive failed: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(strings.ToLower(err.Error()), tc.wantErr) {
				t.Fatalf("expected %q error, got %v", tc.wantErr, err)
			}
		})
	}
}
