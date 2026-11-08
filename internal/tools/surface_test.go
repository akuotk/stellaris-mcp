package tools

import (
	"strings"
	"testing"

	"github.com/bmassemin/stellaris-mcp/internal/gamestate"
)

func TestPlanetSurfaceReferencesAndLegacyFeatures(t *testing.T) {
	old := loc
	loc = gamestate.NewLocalizer(t.TempDir())
	t.Cleanup(func() { loc = old })
	gs, err := gamestate.Load([]byte(`planets={planet={0={districts={0} buildings_cache={0} buildings={0 99} deposits={0 1 99} terraform_process={progress=0.25}}}} districts={0={type=d_city level=1 zones={0}}} zones={0={type=zone_city buildings={0}}} buildings={0={type=building_lab position=0 ruined=no}} deposit={0={type=d_active_volcano swap_type=d_cleared deposit_holder={type=planet id=0}} 1={type=d_feature deposit_holder={type=planet id=1}}}`))
	if err != nil {
		t.Fatal(err)
	}
	result, err := planetDetail(gs, 0)
	if err != nil {
		t.Fatal(err)
	}
	got := resultText(result)
	for _, want := range []string{"District 0:", "Zone 0:", "Building 0:", "Building 99:", "Deposit 0:", "holder ID matches referencing planet ID", "holder ID differs from referencing planet ID", "Deposit 99:", "Planetary Features", "Blockers", "terraform_process"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in %s", want, got)
		}
	}
}

func TestSurfaceAmbiguousFieldsAreReported(t *testing.T) {
	old := loc
	loc = gamestate.NewLocalizer(t.TempDir())
	t.Cleanup(func() { loc = old })
	gs, err := gamestate.Load([]byte(`planets={planet={0={districts={0} districts={1} deposits={0} terraform_process={} terraform_process={}}}} deposit={0={type=d_feature deposit_holder={type=planet id=0 id=1}}}`))
	if err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	writePlanetDeposits(&b, gs.Deposit, 0, gs.Planets.Planet[0].DepositRefs)
	if !strings.Contains(b.String(), "duplicate saved fields") {
		t.Fatal(b.String())
	}
	if !gs.Planets.Planet[0].DistrictRefs.Duplicate || !gs.Planets.Planet[0].TerraformDuplicate {
		t.Fatal("duplicate fields not retained")
	}
}

func TestSurfaceMissingAndExternalReferences(t *testing.T) {
	old := loc
	loc = gamestate.NewLocalizer(t.TempDir())
	t.Cleanup(func() { loc = old })
	gs, err := gamestate.Load([]byte(`planets={planet={0={colony=0 districts={0 99} buildings_cache={0} deposits={0 99} externally_owned_buildings={{building_owner=1 owner_type=country buildings={0 98}}}}}} colony={0={carrier={type=planet reference=0} externally_owned_buildings={{building_owner=2 owner_type=country buildings={0}}}}} districts={0={type=d_city zones={0 99}}} zones={0={type=zone_city buildings={0 99}}} buildings={0={type=building_lab}} deposit={0={type=d_feature deposit_holder=invalid}}`))
	if err != nil {
		t.Fatal(err)
	}
	result, err := planetDetail(gs, 0)
	if err != nil {
		t.Fatal(err)
	}
	got := resultText(result)
	for _, want := range []string{"District 99:", "Zone 99:", "Building 99:", "Building 98:", "Deposit 99:", "unsupported saved shape", "planet.externally_owned_buildings", "colony.externally_owned_buildings"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in %s", want, got)
		}
	}
}
