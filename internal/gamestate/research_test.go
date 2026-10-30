package gamestate

import "testing"

func TestTechnologyLevelIsOnlyPairedWhenAdjacent(t *testing.T) {
	gs, err := Load([]byte(`country={0={tech_status={technology="tech_a" level=3 technology="tech_b" other=1 level=9 technology="tech_c" level=2 stored_techpoints={2 4 6} stored_techpoints_for_tech={tech_a=5.5} physics_queue={{technology="tech_a" progress=4 date="2200.01.02"}}}}}`))
	if err != nil {
		t.Fatal(err)
	}
	ts := gs.Country[0].TechStatus
	if len(ts.Technology) != 3 || len(ts.CompletedLevels) != 3 || ts.CompletedLevels[0] == nil || *ts.CompletedLevels[0] != 3 || ts.CompletedLevels[1] != nil || ts.CompletedLevels[2] == nil || *ts.CompletedLevels[2] != 2 || ts.StoredForTech["tech_a"] != 5.5 || len(ts.StoredPoints) != 3 || ts.PhysicsQueue[0].Date != "2200.01.02" {
		t.Fatalf("incorrect research: %+v", ts)
	}
}
