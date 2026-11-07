package gamestate

import (
	"os"
	"reflect"
	"slices"
	"testing"
)

func TestSystemConnectionsPreserveSavedValuesAndOptionalZeros(t *testing.T) {
	gs, err := Load([]byte(`galactic_object={0={hyperlane={{to=0 length=0.0000001 bridge=unknown} {to=1 length=0} {}} bypasses={0 0} natural_wormholes={0}}
	1={hyperlane={}} 2={}}
	bypasses={0={owner={type=raw id=0} type=example active=unknown linked_to=0 is_manual_lock=no lock_country=0 lock_remaining_days=-1 connections={0} active_connections={}} 1=none}
	natural_wormholes={0={bypass=0 coordinate={origin=0 x=123 y=321}} 1={}}`))
	if err != nil {
		t.Fatal(err)
	}
	s := gs.Systems[0]
	if !s.Hyperlanes.Recorded || s.Hyperlanes.Unsupported || len(s.Hyperlanes.Items) != 3 || s.Hyperlanes.Items[0].To == nil || *s.Hyperlanes.Items[0].To != 0 || *s.Hyperlanes.Items[0].Length != "0.0000001" || *s.Hyperlanes.Items[0].Bridge != "unknown" || s.Hyperlanes.Items[2].To != nil {
		t.Fatal("hyperlane order, raw precision or optional zeros lost")
	}
	if !gs.Systems[1].Hyperlanes.Recorded || len(gs.Systems[1].Hyperlanes.Items) != 0 || gs.Systems[2].Hyperlanes.Recorded || !reflect.DeepEqual(s.BypassRefs.Values, []int{0, 0}) || !reflect.DeepEqual(s.WormholeRefs.Values, []int{0}) {
		t.Fatal("missing/empty state or system references lost")
	}
	b := gs.Bypasses.Items[0]
	if !gs.Bypasses.Recorded || len(gs.Bypasses.Items) != 1 || b.Owner == nil || b.Owner.ID == nil || *b.Owner.ID != 0 || b.LinkedTo == nil || *b.LinkedTo != 0 || b.LockCountry == nil || *b.LockCountry != 0 || *b.LockRemainingDays != "-1" || !b.ActiveConnections.Recorded || len(b.ActiveConnections.Values) != 0 {
		t.Fatal("bypass values or optional references lost")
	}
	w := gs.NaturalWormholes.Items[0]
	if w.Bypass == nil || *w.Bypass != 0 || w.Coordinate.Origin == nil || *w.Coordinate.Origin != 0 || gs.NaturalWormholes.Items[1].Bypass != nil || gs.NaturalWormholes.Items[1].Coordinate.Recorded {
		t.Fatal("wormhole or coordinate references lost")
	}
}

func TestSystemConnectionsUnsupportedRecordsStayIsolated(t *testing.T) {
	gs, err := Load([]byte(`date=2201.01.01 galactic_object={0={name={key="Retained system"} planet={0} sector=0
	 hyperlane={{to=0}} hyperlane={{to=1}} bypasses={0} bypasses={1} natural_wormholes={0} natural_wormholes={1}}
	1={hyperlane={{to=invalid} {to=0 to=1} {to=0 length=1 length=2} {to=0}}}
	2={hyperlane={unexpected={}}}}
	bypasses={0={linked_to=0 linked_to=1} 1={owner={id=0 id=1}} 2={} 2=none 3=none 3={} 4={connections={bad}} unknown={}}
	natural_wormholes={0={bypass=0 bypass=1} 1={coordinate={origin=0 origin=1}}}`))
	if err != nil || gs.Date != "2201.01.01" {
		t.Fatal("optional connections prevented whole-save loading")
	}
	s := gs.Systems[0]
	if !s.Hyperlanes.Unsupported || len(s.Hyperlanes.Items) != 0 || !s.BypassRefs.Unsupported || len(s.BypassRefs.Values) != 0 || !s.WormholeRefs.Unsupported || len(s.WormholeRefs.Values) != 0 || s.Name.Key != "Retained system" || len(s.Planets) != 1 || s.SectorRef.Value == nil || *s.SectorRef.Value != 0 {
		t.Fatal("duplicate sections were used or unrelated system fields lost")
	}
	edges := gs.Systems[1].Hyperlanes.Items
	if len(edges) != 4 || !edges[0].Unsupported || edges[0].To != nil || !edges[1].Unsupported || !edges[2].Unsupported || edges[3].To == nil || *edges[3].To != 0 || !gs.Systems[2].Hyperlanes.Unsupported {
		t.Fatal("malformed lane remained usable or neighboring lane lost")
	}
	if !gs.Bypasses.TableIssue || len(gs.Bypasses.DecodeErrors) != 4 || len(gs.Bypasses.Items) != 1 || !gs.Bypasses.Items[4].Connections.Unsupported {
		t.Fatal("invalid or duplicate bypass records accepted")
	}
	if len(gs.NaturalWormholes.DecodeErrors) != 1 || !gs.NaturalWormholes.Items[1].Coordinate.Unsupported || gs.NaturalWormholes.Items[1].Coordinate.Origin != nil {
		t.Fatal("duplicate wormhole references accepted")
	}
}

// This opt-in survey logs aggregate counts only.
func TestSystemConnectionsLocalCatalogIntegration(t *testing.T) {
	dir := os.Getenv("STELLARIS_TEST_SAVE_DIR")
	if dir == "" {
		t.Skip("set STELLARIS_TEST_SAVE_DIR for read-only system connection coverage")
	}
	files, err := ListSaveFiles(dir)
	if err != nil || len(files) == 0 {
		t.Fatal("local save catalog unavailable or empty")
	}
	decoded, laneLists, lanes, reverseLanes, lengthDifferences, unresolvedLaneTargets, selfLanes, unsupported := 0, 0, 0, 0, 0, 0, 0, 0
	bypasses, bypassRefs, resolvedBypasses, linked, reciprocalLinks, connections, reciprocalConnections, active, reciprocalActive := 0, 0, 0, 0, 0, 0, 0, 0, 0
	wormholes, wormholeRefs, matchingOrigins, matchingSystemBypassRefs, matchingOwnerCandidates := 0, 0, 0, 0, 0
	for index, file := range files {
		gs, err := LoadFromZip(file.Path)
		if err != nil {
			t.Fatalf("save index %d whole-save decoding failed", index)
		}
		decoded++
		bypasses += len(gs.Bypasses.Items)
		wormholes += len(gs.NaturalWormholes.Items)
		unsupported += len(gs.Bypasses.DecodeErrors) + len(gs.NaturalWormholes.DecodeErrors)
		if gs.Bypasses.TableIssue || gs.NaturalWormholes.TableIssue {
			unsupported++
		}
		for systemID, system := range gs.Systems {
			if system.Hyperlanes.Recorded {
				laneLists++
			}
			if system.Hyperlanes.Unsupported || system.BypassRefs.Unsupported || system.WormholeRefs.Unsupported {
				unsupported++
			}
			for _, edge := range system.Hyperlanes.Items {
				lanes++
				if edge.Unsupported {
					unsupported++
					continue
				}
				if edge.To == nil {
					unresolvedLaneTargets++
					continue
				}
				if *edge.To == systemID {
					selfLanes++
					continue
				}
				target, ok := gs.Systems[*edge.To]
				if !ok {
					unresolvedLaneTargets++
					continue
				}
				for _, reverse := range target.Hyperlanes.Items {
					if reverse.Unsupported || reverse.To == nil || *reverse.To != systemID {
						continue
					}
					reverseLanes++
					if edge.Length != nil && reverse.Length != nil && *edge.Length != *reverse.Length {
						lengthDifferences++
					}
					break
				}
			}
			for _, id := range system.BypassRefs.Values {
				bypassRefs++
				if _, ok := gs.Bypasses.Items[id]; ok {
					resolvedBypasses++
				}
			}
			for _, id := range system.WormholeRefs.Values {
				wormholeRefs++
				w, ok := gs.NaturalWormholes.Items[id]
				if !ok {
					continue
				}
				if !w.Coordinate.Unsupported && w.Coordinate.Origin != nil && *w.Coordinate.Origin == systemID {
					matchingOrigins++
				}
				if w.Bypass == nil {
					continue
				}
				if slices.Contains(system.BypassRefs.Values, *w.Bypass) {
					matchingSystemBypassRefs++
				}
				if b, ok := gs.Bypasses.Items[*w.Bypass]; ok && b.Owner != nil && b.Owner.ID != nil && *b.Owner.ID == id {
					matchingOwnerCandidates++
				}
			}
		}
		for id, bypass := range gs.Bypasses.Items {
			if bypass.Connections.Unsupported || bypass.ActiveConnections.Unsupported {
				unsupported++
			}
			if bypass.LinkedTo != nil {
				linked++
				if target, ok := gs.Bypasses.Items[*bypass.LinkedTo]; ok && *bypass.LinkedTo != id && target.LinkedTo != nil && *target.LinkedTo == id {
					reciprocalLinks++
				}
			}
			for _, ref := range bypass.Connections.Values {
				connections++
				if target, ok := gs.Bypasses.Items[ref]; ok && ref != id && slices.Contains(target.Connections.Values, id) {
					reciprocalConnections++
				}
			}
			for _, ref := range bypass.ActiveConnections.Values {
				active++
				if target, ok := gs.Bypasses.Items[ref]; ok && ref != id && slices.Contains(target.ActiveConnections.Values, id) {
					reciprocalActive++
				}
			}
		}
		for _, w := range gs.NaturalWormholes.Items {
			if w.Coordinate.Unsupported {
				unsupported++
			}
		}
	}
	t.Logf("local system connection coverage: saves=%d decoded=%d lane_lists=%d lane_entries=%d reciprocal_lanes=%d length_text_differences=%d unresolved_lane_targets=%d self_lanes=%d bypass_records=%d system_bypass_refs=%d resolved_bypass_refs=%d linked_to_refs=%d reciprocal_linked_to=%d connections=%d reciprocal_connections=%d active_connections=%d reciprocal_active_connections=%d wormhole_records=%d system_wormhole_refs=%d matching_origins=%d matching_system_bypass_refs=%d matching_owner_id_candidates=%d unsupported_shapes=%d", len(files), decoded, laneLists, lanes, reverseLanes, lengthDifferences, unresolvedLaneTargets, selfLanes, bypasses, bypassRefs, resolvedBypasses, linked, reciprocalLinks, connections, reciprocalConnections, active, reciprocalActive, wormholes, wormholeRefs, matchingOrigins, matchingSystemBypassRefs, matchingOwnerCandidates, unsupported)
	if unsupported > 0 {
		t.Fatal("local connection shapes need additional support")
	}
}

func TestSystemConnectionDuplicateRootAndStarbaseRefsAreIsolated(t *testing.T) {
	gs, err := Load([]byte(`date="2201.01.01" galactic_object={0={starbases={1} starbases={2} hyperlane={}}} bypasses={0={type=a}} bypasses={1={type=b}} natural_wormholes={} natural_wormholes={0={bypass=0}}`))
	if err != nil {
		t.Fatal(err)
	}
	if gs.Date != "2201.01.01" || !gs.Systems[0].StarbaseRefs.Recorded || !gs.Systems[0].StarbaseRefs.Unsupported || !gs.Systems[0].Hyperlanes.Recorded {
		t.Fatal("duplicate system references not isolated")
	}
	if !gs.Bypasses.Recorded || !gs.Bypasses.TableIssue || len(gs.Bypasses.Items) != 0 || !gs.NaturalWormholes.Recorded || !gs.NaturalWormholes.TableIssue || len(gs.NaturalWormholes.Items) != 0 {
		t.Fatal("duplicate root tables not flagged")
	}
}
