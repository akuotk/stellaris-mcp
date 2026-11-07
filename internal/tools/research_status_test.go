package tools

import (
	"github.com/bmassemin/stellaris-mcp/internal/gamestate"
	"strings"
	"testing"
)

func TestResearchQueuePreservesPresenceAndPrecision(t *testing.T) {
	old := loc
	loc = gamestate.NewLocalizer(t.TempDir())
	t.Cleanup(func() { loc = old })
	gs, err := gamestate.Load([]byte(`country={0={tech_status={physics_queue={{technology=tech_a progress=0 date=""} {technology=tech_b progress=1.23456789} {}} society_queue={} alternatives={physics={} society={tech_a tech_b}}}}}`))
	if err != nil {
		t.Fatal(err)
	}
	ts := gs.Country[0].TechStatus
	var b strings.Builder
	writeQueue(&b, "Physics", ts.PhysicsQueue)
	writeQueue(&b, "Society", ts.SocietyQueue)
	writeQueue(&b, "Engineering", ts.EngineeringQueue)
	writeAlternatives(&b, "Physics", ts.Alternatives.Physics)
	writeAlternatives(&b, "Engineering", ts.Alternatives.Engineering)
	text := b.String()
	for _, want := range []string{"[tech_a] (saved progress: 0, saved date: \"\")", "[tech_b] (saved progress: 1.23456789, saved date: not recorded)", "Physics: not recorded (saved progress: not recorded", "Society: empty", "Engineering: not recorded", "Physics: empty"} {
		if !strings.Contains(text, want) {
			t.Errorf("missing %q in %s", want, text)
		}
	}
}

func TestResearchQueueShowsSavedSpecialProjectReferences(t *testing.T) {
	old := loc
	loc = gamestate.NewLocalizer(t.TempDir())
	t.Cleanup(func() { loc = old })
	gs, err := gamestate.Load([]byte(`country={0={tech_status={physics_queue={{technology=tech_a special_project=0} {technology=tech_b special_project=1 special_project=2} {technology=tech_c}}}}}`))
	if err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	writeQueue(&b, "Physics", gs.Country[0].TechStatus.PhysicsQueue)
	text := b.String()
	if !strings.Contains(text, "special_project reference=0 (use get_special_projects") || !strings.Contains(text, "special_project reference unsupported or duplicate") || strings.Count(text, "special_project reference=") != 1 {
		t.Fatal(text)
	}
}
