package tools

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/bmassemin/stellaris-mcp/internal/clausewitz"
	"github.com/bmassemin/stellaris-mcp/internal/gamestate"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerSystemContents(s *server.MCPServer) {
	s.AddTool(mcp.NewTool("get_system_contents", mcp.WithReadOnlyHintAnnotation(true), mcp.WithDestructiveHintAnnotation(false),
		mcp.WithDescription("Read a saved galactic_object's star class, coordinates, asteroid belts and celestial-body references. Checks body coordinate.origin and reciprocal moon IDs and lists each body's saved deposits. Coordinates, orbit values, survey status and resource yields are not interpreted."),
		mcp.WithString("save_id", mcp.Description("Optional save ID from list_save_games; omit for latest")),
		mcp.WithNumber("system_id", mcp.Required(), mcp.Description("Saved galactic_object ID"))), handleSystemContents)
}

func handleSystemContents(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if _, ok := req.GetArguments()["system_id"]; !ok {
		return toolError(fmt.Errorf("system_id is required")), nil
	}
	if err := requireStringArgs(req, "save_id"); err != nil {
		return toolError(err), nil
	}
	id, err := rawInteger(req, "system_id", 0, 0, maxRecordID)
	if err != nil {
		return toolError(err), nil
	}
	archive, err := openRawSave(saveDir, req.GetString("save_id", ""))
	if err != nil {
		return toolError(err), nil
	}
	data, err := gamestate.LoadSystemContentsFromArchive(archive.zip, id)
	if err != nil {
		return toolError(err), nil
	}
	return renderSystemContents(data, id), nil
}

func renderSystemContents(data *gamestate.SystemContents, id int) *mcp.CallToolResult {
	var b strings.Builder
	fmt.Fprintf(&b, "=== Saved system contents %d ===\n", id)
	writeSavedTableStatus(&b, "galactic_object", data.Systems)
	system, found := data.Systems.Items[id]
	if !found {
		fmt.Fprintf(&b, "System %d: %s\n", id, diplomaticRecordIssue(data.Systems.DecodeErrors[id]))
		return mcp.NewToolResultText(b.String())
	}
	fmt.Fprintf(&b, "System name: %s\n", name(system.Name))
	fmt.Fprintln(&b, "Locations and links describe saved fields. Coordinate units, orbital scales, visibility and resource yields remain unverified.")
	writeSystemFields(&b, "Saved system fields", system.Value, "star_class", "coordinate", "inner_radius", "outer_radius", "initializer", "asteroid_belts", "ambient_object")
	if system.AmbientRefs.Recorded || system.AmbientRefs.Duplicate {
		fmt.Fprintf(&b, "\nAmbient object references: %s\n", surfaceRefText(system.AmbientRefs))
		writeSavedTableStatus(&b, "ambient_object", data.AmbientObjects)
		counts := map[int]int{}
		for _, ambientID := range system.AmbientRefs.Values {
			counts[ambientID]++
		}
		seen := map[int]bool{}
		for _, ambientID := range system.AmbientRefs.Values {
			if seen[ambientID] {
				continue
			}
			seen[ambientID] = true
			if counts[ambientID] > 1 {
				fmt.Fprintf(&b, "  Ambient object %d: %d duplicate saved references; candidate ambiguous\n", ambientID, counts[ambientID])
				continue
			}
			ambient, ok := data.AmbientObjects.Items[ambientID]
			if !ok {
				fmt.Fprintf(&b, "  Ambient object %d: %s\n", ambientID, diplomaticRecordIssue(data.AmbientObjects.DecodeErrors[ambientID]))
				continue
			}
			fmt.Fprintf(&b, "  Ambient object %d candidate: coordinate.origin=%s", ambientID, optionalIntRefText(ambient.CoordinateOrigin))
			if origin := ambient.CoordinateOrigin.Value; origin != nil {
				fmt.Fprintf(&b, " (%s the saved system ID)", compareSavedIDs(*origin, id))
			}
			fmt.Fprintln(&b, "; location and ownership semantics unverified")
			writeSavedValue(&b, "  Saved ambient object fields", ambient.Value, 0)
		}
	}
	if data.NebulaUnresolved > 0 {
		fmt.Fprintf(&b, "Nebula candidate coverage incomplete: %d unsupported saved record or reference fields. Use raw tools.\n", data.NebulaUnresolved)
	}
	for _, candidate := range data.NebulaCandidates {
		fmt.Fprintf(&b, "\nNebula[%d]: %d saved galactic_object ID match(es); candidate only, membership and visibility unverified.\n", candidate.Position, candidate.Matches)
		writeSavedValue(&b, "Saved nebula fields", candidate.Value, 0)
	}
	writeSavedTableStatus(&b, "planets.planet", data.Planets)
	if !data.Deposits.Recorded {
		fmt.Fprintln(&b, "deposit table not recorded")
	}
	if len(system.Bodies) == 0 {
		if system.Bodies == nil {
			fmt.Fprintln(&b, "System planet references not recorded")
		} else {
			fmt.Fprintln(&b, "System planet references empty")
		}
	}
	counts := map[int]int{}
	for _, body := range system.Bodies {
		if body.ID.Value != nil {
			counts[*body.ID.Value]++
		}
	}
	for position, body := range system.Bodies {
		if body.ID.Value == nil {
			fmt.Fprintf(&b, "\nSystem planet reference[%d] unsupported. Use raw tree/byte tools.\n", position)
			writeSavedValue(&b, "Saved planet reference", body.Value, 0)
			continue
		}
		bodyID := *body.ID.Value
		fmt.Fprintf(&b, "\nSystem planet reference[%d]=%d\n", position, bodyID)
		if counts[bodyID] != 1 {
			fmt.Fprintln(&b, "  Duplicate saved planet ID reference. Body link unresolved.")
			continue
		}
		planet, found := data.Planets.Items[bodyID]
		if !found {
			fmt.Fprintf(&b, "  Body %d: %s\n", bodyID, diplomaticRecordIssue(data.Planets.DecodeErrors[bodyID]))
			continue
		}
		fmt.Fprintf(&b, "  Body %d (%s) planet_class=%s owner=%s surveyed=%s surveyed_by=%s\n", bodyID, name(planet.Name), councilSavedString(planet.Class), optionalIntRefText(planet.Owner), councilSavedString(planet.Surveyed), optionalIntRefText(planet.SurveyedBy))
		fmt.Fprintf(&b, "  coordinate.origin=%s", optionalIntRefText(planet.CoordinateOrigin))
		if origin := planet.CoordinateOrigin.Value; origin != nil {
			fmt.Fprintf(&b, " (%s the saved system ID)", compareSavedIDs(*origin, id))
		}
		fmt.Fprintln(&b)
		fmt.Fprintf(&b, "  orbit=%s is_moon=%s moon_of=%s moons=%s (saved fields)\n", councilSavedString(planet.Orbit), councilSavedString(planet.IsMoon), optionalIntRefText(planet.MoonOf), surfaceRefText(planet.Moons))
		writeSystemFields(&b, "  Saved coordinate and orbital fields", planet.Value, "coordinate", "shipclass_orbital_station", "planet_orbitals")
		if parent := planet.MoonOf.Value; parent != nil {
			fmt.Fprintf(&b, "  moon_of %d: %s\n", *parent, systemMoonLink(data, counts, *parent, bodyID))
		}
		for index, child := range planet.Moons.Values {
			fmt.Fprintf(&b, "  moons[%d]=%d: %s\n", index, child, systemMoonLink(data, counts, bodyID, child))
		}
		writePlanetDeposits(&b, data.Deposits, bodyID, planet.Deposits)
	}
	return mcp.NewToolResultText(b.String())
}

func systemMoonLink(data *gamestate.SystemContents, members map[int]int, parentID, childID int) string {
	if parentID == childID {
		return "same saved parent and child ID. Link unresolved"
	}
	for _, id := range []int{parentID, childID} {
		if members[id] == 0 {
			return fmt.Sprintf("body %d absent from this system's saved planet references", id)
		}
		if members[id] > 1 {
			return fmt.Sprintf("body %d has duplicate system references. Link unresolved", id)
		}
	}
	parent, parentFound := data.Planets.Items[parentID]
	child, childFound := data.Planets.Items[childID]
	if !parentFound {
		return fmt.Sprintf("parent %d %s", parentID, diplomaticRecordIssue(data.Planets.DecodeErrors[parentID]))
	}
	if !childFound {
		return fmt.Sprintf("child %d %s", childID, diplomaticRecordIssue(data.Planets.DecodeErrors[childID]))
	}
	if child.MoonOf.Value == nil {
		return "child moon_of=" + optionalIntRefText(child.MoonOf)
	}
	if *child.MoonOf.Value != parentID {
		return fmt.Sprintf("child moon_of=%d differs from the saved parent ID", *child.MoonOf.Value)
	}
	if parent.Moons.Duplicate || parent.Moons.Unsupported || !parent.Moons.Recorded {
		return "parent moons=" + surfaceRefText(parent.Moons)
	}
	count := 0
	for _, id := range parent.Moons.Values {
		if id == childID {
			count++
		}
	}
	switch count {
	case 0:
		return "parent moons list has no matching child ID"
	case 1:
		return "reciprocal saved moon_of and moons IDs agree"
	default:
		return "duplicate child ID references in parent moons list. Link unresolved"
	}
}

func writeSystemFields(b *strings.Builder, label string, value clausewitz.Value, keys ...string) {
	fmt.Fprintln(b, label)
	seen := map[string]bool{}
	for _, pair := range value.Object.Pairs {
		if !slices.Contains(keys, pair.Key) {
			continue
		}
		field := fmt.Sprintf("  key=%q", pair.Key)
		if seen[pair.Key] {
			field += " [duplicate saved key]"
		}
		seen[pair.Key] = true
		writeSavedValue(b, field, pair.Value, 0)
	}
	if len(seen) == 0 {
		fmt.Fprintln(b, "  not recorded")
	}
}
