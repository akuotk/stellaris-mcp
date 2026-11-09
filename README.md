# Stellaris MCP

MCP (Model Context Protocol) server for analyzing Stellaris save games from Claude Desktop or Claude Code.

Parses `.sav` files (ZIP containing a `gamestate` in Clausewitz format) and exposes analysis and raw exploration tools. The upstream `/stellaris` skill contains strategy guides for patch 4.3 (Cetus), not an assurance of current game compatibility.

## Available Tools

| Tool | Description |
|---|---|
| `get_empire_overview` | Ethics, civics, government, power ratings, naval/starbase capacity, resources |
| `get_research_status` | Current research + progress, available options, completed techs |
| `get_special_projects` | Saved special project records and research queue and fleet order ID candidates |
| `get_event_chains` | Paginated saved country event-chain and completed-chain entries |
| `get_player_events` | Paginated saved player events and selection-history entries |
| `get_saved_messages` | Paginated root message records in saved order, with raw fields |
| `get_trade_routes` | Paginated saved trade routes and manager entries, with numeric ID candidates |
| `get_saved_event_targets` | Paginated root event targets and numeric ID candidates |
| `get_debris` | Saved debris records, components, resources and special project references |
| `get_research_potential` | Paginated saved research-potential records with snapshot revision |
| `get_economy_breakdown` | Current or last month saved budget categories and calculated net |
| `get_planets` | Colonized planets with districts, buildings, jobs, modifiers, production |
| `get_colonies` | Direct saved colony records and carrier references, including non-planet types |
| `get_pre_ftl` | Saved pre-FTL country, planet and observation fleet references |
| `get_construction` | Construction queues and item references, saved progress and resources |
| `get_espionage_operations` | Espionage operations, network cross-references and saved logs |
| `get_first_contacts` | Saved first-contact records, references and event histories |
| `get_system_connections` | Saved system hyperlanes, bypass and natural wormhole references |
| `get_system_contents` | Saved stars, bodies, moons, deposits, ambient objects and nebula candidates |
| `get_galaxy_metadata` | Ordered root galaxy records and saved fields |
| `get_galaxy_regions` | Paginated saved clusters, nebulae and their object entries |
| `get_sectors` | Saved sectors and country, system and capital references |
| `get_casus_belli` | Saved diplomatic casus belli records and country references |
| `get_resource_market` | Saved economy-module resource amounts and country-matched monthly trade entries |
| `get_policies_edicts` | Saved policy selections, edict records and policy flags |
| `get_starbases` | Saved starbase records and candidate station, system, fleet and queue links |
| `get_archaeology` | Saved archaeological sites, fleet order references, event and completion records |
| `get_armies` | Ground armies, country, colony, ship, fleet, leader and species references |
| `get_planets available=true` | Surveyed uncolonized habitable planets with features |
| `get_fleet_templates` | Saved fleet templates, design/count entries and reference links |
| `get_fleet_power` | Fleets with power, ship-level detail (design, HP, weapons) |
| `get_ship_loadout` | Saved ship sections and weapons, design stages and slot comparisons |
| `get_space_combat_objects` | Paginated saved missile or strike-craft records |
| `get_council` | Saved council positions, leader references and agenda records |
| `get_factions` | Saved political factions, member references and colony history |
| `get_leaders` | Leaders with class, traits, council position, assignment |
| `get_relics` | Saved relic keys and last-activation/receipt fields |
| `get_grand_archive` | Saved Grand Archive exhibit and specimen management settings |
| `get_species_rights` | Saved default and species-specific rights records |
| `get_species` | Species identities, traits and saved population references on owned planets |
| `get_neighbors` | Saved bilateral relations, opinion, trust and relative power |
| `get_traditions_ascension` | Adopted traditions and ascension perks |
| `get_espionage` | Saved spy networks, country and leader references, and timed modifiers |
| `get_notifications` | Wars, federations, players, game status |
| `get_wars` | Saved war records and paginated battle history |
| `get_empire_assets` | Recorded agreements, subject references, situations and megastructures |
| `get_galactic_community` | Members, resolution references and saved voting records |
| `get_federations` | Saved federation membership, leadership, progression and laws |
| `list_save_games` | Paginated local save catalog with stable IDs |
| `inspect_save_archive` | Save revision and paginated ZIP entry metadata |
| `read_save_tree` | Ordered tree navigation, including mixed entries and source offsets |
| `search_save_tree` | Paginated search for keys and scalar values |
| `read_save_scalar` | Paginated normalized scalar or key bytes |
| `read_save_bytes` | Paginated exact source bytes from a ZIP entry or container |

Country-specific tools accept `country_id`. If omitted, they select the sole player recorded in a single-player save; if the player is ambiguous or absent, supply the ID explicitly. Use `list_save_games` and `save_id` to select a save, or omit `save_id` for the newest.

Raw exploration preserves ordered duplicate keys, mixed keyed/anonymous blocks, quote/escape flags and source offsets. Normalized values may not preserve spelling or comments: use the offsets with `read_save_bytes` for exact content. Pagination requires the archive revision returned by `inspect_save_archive`; a changed save is rejected. For continued byte, tree, scalar or search reads, pass the returned `page_context` to detect changes in the selected source and query. A nonzero offset without the token remains supported for random access and legacy clients but is marked `pagination_context_verified=false`. The server does not send files to a network service, but clients receive requested data and may forward it according to their own configuration. Never upload private saves. See [COMPATIBILITY.md](COMPATIBILITY.md) for the remaining limits.

Drill-down tools (`get_planets`, `get_fleet_power`, `get_leaders`) accept an ID for detailed view.

## `/stellaris` Skill (Claude Code)

A Claude Code skill at `.claude/skills/stellaris/` provides strategic advice by combining live save data with compiled strategy guides:

- **Economy** -- districts, specializations, trade, designations, mega-economy
- **Military** -- fleet composition, ship design, war timing, crisis counter-builds
- **Research** -- tech priorities, traditions, ascension perks, beelining
- **Expansion** -- colonization criteria, chokepoints, wide vs tall
- **Diplomacy** -- federations, vassals, leaders, council, espionage

Usage: `/stellaris how should I optimize my economy?`

## Installation

### Prerequisites

- Go 1.22+
- Claude Desktop or Claude Code

### Build

```bash
# Linux / macOS
go build -o bin/stellaris-mcp ./cmd/stellaris-mcp

# Windows (cross-compile from Linux/macOS)
GOOS=windows GOARCH=amd64 go build -o bin/stellaris-mcp.exe ./cmd/stellaris-mcp
```

Or download a pre-built binary from [Releases](https://github.com/bmassemin/stellaris-mcp/releases).

### Usage

```
stellaris-mcp <save-games-directory> <localization-directory>
```

- **save-games-directory** (required): root folder containing your Stellaris saves. The server recursively walks all subdirectories and picks the most recently modified `.sav` file.
- **localization-directory** (required): path to the `english` localization folder from your Stellaris installation. Provides real in-game names (e.g. "Energy Credits", "Forge Capital"). The server exits with an error if the path doesn't exist or contains no localization files.

### Claude Desktop

Add to `claude_desktop_config.json`:

**Windows** (`%APPDATA%\Claude\claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "stellaris": {
      "command": "C:\\path\\to\\stellaris-mcp.exe",
      "args": [
        "C:\\Users\\YourName\\Documents\\Paradox Interactive\\Stellaris\\save games",
        "C:\\Program Files (x86)\\Steam\\steamapps\\common\\Stellaris\\localisation\\english"
      ]
    }
  }
}
```

**macOS** (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "stellaris": {
      "command": "/path/to/stellaris-mcp",
      "args": [
        "/home/user/.local/share/Paradox Interactive/Stellaris/save games",
        "/home/user/.steam/steam/steamapps/common/Stellaris/localisation/english"
      ]
    }
  }
}
```

### Claude Code

```bash
claude mcp add --transport stdio stellaris \
  -- /path/to/stellaris-mcp \
  "/path/to/Stellaris/save games" \
  "/path/to/Stellaris/localisation/english"
```

The `/stellaris` skill is automatically available when working in this repository.

## Sample Outputs

See [sample_outputs/examples.md](sample_outputs/examples.md) for real output examples of every tool, generated from a mid-game save with English localization.

## Tests

Copy a `gamestate` file (extracted from a `.sav`) into `internal/clausewitz/testdata/`:

```bash
cp ~/gamestate internal/clausewitz/testdata/gamestate
```

To run localization tests, create a `.env` file at the project root (see `.env.example`):

```bash
cp .env.example .env
# Edit .env with your Stellaris localisation/english path
```

Then run:

```bash
go test ./...
```

Tests that require the fixture or localization files skip gracefully when they are absent.

## Architecture

```
cmd/stellaris-mcp/           Entry point (stdio MCP server)
internal/
├── clausewitz/              Clausewitz format parser (lexer -> parser -> decoder)
│                            Unmarshal(data, &struct) like encoding/json
├── gamestate/               Go structs mapping the Stellaris gamestate
│                            Load from raw bytes, ZIP, or save directory
│                            Localization service (YAML loc files -> l(key))
├── tools/                   MCP tools (one file per tool)
│                            common.go: shared helpers (loadLatestSave, getCountry, l, ll)
└── testutil/                .env file loader for tests
.claude/skills/stellaris/    /stellaris skill + strategy guides
```
