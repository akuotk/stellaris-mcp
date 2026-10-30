package tools

import (
	"archive/zip"
	"context"
	"io"
	"os"
	"strconv"
	"testing"

	"github.com/bmassemin/stellaris-mcp/internal/clausewitz"
	"github.com/bmassemin/stellaris-mcp/internal/gamestate"
)

// The optional audit compares decoded potential entries to an independent raw
// parse, reporting counts only. It does not log or write private save data.
func TestLocalResearchPotentialMatchesRaw(t *testing.T) {
	dir := os.Getenv("STELLARIS_TEST_SAVE_DIR")
	if dir == "" {
		t.Skip("set STELLARIS_TEST_SAVE_DIR for a read-only local audit")
	}
	files, err := gamestate.ListSaveFiles(dir)
	if err != nil {
		t.Fatal(err)
	}
	checked, records, mismatches, failed := 0, 0, 0, 0
	for _, file := range files {
		z, err := zip.OpenReader(file.Path)
		if err != nil {
			failed++
			continue
		}
		var data []byte
		for _, f := range z.File {
			if f.Name == "gamestate" {
				r, e := f.Open()
				if e == nil {
					data, e = io.ReadAll(r)
					r.Close()
				}
				if e != nil {
					data = nil
				}
				break
			}
		}
		z.Close()
		if data == nil {
			failed++
			continue
		}
		gs, err := gamestate.Load(data)
		if err != nil {
			failed++
			continue
		}
		root, err := clausewitz.ParseRaw(context.Background(), data)
		if err != nil {
			failed++
			continue
		}
		var countries *clausewitz.RawNode
		for _, n := range root.Children {
			if n.Key != nil && n.Key.Text == "country" {
				countries = n
				break
			}
		}
		if countries == nil {
			failed++
			continue
		}
		checked++
		for _, country := range countries.Children {
			if country.Key == nil || country.IsScalar() {
				continue
			}
			id, err := strconv.Atoi(country.Key.Text)
			if err != nil {
				continue
			}
			decoded, ok := gs.Country[id]
			if !ok {
				mismatches++
				continue
			}
			var tech *clausewitz.RawNode
			for _, n := range country.Children {
				if n.Key != nil && n.Key.Text == "tech_status" {
					tech = n
					break
				}
			}
			if tech == nil {
				continue
			}
			var potential []*clausewitz.RawNode
			blocks := 0
			for _, n := range tech.Children {
				if n.Key != nil && n.Key.Text == "potential" {
					blocks++
					potential = append(potential, n.Children...)
				}
			}
			if blocks != decoded.TechStatus.PotentialBlocks || len(potential) != len(decoded.TechStatus.Potential) {
				mismatches++
				continue
			}
			for i, n := range potential {
				records++
				v := decoded.TechStatus.Potential[i]
				if n.Key == nil || n.Key.Text != v.Key {
					mismatches++
					continue
				}
				if n.IsScalar() {
					if v.Value == nil || *v.Value != n.Scalar.Text {
						mismatches++
					}
				} else if v.Value != nil {
					mismatches++
				}
			}
		}
	}
	t.Logf("local research audit: saves=%d checked=%d records=%d failed=%d mismatches=%d", len(files), checked, records, failed, mismatches)
	if checked == 0 || failed > 0 || mismatches > 0 {
		t.Fatal("research potential data did not match the raw save structures")
	}
}
