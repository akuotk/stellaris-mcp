package gamestate

import (
	"archive/zip"
	"fmt"
	"strconv"
	"strings"

	"github.com/bmassemin/stellaris-mcp/internal/clausewitz"
)

// LocationConstruction follows saved planet and colony queue fields into the construction tables.
// Matches are numeric; queue location codes, ownership and buildability are not interpreted.
type LocationConstruction struct {
	PlanetID, ColonyID     *int
	Planet, Colony         LoadoutRecord
	PlanetColony           OptionalIntRef // planet.colony
	CarrierType            OptionalString // colony.carrier.type
	CarrierRef             OptionalIntRef // colony.carrier.reference
	QueueTableIssue        string
	ItemTableIssue         string
	Refs                   []LocationQueueRef
	UnreferencedQueues     []LocationQueue
	UnreferencedItems      []LocationItem
	UnreferencedQueueCount int
	UnreferencedItemCount  int
	// Location types of the queues reached through saved fields; other queues are listed only
	// when their location.type is one of these. Otherwise they are only counted.
	ReferencedLocationTypes []int
	OtherTypeQueueCount     int
	// buildable_*.planet holds colony IDs where a colony table exists (observed in 4.x saves)
	// and planet IDs otherwise; ItemLocationTable names the table compared.
	ItemLocationTable string
	ItemLocationID    *int
	ColonyTableIssue  string
}

const LocationConstructionPreviewLimit = 20

type LocationQueueRef struct {
	Source string // saved path of the reference
	Key    *OptionalIntRef
	Ref    OptionalIntRef
	Queue  *LocationQueue
}

type LocationQueue struct {
	ID                    int
	Record                LoadoutRecord
	Type, Disabled        OptionalString
	Simultaneous          OptionalString
	Owner, LocType, LocID OptionalIntRef
	Items                 []LocationItem
	ItemsRecorded         bool
	ItemsUnsupported      bool
	LocationMatchesPlanet bool
}

type LocationItem struct {
	ID            int
	Record        LoadoutRecord
	Queue         OptionalIntRef
	PayingCountry OptionalIntRef
	Progress      OptionalString
	Needed        OptionalString
	Buildables    []clausewitz.Pair
	PlanetFields  []OptionalIntRef // buildable_*.planet in saved order
}

func LoadLocationConstructionFromArchive(archive *zip.Reader, planetID, colonyID *int) (*LocationConstruction, error) {
	data, err := extractGamestateEntry(archive)
	if err != nil {
		return nil, err
	}
	root, err := clausewitz.Parse(data)
	if err != nil {
		return nil, fmt.Errorf("parse gamestate: %w", err)
	}
	return decodeLocationConstruction(clausewitz.Value{Object: root}, planetID, colonyID), nil
}

type savedIndex struct {
	byID  map[int][]clausewitz.Value
	order []int
	issue string
}

func indexSavedTable(root clausewitz.Value, path ...string) savedIndex {
	table, recorded, unsupported := savedFieldPath(root, path...)
	switch {
	case !recorded:
		return savedIndex{issue: "table not recorded"}
	case unsupported || !table.IsObject():
		return savedIndex{issue: "table unsupported or duplicate"}
	}
	index := savedIndex{byID: map[int][]clausewitz.Value{}}
	for _, pair := range table.Object.Pairs {
		id, err := strconv.Atoi(pair.Key)
		if err != nil {
			continue
		}
		if _, seen := index.byID[id]; !seen {
			index.order = append(index.order, id)
		}
		index.byID[id] = append(index.byID[id], pair.Value)
	}
	return index
}

func (index savedIndex) record(id int) LoadoutRecord {
	if index.issue != "" {
		return LoadoutRecord{Issue: index.issue}
	}
	values := index.byID[id]
	var record LoadoutRecord
	switch {
	case len(values) == 0:
		record.Issue = "record unavailable"
	case len(values) > 1:
		record.Issue = "duplicate saved record ID"
	case values[0].IsScalar() && values[0].Scalar == "none":
		record.Issue = `saved record is "none"`
	case !values[0].IsObject():
		record.Issue = "record shape unsupported"
	default:
		record.Value = values[0]
	}
	return record
}

func decodeLocationConstruction(root clausewitz.Value, planetID, colonyID *int) *LocationConstruction {
	out := &LocationConstruction{PlanetID: planetID, ColonyID: colonyID}
	planets := indexSavedTable(root, "planets", "planet")
	colonies := indexSavedTable(root, "colony")
	queues := indexSavedTable(root, "construction", "queue_mgr", "queues")
	items := indexSavedTable(root, "construction", "item_mgr", "items")
	out.QueueTableIssue, out.ItemTableIssue, out.ColonyTableIssue = queues.issue, items.issue, colonies.issue
	if planetID != nil {
		out.Planet = planets.record(*planetID)
		if out.Planet.Issue == "" {
			out.PlanetColony = savedPathInt(out.Planet.Value, "colony")
			if id := out.PlanetColony.Value; id != nil && colonyID == nil {
				out.ColonyID = id
			}
		}
	}
	if out.ColonyID != nil {
		out.Colony = colonies.record(*out.ColonyID)
		if out.Colony.Issue == "" {
			out.CarrierType = savedFieldStringPath(out.Colony.Value, "carrier", "type")
			out.CarrierRef = savedPathInt(out.Colony.Value, "carrier", "reference")
			if planetID == nil && out.CarrierType.Value == "planet" && !out.CarrierType.Unsupported && out.CarrierRef.Value != nil {
				out.PlanetID = out.CarrierRef.Value
				out.Planet = planets.record(*out.PlanetID)
				if out.Planet.Issue == "" {
					out.PlanetColony = savedPathInt(out.Planet.Value, "colony")
				}
			}
		}
	}
	if out.PlanetID != nil && out.Planet.Issue == "" {
		out.Refs = append(out.Refs, locationQueueRefs(out.Planet.Value, fmt.Sprintf("planet %d", *out.PlanetID))...)
	}
	if out.ColonyID != nil && out.Colony.Issue == "" {
		out.Refs = append(out.Refs, locationQueueRefs(out.Colony.Value, fmt.Sprintf("colony %d", *out.ColonyID))...)
	}
	if colonies.issue == "" {
		out.ItemLocationTable = "colony"
		if out.ColonyID != nil && out.Colony.Issue == "" {
			out.ItemLocationID = out.ColonyID
		}
	} else {
		out.ItemLocationTable, out.ItemLocationID = "planet", out.PlanetID
	}
	listed := map[int]bool{}
	listedItems := map[int]bool{}
	locTypes := map[int]bool{}
	for i := range out.Refs {
		ref := &out.Refs[i]
		if ref.Ref.Value == nil || ref.Ref.Unsupported {
			continue
		}
		queue := decodeLocationQueue(queues, items, *ref.Ref.Value)
		if out.PlanetID != nil && queue.LocID.Value != nil && *queue.LocID.Value == *out.PlanetID {
			queue.LocationMatchesPlanet = true
		}
		ref.Queue = &queue
		listed[queue.ID] = true
		if queue.LocationMatchesPlanet && queue.LocType.Value != nil && !locTypes[*queue.LocType.Value] {
			locTypes[*queue.LocType.Value] = true
			out.ReferencedLocationTypes = append(out.ReferencedLocationTypes, *queue.LocType.Value)
		}
		for _, item := range queue.Items {
			listedItems[item.ID] = true
		}
	}
	if queues.issue != "" {
		return out
	}
	for _, id := range queues.order {
		if out.PlanetID == nil {
			// Queue location IDs are planet IDs; without a planet only items are scanned.
			break
		}
		if listed[id] {
			continue
		}
		record := queues.record(id)
		if record.Issue != "" {
			continue
		}
		if loc := savedPathInt(record.Value, "location", "id"); loc.Value != nil && *loc.Value == *out.PlanetID {
			if len(locTypes) > 0 {
				if typ := savedPathInt(record.Value, "location", "type"); typ.Value == nil || !locTypes[*typ.Value] {
					out.OtherTypeQueueCount++
					continue
				}
			}
			queue := decodeLocationQueue(queues, items, id)
			for _, item := range queue.Items {
				listedItems[item.ID] = true
			}
			out.UnreferencedQueueCount++
			if len(out.UnreferencedQueues) < LocationConstructionPreviewLimit {
				out.UnreferencedQueues = append(out.UnreferencedQueues, queue)
			}
		}
	}
	if items.issue != "" || out.ItemLocationID == nil {
		return out
	}
	for _, id := range items.order {
		if listedItems[id] {
			continue
		}
		item := decodeLocationItem(items, id)
		for _, planet := range item.PlanetFields {
			if planet.Value != nil && *planet.Value == *out.ItemLocationID {
				out.UnreferencedItemCount++
				if len(out.UnreferencedItems) < LocationConstructionPreviewLimit {
					out.UnreferencedItems = append(out.UnreferencedItems, item)
				}
				break
			}
		}
	}
	return out
}

// locationQueueRefs lists queue-valued fields of a planet or colony record in saved order.
func locationQueueRefs(value clausewitz.Value, owner string) []LocationQueueRef {
	var refs []LocationQueueRef
	counts := map[string]int{}
	for _, pair := range value.Object.Pairs {
		counts[pair.Key]++
	}
	for _, pair := range value.Object.Pairs {
		if !strings.Contains(pair.Key, "queue") {
			continue
		}
		source := owner + " " + pair.Key
		if counts[pair.Key] > 1 {
			source += " [duplicate saved key]"
		}
		if pair.Key == "externally_owned_build_queues" {
			switch {
			case pair.Value.Mixed || (!pair.Value.IsList() && !(pair.Value.IsObject() && len(pair.Value.Object.Pairs) == 0)):
				refs = append(refs, LocationQueueRef{Source: source, Ref: OptionalIntRef{Recorded: true, Unsupported: true}})
			default:
				for i, entry := range pair.Value.List {
					key := savedPathInt(entry, "key")
					ref := LocationQueueRef{Source: fmt.Sprintf("%s[%d].value", source, i), Key: &key, Ref: savedPathInt(entry, "value")}
					if !entry.IsObject() {
						ref.Ref = OptionalIntRef{Recorded: true, Unsupported: true}
					}
					refs = append(refs, ref)
				}
			}
			continue
		}
		var ref OptionalIntRef
		ref.UnmarshalClausewitz(pair.Value)
		refs = append(refs, LocationQueueRef{Source: source, Ref: ref})
	}
	return refs
}

func decodeLocationQueue(queues, items savedIndex, id int) LocationQueue {
	queue := LocationQueue{ID: id, Record: queues.record(id)}
	if queue.Record.Issue != "" {
		return queue
	}
	value := queue.Record.Value
	queue.Type = savedFieldString(value, "type")
	queue.Disabled = savedFieldString(value, "disabled")
	queue.Simultaneous = savedFieldString(value, "simultaneous")
	queue.Owner = savedPathInt(value, "owner")
	queue.LocType = savedPathInt(value, "location", "type")
	queue.LocID = savedPathInt(value, "location", "id")
	list, recorded, unsupported := savedFieldPath(value, "items")
	queue.ItemsRecorded = recorded
	if !recorded {
		return queue
	}
	var ids OptionalIntList
	if !unsupported {
		ids.UnmarshalClausewitz(list)
	}
	if unsupported || ids.Unsupported {
		queue.ItemsUnsupported = true
		return queue
	}
	for _, itemID := range ids.Values {
		queue.Items = append(queue.Items, decodeLocationItem(items, itemID))
	}
	return queue
}

func decodeLocationItem(items savedIndex, id int) LocationItem {
	item := LocationItem{ID: id, Record: items.record(id)}
	if item.Record.Issue != "" {
		return item
	}
	value := item.Record.Value
	item.Queue = savedPathInt(value, "queue")
	item.PayingCountry = savedPathInt(value, "paying_country")
	item.Progress = savedFieldString(value, "progress")
	item.Needed = savedFieldString(value, "progress_needed")
	for _, pair := range value.Object.Pairs {
		if strings.HasPrefix(pair.Key, "buildable_") {
			item.Buildables = append(item.Buildables, pair)
			if planet := savedPathInt(pair.Value, "planet"); planet.Recorded {
				item.PlanetFields = append(item.PlanetFields, planet)
			}
		}
	}
	return item
}

func savedFieldStringPath(value clausewitz.Value, path ...string) OptionalString {
	field, recorded, unsupported := savedFieldPath(value, path...)
	ref := OptionalString{Recorded: recorded, Unsupported: unsupported}
	if recorded && !unsupported {
		ref.UnmarshalClausewitz(field)
	}
	return ref
}
