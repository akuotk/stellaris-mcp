package gamestate

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"os"

	"github.com/bmassemin/stellaris-mcp/internal/clausewitz"
)

type GameState struct {
	Debris              DebrisTable                                 `clausewitz:"debris"`
	Bypasses            ConnectionRecordTable[SavedBypass]          `clausewitz:"bypasses"`
	NaturalWormholes    ConnectionRecordTable[SavedNaturalWormhole] `clausewitz:"natural_wormholes"`
	CouncilPositions    CouncilPositionTable                        `clausewitz:"council_positions"`
	FleetTemplates      FleetTemplates                              `clausewitz:"fleet_template"`
	Sectors             Sectors                                     `clausewitz:"sectors"`
	MarketOrders        MarketOrders                                `clausewitz:"market"`
	Starbases           Starbases                                   `clausewitz:"starbase_mgr"`
	FirstContacts       FirstContacts                               `clausewitz:"first_contacts"`
	ArchaeologySites    ArchaeologySites                            `clausewitz:"archaeological_sites"`
	EspionageOperations EspionageOperations                         `clausewitz:"espionage_operations"`
	Armies              Armies                                      `clausewitz:"army"`
	Construction        Construction                                `clausewitz:"construction"`
	Colonies            map[int]Colony                              `clausewitz:"colony"`
	PopGroups           map[int]PopGroup                            `clausewitz:"pop_groups"`
	Species             map[int]Species                             `clausewitz:"species_db"`
	PopFactions         map[int]PopFaction                          `clausewitz:"pop_factions"`
	Agreements          AgreementStore                              `clausewitz:"agreements"`
	Situations          SituationStore                              `clausewitz:"situations"`
	Megastructures      ConnectionRecordTable[Megastructure]        `clausewitz:"megastructures"`
	Systems             map[int]GalacticSystem                      `clausewitz:"galactic_object"`
	Community           *GalacticCommunity                          `clausewitz:"galactic_community"`
	Resolutions         map[int]Resolution                          `clausewitz:"resolution"`
	Version             string                                      `clausewitz:"version"`
	Name                string                                      `clausewitz:"name"`
	Date                string                                      `clausewitz:"date"`
	Player              []Player                                    `clausewitz:"player"`
	Country             map[int]Country                             `clausewitz:"country"`
	Fleet               map[int]Fleet                               `clausewitz:"fleet"`
	Ships               map[int]Ship                                `clausewitz:"ships"`
	ShipDesign          map[int]ShipDesign                          `clausewitz:"ship_design"`
	Planets             PlanetsWrapper                              `clausewitz:"planets"`
	Buildings           map[int]Building                            `clausewitz:"buildings"`
	Districts           map[int]District                            `clausewitz:"districts"`
	Zones               map[int]Zone                                `clausewitz:"zones"`
	Deposit             map[int]Deposit                             `clausewitz:"deposit"`
	SpyNetworks         map[int]SpyNetwork                          `clausewitz:"spy_networks"`
	PopJobs             map[int]PopJob                              `clausewitz:"pop_jobs"`
	Leaders             map[int]Leader                              `clausewitz:"leaders"`
	War                 ConnectionRecordTable[War]                  `clausewitz:"war"`
	Federation          map[int]Federation                          `clausewitz:"federation"`
}

type Building struct {
	Type     string `clausewitz:"type"`
	Position int    `clausewitz:"position"`
}

type SpyNetwork struct {
	Owner            *int     `clausewitz:"owner"`
	Target           *int     `clausewitz:"target"`
	Leader           *int     `clausewitz:"leader"`
	Power            *float64 `clausewitz:"power"`
	ActiveOperations []int    `clausewitz:"active_operations"`
	Formed           string   `clausewitz:"formed"`
}

type PopJob struct {
	PlanetID     *int     `clausewitz:"planet"`
	Type         string   `clausewitz:"type"`
	Planet       int      `clausewitz:"planet"`
	Workforce    float64  `clausewitz:"workforce"`
	WorkforceRaw *float64 `clausewitz:"workforce"`
}

type Deposit struct {
	Type     string `clausewitz:"type"`
	SwapType string `clausewitz:"swap_type"`
}

type District struct {
	Type  string `clausewitz:"type"`
	Level int    `clausewitz:"level"`
	Zones []int  `clausewitz:"zones"`
}

type Zone struct {
	Type      string `clausewitz:"type"`
	Buildings []int  `clausewitz:"buildings"`
}

type Player struct {
	Name      string `clausewitz:"name"`
	Country   int    `clausewitz:"country"`
	CountryID *int   `clausewitz:"country"`
}

type PlanetsWrapper struct {
	Planet map[int]Planet `clausewitz:"planet"`
}

// LoadFromDir finds the most recent .sav file in dir,
// extracts the "gamestate" file from the ZIP, and parses it.
func LoadFromDir(dir string) (*GameState, error) {
	savPath, err := FindLatestSave(dir)
	if err != nil {
		return nil, err
	}
	return LoadFromZip(savPath)
}

// LoadFromZip extracts "gamestate" from a .sav ZIP and parses it.
func LoadFromZip(path string) (*GameState, error) {
	data, err := extractGamestate(path)
	if err != nil {
		return nil, err
	}
	return Load(data)
}

// LoadFromArchive parses the gamestate entry in an already opened save snapshot.
func LoadFromArchive(archive *zip.Reader) (*GameState, error) {
	data, err := extractGamestateEntry(archive)
	if err != nil {
		return nil, err
	}
	return Load(data)
}

// Load parses raw gamestate bytes into a GameState.
func Load(data []byte) (*GameState, error) {
	var gs GameState
	root, err := clausewitz.Parse(data)
	if err != nil {
		return nil, fmt.Errorf("parse gamestate: %w", err)
	}
	value := clausewitz.Value{Object: root}
	if err := clausewitz.DecodeValue(value, &gs); err != nil {
		return nil, fmt.Errorf("parse gamestate: %w", err)
	}
	if uniqueSavedFields(value, "war") != nil {
		gs.War = ConnectionRecordTable[War]{Recorded: true, TableIssue: true}
	}
	if uniqueSavedFields(value, "debris") != nil {
		gs.Debris = DebrisTable{Recorded: true, Unsupported: true}
	}
	if uniqueSavedFields(value, "bypasses") != nil {
		gs.Bypasses = ConnectionRecordTable[SavedBypass]{Recorded: true, TableIssue: true}
	}
	if uniqueSavedFields(value, "natural_wormholes") != nil {
		gs.NaturalWormholes = ConnectionRecordTable[SavedNaturalWormhole]{Recorded: true, TableIssue: true}
	}
	if uniqueSavedFields(value, "agreements") != nil {
		gs.Agreements = AgreementStore{}
		gs.Agreements.TableIssue = true
	}
	if uniqueSavedFields(value, "megastructures") != nil {
		gs.Megastructures = ConnectionRecordTable[Megastructure]{Recorded: true, TableIssue: true}
	}
	if uniqueSavedFields(value, "situations") != nil {
		gs.Situations = SituationStore{}
		gs.Situations.TableIssue = true
	}
	gs.markAmbiguousFleetOrders(value)
	if uniqueSavedFields(value, "archaeological_sites") != nil {
		gs.ArchaeologySites.TableIssue = true
	}
	gs.resolveColonies()
	return &gs, nil
}

// FindLatestSave selects by modification time, with a deterministic path tie-break.
// It reports inaccessible directories instead of silently selecting an older save.
func FindLatestSave(dir string) (string, error) {
	return FindSaveByID(dir, "")
}

func extractGamestate(zipPath string) ([]byte, error) {
	data, err := os.ReadFile(zipPath)
	if err != nil {
		return nil, fmt.Errorf("read save archive: %w", err)
	}
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("open save archive: %w", err)
	}
	return extractGamestateEntry(r)
}

func extractGamestateEntry(r *zip.Reader) ([]byte, error) {
	var selected *zip.File
	for _, f := range r.File {
		if f.Name != "gamestate" {
			continue
		}
		if selected != nil {
			return nil, fmt.Errorf("multiple gamestate entries; typed analysis cannot select one safely")
		}
		selected = f
	}
	if selected == nil {
		return nil, fmt.Errorf("no gamestate entry found")
	}
	if selected.FileInfo().IsDir() {
		return nil, fmt.Errorf("gamestate entry is a directory")
	}
	rc, err := selected.Open()
	if err != nil {
		return nil, fmt.Errorf("open gamestate in zip: %w", err)
	}
	defer rc.Close()
	contents, err := io.ReadAll(rc)
	if err != nil {
		return nil, fmt.Errorf("read gamestate in zip: %w", err)
	}
	return contents, nil
}
