package gamestate

import (
	"archive/zip"
	"context"
	"github.com/bmassemin/stellaris-mcp/internal/clausewitz"
	"io"
	"os"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"
)

func TestLocalTraditionFieldShapes(t *testing.T) {
	path := os.Getenv("STELLARIS_TEST_SAVE")
	if path == "" {
		t.Skip("local save unset")
	}
	z, e := zip.OpenReader(path)
	if e != nil {
		t.Fatal(e)
	}
	defer z.Close()
	var data []byte
	for _, f := range z.File {
		if f.Name == "gamestate" {
			r, e := f.Open()
			if e != nil {
				t.Fatal(e)
			}
			data, e = io.ReadAll(r)
			r.Close()
			if e != nil {
				t.Fatal(e)
			}
			break
		}
	}
	root, e := clausewitz.ParseRaw(context.Background(), data)
	if e != nil {
		t.Fatal(e)
	}
	counts := map[string]int{}
	for _, rootNode := range root.Children {
		if rootNode.Key == nil || rootNode.Key.Text != "country" {
			continue
		}
		for _, country := range rootNode.Children {
			for _, field := range country.Children {
				if field.Key == nil {
					continue
				}
				key := field.Key.Text
				if strings.Contains(key, "tradition") || strings.Contains(key, "ascension") {
					counts[key+":"+field.Kind]++
				}
			}
		}
	}
	keys := make([]string, 0, len(counts))
	for k := range counts {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		t.Logf("field=%s country_records=%d", k, counts[k])
	}
}

func TestLocalTraditionListsMatchRaw(t *testing.T) {
	dir := os.Getenv("STELLARIS_TEST_SAVE_DIR")
	if dir == "" {
		t.Skip("local catalog not set")
	}
	files, e := ListSaveFiles(dir)
	if e != nil {
		t.Fatal(e)
	}
	var saves, records, unsupported, mismatch, lastRecords, lastMismatch int
	for _, file := range files {
		z, e := zip.OpenReader(file.Path)
		if e != nil {
			t.Fatal("save open failed")
		}
		var data []byte
		for _, f := range z.File {
			if f.Name == "gamestate" {
				r, err := f.Open()
				if err != nil {
					t.Fatal("gamestate open failed")
				}
				data, err = io.ReadAll(r)
				r.Close()
				if err != nil {
					t.Fatal("gamestate read failed")
				}
				break
			}
		}
		z.Close()
		gs, err := Load(data)
		if err != nil {
			t.Fatal("typed decode failed")
		}
		root, err := clausewitz.ParseRaw(context.Background(), data)
		if err != nil {
			t.Fatal("raw parse failed")
		}
		saves++
		for _, node := range root.Children {
			if node.Key == nil || node.Key.Text != "country" {
				continue
			}
			for _, cn := range node.Children {
				if cn.Key == nil {
					continue
				}
				id, err := strconv.Atoi(cn.Key.Text)
				if err != nil {
					continue
				}
				country, ok := gs.Country[id]
				if !ok {
					continue
				}
				for _, field := range cn.Children {
					if field.Key == nil {
						continue
					}
					if field.Key.Text == "last_picked_tradition" {
						lastRecords++
						value := country.LastPickedTradition
						if field.IsScalar() {
							if !value.Recorded || value.Unsupported || value.Value != field.Scalar.Text {
								lastMismatch++
							}
						} else if !value.Recorded || !value.Unsupported {
							lastMismatch++
						}
						continue
					}
					var list OptionalStringList
					switch field.Key.Text {
					case "traditions":
						list = country.Traditions
					case "ascension_perks":
						list = country.AscensionPerks
					case "tradition_categories":
						list = country.TraditionCategories
					default:
						continue
					}
					records++
					if list.Unsupported {
						unsupported++
						continue
					}
					var actual []string
					if field.IsScalar() {
						actual = []string{field.Scalar.Text}
					} else {
						for _, child := range field.Children {
							if child.Key != nil || !child.IsScalar() {
								unsupported++
								actual = nil
								break
							}
							actual = append(actual, child.Scalar.Text)
						}
					}
					if !reflect.DeepEqual(actual, list.Values) && !(len(actual) == 0 && len(list.Values) == 0) {
						mismatch++
					}
				}
			}
		}
	}
	t.Logf("tradition audit saves=%d list_records=%d unsupported=%d mismatch=%d last_records=%d last_mismatch=%d", saves, records, unsupported, mismatch, lastRecords, lastMismatch)
	if mismatch > 0 || lastMismatch > 0 {
		t.Fatal("raw and typed lists disagree")
	}
}
