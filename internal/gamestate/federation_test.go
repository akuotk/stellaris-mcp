package gamestate

import (
	"context"
	"os"
	"testing"

	"github.com/bmassemin/stellaris-mcp/internal/clausewitz"
)

func TestFederationIndexedRecordsAndProgression(t *testing.T) {
	gs, err := Load([]byte(`federation={
  0={name={key="Synthetic union"} leader=0 members={0 8} associates={21} start_date="2201.01.01"
   federation_progression={federation_type="synthetic_type" experience=0 base_cohesion=-0.25 cohesion=12.5 levels=0
    laws={z_law=option_two a_law=option_one}
    action_settings={synthetic_action={vote_type=unanimous acceptance_type=full}}
    envoy={17 18} perks={{level=2 type="synthetic_perk"}}
    timed_modifier={items={{modifier="synthetic_modifier" days=-1 multiplier=0.5}}}
    succession_type="rotation" succession_term="term_option" last_succession_date="2201.01.02" expired="2202.01.01"
    flags={synthetic_flag=1234} only_leader_builds_fleets=no allow_subjects_to_join=yes equal_voting_power=no free_migration=yes research_agreement=no failed_vote_half_price=yes research_sharing=no
   }
   ship_design_collection={ship_design={7 9}}
   proposed_law_votes_cooldowns={token="first_vote" target=0 start_date="2201.02.01"}
   proposed_law_votes_cooldowns={token="second_vote" target=8 start_date="2201.03.01"}
  }
  7={name={key="Synthetic empty progression"} federation_progression={} members={}}
  100=none
 }`))
	if err != nil {
		t.Fatal(err)
	}
	if len(gs.Federation) != 2 {
		t.Fatalf("indexed records not retained: %d", len(gs.Federation))
	}
	f := gs.Federation[0]
	p := f.FederationProgression
	if f.Leader == nil || *f.Leader != 0 || len(f.Members) != 2 || len(f.Associates) != 1 || f.StartDate != "2201.01.01" {
		t.Fatal("zero leader or membership lost")
	}
	if p.Experience == nil || *p.Experience != 0 || p.Cohesion == nil || *p.Cohesion != 12.5 || p.Levels == nil || *p.Levels != 0 || p.BaseCohesion == nil || *p.BaseCohesion != -0.25 {
		t.Fatal("progress values missing or scaled")
	}
	if p.Laws["a_law"] != "option_one" || p.ActionSettings["synthetic_action"].VoteType != "unanimous" || len(p.Envoy) != 2 || p.Perks[0].Type != "synthetic_perk" {
		t.Fatal("laws/actions/envoys/perks missing")
	}
	if len(p.TimedModifier.Items) != 1 || p.TimedModifier.Items[0].Multiplier == nil || *p.TimedModifier.Items[0].Multiplier != 0.5 || p.TimedModifier.Items[0].Days == nil || *p.TimedModifier.Items[0].Days != -1 {
		t.Fatal("modifier fields missing")
	}
	if p.OnlyLeaderBuildsFleets == nil || *p.OnlyLeaderBuildsFleets || p.AllowSubjectsToJoin == nil || !*p.AllowSubjectsToJoin || p.Flags["synthetic_flag"] != "1234" {
		t.Fatal("explicit false or flags missing")
	}
	if len(f.ProposedLawVotesCooldowns) != 2 || f.ProposedLawVotesCooldowns[0].Token != "first_vote" || f.ProposedLawVotesCooldowns[1].Target != "8" || len(f.ShipDesignCollection.ShipDesign) != 2 {
		t.Fatal("duplicate cooldowns or ship references lost")
	}
	empty := gs.Federation[7]
	if empty.Leader != nil || empty.FederationProgression.Cohesion != nil || empty.FederationProgression.FreeMigration != nil {
		t.Fatal("absent fields turned into valid zero values")
	}
}

func TestFederationEmptyTableHasNoPhantomRecord(t *testing.T) {
	gs, err := Load([]byte(`federation={}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(gs.Federation) != 0 {
		t.Fatal("empty table became a federation")
	}
}

func TestFederationLocalSaveCatalogIntegration(t *testing.T) {
	dir := os.Getenv("STELLARIS_TEST_SAVE_DIR")
	if dir == "" {
		t.Skip("set STELLARIS_TEST_SAVE_DIR for local read-only federation integration")
	}
	files, err := ListSaveFiles(dir)
	if err != nil {
		t.Fatal("save catalog read failed")
	}
	empty, nonempty, records := 0, 0, 0
	for index, file := range files {
		data, err := extractGamestate(file.Path)
		if err != nil {
			t.Fatalf("save index %d read failed", index)
		}
		root, err := clausewitz.ParseRaw(context.Background(), data)
		if err != nil {
			t.Fatalf("save index %d raw parsing failed", index)
		}
		var table *clausewitz.RawNode
		for _, child := range root.Children {
			if child.Key != nil && child.Key.Text == "federation" {
				table = child
				break
			}
		}
		if table == nil {
			continue
		}
		// Parse only the local federation subtree so unrelated legacy typed fields cannot hide coverage.
		gs, err := Load(data[table.EntryStart:table.End])
		if err != nil {
			t.Fatalf("save index %d federation parsing failed", index)
		}
		expected := 0
		for _, child := range table.Children {
			if !child.IsScalar() {
				expected++
			}
		}
		if len(gs.Federation) != expected {
			t.Fatalf("save index %d federation record count mismatch", index)
		}
		if expected == 0 {
			empty++
		} else {
			nonempty++
			records += expected
		}
		for _, f := range gs.Federation {
			if f.Name.Key == "" || f.Leader == nil || len(f.Members) == 0 || f.FederationProgression.FederationType == "" || f.FederationProgression.Cohesion == nil {
				t.Fatalf("save index %d federation identity/progression missing", index)
			}
		}
	}
	t.Logf("local federation coverage: catalog_saves=%d empty_tables=%d nonempty_tables=%d parsed_records=%d", len(files), empty, nonempty, records)
}
