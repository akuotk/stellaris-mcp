package tools

import (
	"github.com/bmassemin/stellaris-mcp/internal/gamestate"
	"os"
	"testing"
)

// Aggregate-only audit of local saves; no save content is logged.
func TestLocalSpeciesReferenceAmbiguity(t *testing.T) {
	dir := os.Getenv("STELLARIS_TEST_SAVE_DIR")
	if dir == "" {
		t.Skip("local catalog not set")
	}
	files, err := gamestate.ListSaveFiles(dir)
	if err != nil || len(files) == 0 {
		t.Fatal("catalog unavailable or empty")
	}
	var saves, countries, groups, duplicates, ambiguous, missing int
	for _, f := range files {
		gs, err := gamestate.LoadFromZip(f.Path)
		if err != nil {
			t.Fatal("save decode failed")
		}
		saves++
		for id := range gs.Country {
			countries++
			refs := map[int]map[int]bool{}
			for pid, p := range gs.Planets.Planet {
				if p.OwnerID == nil || *p.OwnerID != id {
					continue
				}
				for _, gid := range p.PopGroups {
					if refs[gid] == nil {
						refs[gid] = map[int]bool{}
					} else {
						duplicates++
					}
					refs[gid][pid] = true
				}
			}
			for gid, planets := range refs {
				groups++
				if len(planets) > 1 {
					ambiguous++
				}
				if _, ok := gs.PopGroups[gid]; !ok {
					missing++
				}
			}
		}
	}
	t.Logf("saves=%d countries=%d unique_group_refs=%d duplicate_refs=%d multi_planet_groups=%d missing_group_records=%d", saves, countries, groups, duplicates, ambiguous, missing)
}
