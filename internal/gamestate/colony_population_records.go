package gamestate

import (
	"archive/zip"
	"context"
	"sort"
	"strconv"

	"github.com/bmassemin/stellaris-mcp/internal/clausewitz"
)

const ColonyPopulationPageSize = 20

type PopulationReferenceListStatus struct {
	Status      string `json:"status"`
	Occurrences int    `json:"field_occurrences"`
	Unsupported int    `json:"unsupported_rows"`
}

type ColonyPopulationRow struct {
	Path, ContainerPath, Role string
	Node                      *clausewitz.RawNode
	Index                     int
	Reference                 *CountryExplorationCandidate
	ReferenceOccurrences      int
	Backref                   string
}

type SavedColonyPopulationRecords struct {
	CountryID, EntryIndex, RowCount, OwnedOccurrences int
	CountryPath, Schema, ViewStatus, PlanetLink       string
	RecordBackref                                     string
	RecordListOccurrences                             int
	RecordReverseObserved                             bool
	Ownership                                         PopulationReferenceListStatus
	LocationTable                                     CountryExplorationTable
	Location, Planet, Record                          *CountryExplorationCandidate
	PopulationTable                                   *CountryExplorationTable
	PopulationList                                    *PopulationReferenceListStatus
	OwnedPaths                                        []string
	Rows                                              []ColonyPopulationRow
	offset                                            int
}

type populationListEntry struct {
	path, parent string
	index        int
	node         *clausewitz.RawNode
	id           *int
	role         string
}

type populationList struct {
	PopulationReferenceListStatus
	rows []populationListEntry
	ids  map[int]int
}

func LoadColonyPopulationRecordsFromArchive(ctx context.Context, archive *zip.Reader, requestedCountry *int, locationID int, view string, recordID *int, offset int) (SavedColonyPopulationRecords, error) {
	saved, err := loadRawSavedCountry(ctx, archive, requestedCountry, false)
	if err != nil {
		return SavedColonyPopulationRecords{}, err
	}
	owned := readPopulationList(saved.Node, saved.Path, "owned_planets")
	out := SavedColonyPopulationRecords{
		CountryID: saved.CountryID, EntryIndex: saved.EntryIndex, CountryPath: saved.Path,
		Ownership: owned.PopulationReferenceListStatus, OwnedOccurrences: owned.ids[locationID],
		PlanetLink: "not-checked", RecordBackref: "not-checked",
		Rows: make([]ColonyPopulationRow, 0, ColonyPopulationPageSize), offset: offset,
	}
	for _, row := range owned.rows {
		if row.id != nil && *row.id == locationID && len(out.OwnedPaths) < ColonyPopulationPageSize {
			out.OwnedPaths = append(out.OwnedPaths, row.path)
		}
	}
	out.Schema = "colony"
	out.LocationTable = explorationTable(saved.root, "colony", "colony")
	if out.LocationTable.Status == "not-recorded" {
		out.Schema = "legacy-planet"
		out.LocationTable = explorationTable(saved.root, "planet", "planets", "planet")
	} else if out.LocationTable.Status == "duplicate" || out.LocationTable.Status == "unsupported" || out.LocationTable.Occurrences == 0 {
		out.Schema = "unresolved"
	}
	if out.OwnedOccurrences == 0 {
		out.ViewStatus = "location-not-observed-in-owned-planets"
		return out, nil
	}
	candidate := out.LocationTable.candidate(locationID)
	out.Location = &candidate
	if candidate.RecordStatus != "object" {
		out.ViewStatus = "location-" + candidate.Status
		out.addCandidateRows(out.LocationTable, locationID)
		return out, nil
	}
	location := out.LocationTable.byID[locationID][0].node
	if out.Schema == "legacy-planet" {
		if _, state := populationInteger(location, "colony"); state != "not-recorded" {
			out.Schema, out.ViewStatus = "unresolved", "colony-reference-without-colony-table"
			out.addCandidateRows(out.LocationTable, locationID)
			return out, nil
		}
		out.PlanetLink = "owner-" + populationNumberMatch(location, "owner", saved.CountryID)
		if view != "growth" {
			out.ViewStatus = "legacy-view-unavailable"
			return out, nil
		}
	} else {
		out.linkPlanet(saved.root, location, locationID)
	}
	out.ViewStatus = "recorded"
	if view == "growth" {
		err := out.walkFields(ctx, location, candidate.Path, func(key string) bool {
			if out.Schema == "legacy-planet" {
				switch key {
				case "migration", "growth", "growth_species_ref", "pop_assembly", "pop", "jobs_cache", "decline", "declining_pop":
					return true
				}
				return false
			}
			return key == "current_month_growth_data" || key == "last_month_growth_data"
		})
		if err == nil && out.RowCount == 0 {
			out.ViewStatus = "growth-fields-not-recorded"
		}
		return out, err
	}
	field := "pop_groups"
	if view == "pop_jobs" || view == "job_fields" {
		field = "pop_jobs"
	}
	refs := readPopulationList(location, candidate.Path, field)
	out.PopulationList = &refs.PopulationReferenceListStatus
	popTable := explorationTable(saved.root, field, field)
	out.PopulationTable = &popTable
	switch {
	case refs.Status == "not-recorded" && popTable.Status == "not-recorded":
		out.ViewStatus = "population-data-not-recorded"
	case (refs.Status != "recorded" && refs.Status != "empty") || popTable.Status != "recorded":
		out.ViewStatus = "partial"
	}
	var viewErr error
	if recordID != nil {
		viewErr = out.readRecordFields(ctx, popTable, refs, locationID, *recordID)
	} else {
		viewErr = out.readPopulationRows(ctx, popTable, refs, locationID)
	}
	return out, viewErr
}

func (out *SavedColonyPopulationRecords) linkPlanet(root, colony *clausewitz.RawNode, colonyID int) {
	carrier, state := populationField(colony, "carrier")
	if state != "unique" || carrier.Kind != "object" {
		out.PlanetLink = "carrier-" + state
		if state == "unique" {
			out.PlanetLink = "carrier-unsupported"
		}
		return
	}
	kind, state := populationField(carrier, "type")
	if state != "unique" || !kind.IsScalar() {
		out.PlanetLink = "carrier-type-" + state
		if state == "unique" {
			out.PlanetLink = "carrier-type-unsupported"
		}
		return
	}
	if kind.Scalar.Text != "planet" {
		out.PlanetLink = "nonplanet-carrier"
		return
	}
	planetID, state := populationInteger(carrier, "reference")
	if state != "integer" {
		out.PlanetLink = "carrier-reference-" + state
		return
	}
	planets := explorationTable(root, "planet", "planets", "planet")
	candidate := planets.candidate(planetID)
	out.Planet = &candidate
	if candidate.RecordStatus != "object" {
		out.PlanetLink = "planet-" + candidate.Status
		return
	}
	planet := planets.byID[planetID][0].node
	out.PlanetLink = populationNumberMatch(planet, "colony", colonyID)
	if out.PlanetLink == "same-number" {
		out.PlanetLink = "reciprocal"
		if candidate.Status == "observed" || out.Location.Status == "observed" {
			out.PlanetLink = "observed-reciprocal"
		}
	}
}

func readPopulationList(node *clausewitz.RawNode, path, field string) populationList {
	out := populationList{PopulationReferenceListStatus: PopulationReferenceListStatus{Status: "not-recorded"}, ids: map[int]int{}}
	for i, wrapper := range node.Children {
		if wrapper.Key == nil || wrapper.Key.Text != field {
			continue
		}
		out.Occurrences++
		wrapperPath := path + "/" + strconv.Itoa(i)
		out.Status = "recorded"
		switch wrapper.Kind {
		case "empty_block":
			out.Status = "empty"
			out.rows = append(out.rows, populationListEntry{wrapperPath, path, i, wrapper, nil, "empty-list"})
		case "list", "mixed":
			if wrapper.Kind == "mixed" {
				out.Status = "partial"
			}
			for j, child := range wrapper.Children {
				entry := populationListEntry{wrapperPath + "/" + strconv.Itoa(j), wrapperPath, j, child, nil, "unsupported-reference"}
				if child.Key == nil && child.IsScalar() {
					if id, err := strconv.Atoi(child.Scalar.Text); err == nil && id >= 0 {
						entry.id, entry.role = &id, "listed-reference"
						out.ids[id]++
					}
				}
				if entry.id == nil {
					out.Unsupported++
					out.Status = "partial"
				}
				out.rows = append(out.rows, entry)
			}
		default:
			out.Status = "unsupported"
			out.Unsupported++
			out.rows = append(out.rows, populationListEntry{wrapperPath, path, i, wrapper, nil, "unsupported-list"})
		}
	}
	if out.Occurrences > 1 {
		out.Status = "duplicate"
	}
	return out
}

func populationField(node *clausewitz.RawNode, key string) (*clausewitz.RawNode, string) {
	var field *clausewitz.RawNode
	for _, child := range node.Children {
		if child.Key != nil && child.Key.Text == key {
			if field != nil {
				return nil, "duplicate"
			}
			field = child
		}
	}
	if field == nil {
		return nil, "not-recorded"
	}
	return field, "unique"
}

func populationInteger(node *clausewitz.RawNode, key string) (int, string) {
	field, state := populationField(node, key)
	if state != "unique" {
		return 0, state
	}
	if field.IsScalar() {
		if id, err := strconv.Atoi(field.Scalar.Text); err == nil && id >= 0 {
			return id, "integer"
		}
	}
	return 0, "unsupported"
}

func populationNumberMatch(node *clausewitz.RawNode, key string, expected int) string {
	id, state := populationInteger(node, key)
	if state != "integer" {
		return state
	}
	if id == expected {
		return "same-number"
	}
	return "different-number"
}

func populationReverseObserved(node *clausewitz.RawNode, locationID int) bool {
	for _, field := range node.Children {
		if field.Key != nil && field.Key.Text == "planet" && field.IsScalar() {
			if id, err := strconv.Atoi(field.Scalar.Text); err == nil && id == locationID {
				return true
			}
		}
	}
	return false
}

func (out *SavedColonyPopulationRecords) readPopulationRows(ctx context.Context, table CountryExplorationTable, list populationList, locationID int) error {
	for _, entry := range list.rows {
		if err := ctx.Err(); err != nil {
			return err
		}
		if out.inPage() {
			row := ColonyPopulationRow{Path: entry.path, ContainerPath: entry.parent, Index: entry.index, Node: entry.node, Role: entry.role}
			if entry.id != nil {
				candidate := table.candidate(*entry.id)
				row.Reference = &candidate
				row.ReferenceOccurrences = list.ids[*entry.id]
				row.Backref = "unresolved"
				if candidate.RecordStatus == "object" {
					row.Backref = populationNumberMatch(table.byID[*entry.id][0].node, "planet", locationID)
				}
			}
			out.Rows = append(out.Rows, row)
		}
		out.RowCount++
	}
	type reverseEntry struct {
		id     int
		record explorationCandidateRecord
	}
	var reverse []reverseEntry
	for id, records := range table.byID {
		if err := ctx.Err(); err != nil {
			return err
		}
		if list.ids[id] > 0 {
			continue
		}
		for _, record := range records {
			if populationReverseObserved(record.node, locationID) {
				reverse = append(reverse, reverseEntry{id, record})
			}
		}
	}
	sort.Slice(reverse, func(i, j int) bool { return reverse[i].record.index < reverse[j].record.index })
	for _, entry := range reverse {
		if out.inPage() {
			candidate := table.candidate(entry.id)
			out.Rows = append(out.Rows, ColonyPopulationRow{
				Path: table.Path + "/" + strconv.Itoa(entry.record.index), ContainerPath: table.Path,
				Index: entry.record.index, Node: entry.record.node, Role: "reverse-only-candidate", Reference: &candidate,
				Backref: populationNumberMatch(entry.record.node, "planet", locationID),
			})
		}
		out.RowCount++
	}
	return nil
}

func (out *SavedColonyPopulationRecords) readRecordFields(ctx context.Context, table CountryExplorationTable, list populationList, locationID, recordID int) error {
	records := table.byID[recordID]
	out.RecordListOccurrences = list.ids[recordID]
	for _, record := range records {
		out.RecordReverseObserved = out.RecordReverseObserved || populationReverseObserved(record.node, locationID)
	}
	if out.RecordListOccurrences == 0 && !out.RecordReverseObserved {
		out.ViewStatus = "record-not-observed-for-location"
		return nil
	}
	candidate := table.candidate(recordID)
	out.Record = &candidate
	if candidate.RecordStatus != "object" {
		out.ViewStatus = "record-" + candidate.Status
		out.RecordBackref = "unresolved"
		out.addCandidateRows(table, recordID)
		return nil
	}
	out.RecordBackref = populationNumberMatch(records[0].node, "planet", locationID)
	return out.walkFields(ctx, records[0].node, candidate.Path, nil)
}

func (out *SavedColonyPopulationRecords) addCandidateRows(table CountryExplorationTable, id int) {
	for _, record := range table.byID[id] {
		if out.inPage() {
			out.Rows = append(out.Rows, ColonyPopulationRow{
				Path: table.Path + "/" + strconv.Itoa(record.index), ContainerPath: table.Path,
				Index: record.index, Node: record.node, Role: "record-candidate",
			})
		}
		out.RowCount++
	}
}

func (out *SavedColonyPopulationRecords) inPage() bool {
	return out.RowCount >= out.offset && len(out.Rows) < ColonyPopulationPageSize
}

func (out *SavedColonyPopulationRecords) walkFields(ctx context.Context, root *clausewitz.RawNode, path string, include func(string) bool) error {
	type frame struct {
		node *clausewitz.RawNode
		next int
	}
	stack := []frame{{node: root}}
	var buffer []byte
	for len(stack) > 0 {
		parent := &stack[len(stack)-1]
		if parent.next == len(parent.node.Children) {
			stack = stack[:len(stack)-1]
			continue
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		i := parent.next
		child := parent.node.Children[i]
		parent.next++
		if len(stack) == 1 && include != nil && (child.Key == nil || !include(child.Key.Text)) {
			continue
		}
		if out.inPage() {
			buffer = append(buffer[:0], path...)
			for _, ancestor := range stack[:len(stack)-1] {
				buffer = append(buffer, '/')
				buffer = strconv.AppendInt(buffer, int64(ancestor.next-1), 10)
			}
			container := string(buffer)
			out.Rows = append(out.Rows, ColonyPopulationRow{
				Path: container + "/" + strconv.Itoa(i), ContainerPath: container,
				Index: i, Node: child, Role: "saved-field",
			})
		}
		out.RowCount++
		if len(child.Children) > 0 {
			stack = append(stack, frame{node: child})
		}
	}
	return nil
}
