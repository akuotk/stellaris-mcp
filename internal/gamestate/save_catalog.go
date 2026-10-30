package gamestate

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// SaveFile identifies a file under the configured save directory. ID is
// derived from its relative path, not its content, and stays stable on updates.
type SaveFile struct {
	ID           string
	RelativePath string
	Path         string
	Modified     time.Time
	Size         int64
}

func ListSaveFiles(dir string) ([]SaveFile, error) {
	var files []SaveFile
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() || !strings.EqualFold(filepath.Ext(d.Name()), ".sav") {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		sum := sha256.Sum256([]byte(rel))
		files = append(files, SaveFile{ID: hex.EncodeToString(sum[:]), RelativePath: rel, Path: path, Modified: info.ModTime(), Size: info.Size()})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk save directory %q: %w", dir, err)
	}
	sort.Slice(files, func(i, j int) bool {
		if files[i].Modified.Equal(files[j].Modified) {
			return files[i].Path < files[j].Path
		}
		return files[i].Modified.After(files[j].Modified)
	})
	return files, nil
}

// ResolveSaveByID accepts only catalog IDs; an empty ID selects latest.
func ResolveSaveByID(dir, id string) (SaveFile, error) {
	files, err := ListSaveFiles(dir)
	if err != nil {
		return SaveFile{}, err
	}
	if len(files) == 0 {
		return SaveFile{}, fmt.Errorf("no .sav files found in %q", dir)
	}
	if id == "" {
		return files[0], nil
	}
	for _, f := range files {
		if f.ID == id {
			return f, nil
		}
	}
	return SaveFile{}, fmt.Errorf("save_id not found; call list_save_games again")
}

// FindSaveByID resolves a catalog ID to its path; empty selects latest.
func FindSaveByID(dir, id string) (string, error) {
	file, err := ResolveSaveByID(dir, id)
	if err != nil {
		return "", err
	}
	return file.Path, nil
}
