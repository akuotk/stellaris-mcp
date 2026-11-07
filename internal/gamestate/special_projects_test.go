package gamestate

import (
	"os"
	"testing"
)

func TestSpecialProjectsPreserveRepeatedRecordsAndRawFields(t *testing.T) {
	gs, err := Load([]byte(`country={0={events={
	 special_project={id=0 special_project=example coordinate={type=0 id=0} planet=0 status=0 days_left=-1 scope={from={id=0}}}
	 special_project=none
	 special_project={id=1 debris=0}
	 } tech_status={physics_queue={{special_project=0 progress=0 date=0}}} last_completed_special_project=example}
	 1={events={}} 2={}}`))
	if err != nil {
		t.Fatal(err)
	}
	c := gs.Country[0]
	if !c.SpecialProjects.Recorded || c.SpecialProjects.Unsupported || len(c.SpecialProjects.Items) != 3 {
		t.Fatal("project event order or records lost")
	}
	first, bad, last := c.SpecialProjects.Items[0], c.SpecialProjects.Items[1], c.SpecialProjects.Items[2]
	if first.ID.Value == nil || *first.ID.Value != 0 || first.Planet.Value == nil || *first.Planet.Value != 0 || len(first.Value.Object.Pairs) != 7 || first.Value.Object.Pairs[5].Value.Scalar != "-1" || !first.Value.Object.Pairs[6].Value.IsObject() {
		t.Fatal("zero ID, raw fields or nested scope lost")
	}
	if !bad.Unsupported || bad.Value.Scalar != "none" || last.ID.Value == nil || *last.ID.Value != 1 || last.Planet.Recorded {
		t.Fatal("unsupported or missing field was lost or acquired a default")
	}
	queue := c.TechStatus.PhysicsQueue[0]
	if queue.SpecialProjectRef.Value == nil || *queue.SpecialProjectRef.Value != 0 || queue.ProgressRaw == nil || *queue.ProgressRaw != 0 || queue.DateRaw == nil || *queue.DateRaw != "0" || c.LastCompletedSpecialProject.Value != "example" {
		t.Fatal("research reference or raw queue fields lost")
	}
	if !gs.Country[1].SpecialProjects.Recorded || len(gs.Country[1].SpecialProjects.Items) != 0 || gs.Country[2].SpecialProjects.Recorded {
		t.Fatal("missing and empty event containers conflated")
	}
}

func TestSpecialProjectsAmbiguousFieldsStayIsolated(t *testing.T) {
	gs, err := Load([]byte(`date=2201.01.01 country={
	0={num_sapient_pops=7 events={special_project={id=0 id=1 planet=0 planet=1} special_project={id=bad}}
	 last_completed_special_project=first last_completed_special_project=second
	 tech_status={physics_queue={{special_project=0 special_project=1 technology=example progress=0}}}}
	1={government={type=kept} events=none}
	2={events={special_project={id=0}} events={special_project={id=1}}}
	}`))
	if err != nil || gs.Date != "2201.01.01" {
		t.Fatal("unsupported optional project data prevented whole-save loading")
	}
	c := gs.Country[0]
	p := c.SpecialProjects.Items[0]
	if !p.ID.Unsupported || p.ID.Value != nil || !p.Planet.Unsupported || p.Planet.Value != nil || len(p.Value.Object.Pairs) != 4 || !c.SpecialProjects.Items[1].ID.Unsupported {
		t.Fatal("ambiguous fields formed an authoritative reference or raw pairs were lost")
	}
	q := c.TechStatus.PhysicsQueue[0]
	if !q.SpecialProjectRef.Unsupported || q.SpecialProjectRef.Value != nil || q.TechnologyRaw == nil || *q.TechnologyRaw != "example" || q.ProgressRaw == nil || *q.ProgressRaw != 0 || c.NumPops != 7 || !c.LastCompletedSpecialProject.Unsupported {
		t.Fatal("duplicate queue reference or completion marker affected neighboring data")
	}
	if !gs.Country[1].SpecialProjects.Unsupported || gs.Country[1].Government.Type != "kept" || !gs.Country[2].SpecialProjects.Unsupported || len(gs.Country[2].SpecialProjects.Items) != 0 {
		t.Fatal("invalid or duplicate events created project records")
	}
}

// This opt-in survey reads every local save and logs aggregate counts only.
func TestSpecialProjectsLocalCatalogIntegration(t *testing.T) {
	dir := os.Getenv("STELLARIS_TEST_SAVE_DIR")
	if dir == "" {
		t.Skip("set STELLARIS_TEST_SAVE_DIR for read-only special project coverage")
	}
	files, err := ListSaveFiles(dir)
	if err != nil || len(files) == 0 {
		t.Fatal("local save catalog unavailable or empty")
	}
	var decoded, records, ids, duplicateIDs, planetRefs, planetCandidates, lastMarkers, unsupported int
	var queueRefs, queueCandidates, ambiguousQueueRefs, queueProgress, queueDates int
	fields := map[string]int{}
	for index, file := range files {
		gs, err := LoadFromZip(file.Path)
		if err != nil {
			t.Fatalf("save index %d whole-save decoding failed", index)
		}
		decoded++
		for _, c := range gs.Country {
			if c.SpecialProjects.Unsupported || c.LastCompletedSpecialProject.Unsupported {
				unsupported++
			}
			if c.LastCompletedSpecialProject.Recorded {
				lastMarkers++
			}
			counts := map[int]int{}
			for _, project := range c.SpecialProjects.Items {
				records++
				if project.Unsupported || project.ID.Unsupported || project.Planet.Unsupported {
					unsupported++
				}
				if project.ID.Value != nil {
					ids++
					id := *project.ID.Value
					if counts[id] > 0 {
						duplicateIDs++
					}
					counts[id]++
				}
				if project.Planet.Value != nil {
					planetRefs++
					if _, ok := gs.Planets.Planet[*project.Planet.Value]; ok {
						planetCandidates++
					}
				}
				if project.Value.IsObject() {
					for _, pair := range project.Value.Object.Pairs {
						switch pair.Key {
						case "status", "days_left", "carrier", "debris", "scope":
							fields[pair.Key]++
						}
					}
				}
			}
			for _, queue := range [][]ResearchItem{c.TechStatus.PhysicsQueue, c.TechStatus.SocietyQueue, c.TechStatus.EngineeringQueue} {
				for _, item := range queue {
					if item.SpecialProjectRef.Unsupported {
						unsupported++
					}
					if item.SpecialProjectRef.Value == nil {
						continue
					}
					queueRefs++
					switch count := counts[*item.SpecialProjectRef.Value]; {
					case count == 1:
						queueCandidates++
					case count > 1:
						ambiguousQueueRefs++
					}
					if item.ProgressRaw != nil {
						queueProgress++
					}
					if item.DateRaw != nil {
						queueDates++
					}
				}
			}
		}
	}
	t.Logf("local special project coverage: saves=%d decoded=%d project_records=%d saved_ids=%d duplicate_ids=%d last_markers=%d planet_refs=%d planet_id_candidates=%d queue_refs=%d unique_same_country_id_candidates=%d ambiguous_queue_refs=%d queue_progress_fields=%d queue_date_fields=%d status_fields=%d days_left_fields=%d carrier_fields=%d debris_fields=%d scope_fields=%d unsupported_shapes=%d", len(files), decoded, records, ids, duplicateIDs, lastMarkers, planetRefs, planetCandidates, queueRefs, queueCandidates, ambiguousQueueRefs, queueProgress, queueDates, fields["status"], fields["days_left"], fields["carrier"], fields["debris"], fields["scope"], unsupported)
	if unsupported > 0 {
		t.Fatal("local special project shapes need additional support")
	}
}
