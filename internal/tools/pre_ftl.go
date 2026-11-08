package tools

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/bmassemin/stellaris-mcp/internal/gamestate"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerPreFTL(s *server.MCPServer) {
	s.AddTool(mcp.NewTool("get_pre_ftl", mcp.WithReadOnlyHintAnnotation(true), mcp.WithDestructiveHintAnnotation(false),
		mcp.WithDescription("Read save-wide country records with type=primitive or a saved preftl_age field, and fleets with ship_class=shipclass_observation_station. Shows raw age and awareness, capital candidates, reciprocal planet/orbit links and saved fleet and ship references. Age, awareness units and present visibility are unverified."),
		mcp.WithString("save_id", mcp.Description("Optional save ID from list_save_games; omit for latest"))), handlePreFTL)
}

func handlePreFTL(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if value, ok := req.GetArguments()["save_id"]; ok {
		if _, ok := value.(string); !ok {
			return toolError(fmt.Errorf("save_id must be a string")), nil
		}
	}
	archive, err := openRawSave(saveDir, req.GetString("save_id", ""))
	if err != nil {
		return toolError(err), nil
	}
	data, err := gamestate.LoadPreFTLFromArchive(archive.zip)
	if err != nil {
		return toolError(err), nil
	}
	return renderPreFTL(data), nil
}

func renderPreFTL(data *gamestate.PreFTLState) *mcp.CallToolResult {
	var b strings.Builder
	fmt.Fprintln(&b, "=== Saved pre-FTL markers and observation fleets (save-wide) ===")
	fmt.Fprintln(&b, "Country scope uses type=primitive or the presence of preftl_age. The saved age marker can coexist with other country types. Awareness scale and present visibility remain unverified.")
	writePreFTLTableStatus(&b, "country", data.Countries)
	writePreFTLTableStatus(&b, "planets.planet", data.Planets)
	writePreFTLTableStatus(&b, "fleet", data.Fleets)
	writePreFTLTableStatus(&b, "ships", data.Ships)
	var countryIDs, observationIDs []int
	for id, country := range data.Countries.Items {
		if country.Relevant() {
			countryIDs = append(countryIDs, id)
		}
	}
	for id, fleet := range data.Fleets.Items {
		if fleet.Recorded {
			observationIDs = append(observationIDs, id)
		}
	}
	sort.Ints(countryIDs)
	sort.Ints(observationIDs)
	fmt.Fprintln(&b, "\nSaved country markers")
	if len(countryIDs) == 0 {
		fmt.Fprintln(&b, "  No decoded country records with the selected markers")
	}
	for _, id := range countryIDs {
		c := data.Countries.Items[id]
		fmt.Fprintf(&b, "  Country %d (%s) type=%s preftl_age=%s awareness=%s old_awareness=%s\n", id, name(c.Name), councilSavedString(c.Type), councilSavedString(c.Age), councilSavedString(c.Awareness), councilSavedString(c.OldAwareness))
		fmt.Fprintf(&b, "    capital=%s (saved reference)\n", preFTLRefText(c.Capital))
		if c.Capital.Value == nil {
			continue
		}
		planet, found := writePreFTLPlanet(&b, data, "Capital", *c.Capital.Value)
		if !found {
			continue
		}
		for _, field := range []struct {
			name string
			ref  gamestate.OptionalIntRef
		}{{"owner", planet.Owner}, {"controller", planet.Controller}} {
			if field.ref.Value != nil {
				fmt.Fprintf(&b, "    Planet %s numeric ID %s the country ID\n", field.name, preFTLIDComparison(*field.ref.Value, id))
			}
		}
	}
	fmt.Fprintln(&b, "\nSaved observation fleet markers")
	if len(observationIDs) == 0 {
		fmt.Fprintln(&b, "  No decoded observation fleet markers")
		return mcp.NewToolResultText(b.String())
	}
	owners := map[int][]int{}
	planetRefs := map[int][]int{}
	ambiguousManagers := 0
	for countryID, c := range data.Countries.Items {
		if c.Fleets.OwnershipAmbiguous {
			ambiguousManagers++
			continue
		}
		for _, entry := range c.Fleets.OwnedFleets {
			if entry.FleetID != nil && data.Fleets.Items[*entry.FleetID].Recorded {
				owners[*entry.FleetID] = append(owners[*entry.FleetID], countryID)
			}
		}
	}
	if ambiguousManagers > 0 {
		fmt.Fprintf(&b, "  Saved owned_fleets scope incomplete for %d countries with unsupported or duplicate fields\n", ambiguousManagers)
	}
	for planetID, p := range data.Planets.Items {
		if p.OrbitalStation.Value != nil && data.Fleets.Items[*p.OrbitalStation.Value].Recorded {
			planetRefs[*p.OrbitalStation.Value] = append(planetRefs[*p.OrbitalStation.Value], planetID)
		}
	}
	for _, id := range observationIDs {
		writeObservationFleet(&b, data, id, owners[id], planetRefs[id])
	}
	return mcp.NewToolResultText(b.String())
}

func writePreFTLTableStatus[T any](b *strings.Builder, label string, table gamestate.ConnectionRecordTable[T]) {
	if !table.Recorded {
		fmt.Fprintf(b, "%s table not recorded\n", label)
	}
	if table.TableIssue || len(table.DecodeErrors) > 0 {
		fmt.Fprintf(b, "%s table incomplete, unsupported or duplicate. Affected records=%d. Use raw tree/byte tools.\n", label, len(table.DecodeErrors))
	}
}

func writePreFTLPlanet(b *strings.Builder, data *gamestate.PreFTLState, label string, id int) (gamestate.PreFTLPlanet, bool) {
	planet, found := data.Planets.Items[id]
	if !found {
		_, unsupported := data.Planets.DecodeErrors[id]
		fmt.Fprintf(b, "    %s planet ID candidate %d (%s)\n", label, id, preFTLUnavailable(unsupported))
		return planet, false
	}
	fmt.Fprintf(b, "    %s planet ID candidate %d (%s) owner=%s controller=%s shipclass_orbital_station=%s\n", label, id, name(planet.Name), preFTLRefText(planet.Owner), preFTLRefText(planet.Controller), preFTLRefText(planet.OrbitalStation))
	if station := planet.OrbitalStation.Value; station != nil {
		if _, found := data.Fleets.Items[*station]; !found {
			_, unsupported := data.Fleets.DecodeErrors[*station]
			fmt.Fprintf(b, "    Orbital-station fleet ID candidate %d (%s)\n", *station, preFTLUnavailable(unsupported))
		}
	}
	return planet, true
}

func writeObservationFleet(b *strings.Builder, data *gamestate.PreFTLState, id int, owners, planetRefs []int) {
	fleet := data.Fleets.Items[id]
	fmt.Fprintf(b, "  Fleet %d (%s) ship_class=%s\n", id, name(fleet.Name), councilSavedString(fleet.Class))
	if fleet.Class.Unsupported {
		fmt.Fprintln(b, "    Observation class evidence is ambiguous. Use raw tree/byte tools.")
		return
	}
	sort.Ints(owners)
	sort.Ints(planetRefs)
	switch len(owners) {
	case 0:
		fmt.Fprintln(b, "    No decoded country owned_fleets references")
	case 1:
		fmt.Fprintf(b, "    Saved owned_fleets reference from country %d (%s)\n", owners[0], name(data.Countries.Items[owners[0]].Name))
	default:
		fmt.Fprintf(b, "    Ambiguous saved owned_fleets references from country IDs %v\n", owners)
	}
	if len(planetRefs) > 0 {
		fmt.Fprintf(b, "    Planets with matching saved shipclass_orbital_station IDs: %v\n", planetRefs)
	}
	if len(planetRefs) > 1 {
		fmt.Fprintln(b, "    Ambiguous planet reverse references")
	}
	fmt.Fprintf(b, "    movement_manager.coordinate.origin=%s orbit.orbitable.planet=%s (saved fields)\n", preFTLRefText(fleet.Origin), preFTLRefText(fleet.OrbitPlanet))
	if fleet.OrbitPlanet.Unsupported && fleet.OrbitableSaved {
		writeSavedValue(b, "    Saved orbitable", fleet.Orbitable, 0)
	}
	if fleet.OrbitPlanet.Value != nil {
		planet, found := writePreFTLPlanet(b, data, "Orbit", *fleet.OrbitPlanet.Value)
		if found {
			if station := planet.OrbitalStation.Value; station != nil {
				if *station != id {
					fmt.Fprintln(b, "    Saved planet orbital-station ID differs from the fleet ID")
				} else if len(planetRefs) == 1 {
					fmt.Fprintln(b, "    Reciprocal saved fleet orbit and planet orbital-station IDs agree")
				}
			}
			if planet.Owner.Value != nil {
				ownerID := *planet.Owner.Value
				if c, ok := data.Countries.Items[ownerID]; ok {
					fmt.Fprintf(b, "    Orbit planet owner country ID candidate %d (%s) type=%s preftl_age=%s\n", ownerID, name(c.Name), councilSavedString(c.Type), councilSavedString(c.Age))
				} else {
					_, unsupported := data.Countries.DecodeErrors[ownerID]
					fmt.Fprintf(b, "    Orbit planet owner country ID candidate %d (%s)\n", ownerID, preFTLUnavailable(unsupported))
				}
				if len(owners) == 1 {
					fmt.Fprintf(b, "    Planet owner ID %s the country ID with the saved owned_fleets reference\n", preFTLIDComparison(ownerID, owners[0]))
				}
			}
		}
	}
	writeObservationShips(b, data, id, fleet.Ships)
}

func writeObservationShips(b *strings.Builder, data *gamestate.PreFTLState, fleetID int, refs gamestate.OptionalIntList) {
	switch {
	case refs.Unsupported:
		fmt.Fprintln(b, "    ships unsupported or duplicate")
	case !refs.Recorded:
		fmt.Fprintln(b, "    ships not recorded")
	case len(refs.Values) == 0:
		fmt.Fprintln(b, "    ships empty")
	}
	counts := map[int]int{}
	for _, id := range refs.Values {
		counts[id]++
	}
	for position, id := range refs.Values {
		fmt.Fprintf(b, "    ships[%d]=%d", position, id)
		ship, found := data.Ships.Items[id]
		_, unsupported := data.Ships.DecodeErrors[id]
		switch {
		case counts[id] > 1:
			fmt.Fprintln(b, " (duplicate saved ship ID reference)")
		case !found:
			fmt.Fprintf(b, " (%s)\n", preFTLUnavailable(unsupported))
		case ship.Fleet.Value == nil:
			fmt.Fprintf(b, " ship.fleet=%s\n", preFTLRefText(ship.Fleet))
		default:
			fmt.Fprintf(b, " ship.fleet=%d (%s the saved fleet ID)\n", *ship.Fleet.Value, preFTLIDComparison(*ship.Fleet.Value, fleetID))
		}
	}
}

func preFTLRefText(ref gamestate.OptionalIntRef) string {
	if ref.Unsupported {
		return "unsupported or duplicate"
	}
	return armyInt(ref.Value)
}

func preFTLUnavailable(unsupported bool) string {
	if unsupported {
		return "record unsupported or duplicate"
	}
	return "record unavailable"
}

func preFTLIDComparison(a, b int) string {
	if a == b {
		return "agrees with"
	}
	return "differs from"
}
