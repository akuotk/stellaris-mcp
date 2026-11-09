# Compatibility notes

This fork adds preliminary support for Stellaris 4.5 colony and population records. It reads colony data through verified planet-to-colony references, connects population groups to species, ethics and factions, and reports population values in raw save units; it does not assume a conversion to the game's displayed POP counts. Mismatched distribution arrays are explicitly reported rather than paired.

`get_neighbors` reports saved contact and strength values, with a derived military ratio where both inputs permit it. It does not assign Stellaris UI strength categories. Missing country and fleet references are reported rather than treated as ID zero.

With `neighbor_id`, `get_neighbors` displays both saved directions independently, including trust, opinion modifiers and truce fields. Differences between the two records remain visible; timing and modifier semantics are not inferred.

`get_empire_overview` distinguishes missing population, fleet size, naval capacity, starbase capacity and empire size from zero. Its planet count covers decoded owned planet records with colony indicators; non-planet carriers are excluded.

The military-class total only includes fleets whose `ship_class` is `shipclass_military`. Ship hull, shield and armor values preserve missing versus zero and are shown in saved precision. Ship designs can have multiple sections and growth stages; fleet detail lists each saved design stage without claiming which stage the ship currently uses. Missing fleet and ship references are reported, and military-class totals are labeled incomplete when fleet records are missing. Spy network power is reported as a raw save value; infiltration conversion and operation availability are not established.

`get_ship_loadout` exposes saved ship sections and weapon entries alongside legacy or growth-stage design layouts, checking the fleet's reciprocal ship list. Slot comparisons do not establish which growth stage is active or infer design component roles. Runtime weapon values remain raw.

`get_space_combat_objects` displays non-`none` missile or strike-craft records in saved order, with page contexts bound to the save revision and table. `none` entries are counted and remain available through raw tools. Fleet and position-origin IDs are matched to saved table candidates without inferring relationships, ownership, activity, target types or units.

Planet and colony details expose saved district, zone, building and deposit references, including external building records and raw terraforming data when present. Duplicate or missing references are reported. Deposit holder ID comparisons do not establish the meaning of holder types.

The `get_empire_assets` tool lists saved agreements, situations and megastructures matching the selected country. Agreement records are included when either saved owner or target matches the selected country; missing and unresolved counterpart references are explicit. Saved agreement owner and target references are checked against country subject and overlord references when available. Discrete and resource terms, selected subject-specialization fields, and situation progress are shown as stored without inferring their in-game meaning. Megastructure owner and system references are cross-checked against saved country and system lists when available. Planet and bypass matches are candidates, and bypass owner type and build queue semantics remain unverified. It does not reconstruct all diplomacy, situation mechanics, or unrecorded game state.

`get_galactic_community` links the community's member and resolution IDs to the saved country and resolution records. `get_research_status` distinguishes missing and empty research queues and alternatives, and reports saved progress without rounding; it shows a completed technology level only when the corresponding `level` immediately follows that technology in the saved record; other levels are labeled unavailable. `get_research_potential` pages through saved internal potential entries. Their numerical meaning is not established, and they must not be treated as current research choices or probabilities. Stored research values are reported as saved.

`get_special_projects` exposes country event records and matching research-queue and fleet current-order ID candidates. Duplicate or unsupported records remain ambiguous; project availability, completion, time units and location namespaces are not established. Deep or mixed payloads remain accessible through the raw tools.

`get_event_chains` displays saved `event_chain` and `completed_event_chain` fields in country event order, with page contexts bound to the save and country. Nested event-target records in chains and special projects are compared against root targets by saved name, type and ID, and their numeric IDs against decoded country, system, planet, ship and fleet tables. These are candidates only; opener, namespace, source uniqueness and identity remain unverified. Scope types, counters, completion semantics and current availability are unverified; deep or mixed payloads remain accessible through raw tools.

`get_saved_event_targets` displays save-wide root target records in saved order and flags repeated unqualified numeric IDs. Numeric matches to country, system, planet, ship and fleet tables are candidates only; the saved type's namespace, ownership and nested event-target semantics are unverified. Continuations bind to the save revision.

`get_debris` exposes save-wide debris records and special project ID candidates, including debris without a project reference. Coordinates, components, resources and country fields are shown as saved; matching IDs do not establish ownership, visibility or availability. Deep or mixed payloads remain accessible through the raw tools.

`get_federations` and `get_notifications` expose indexed federation records, including members, leadership, laws and saved progression values. Cohesion and experience are reported as saved rather than converted into UI percentages.

Federation country and envoy references are checked against available reverse references. Duplicate or malformed federation records are reported separately from missing IDs. `get_galactic_community` exposes saved voting rows and resolution references; voting codes and present Senate state remain unverified.

`list_save_games` lists local saves and their path-based IDs. Pass a `save_id` to an analysis or raw exploration tool to select a listed save, or omit it for the newest. For country-specific tools, an omitted `country_id` selects the sole recorded player; multiplayer or absent player data requires an explicit ID. `inspect_save_archive`, `read_save_tree`, `search_save_tree` and `read_save_scalar` navigate entries using positional paths and a required archive revision. Optional page-context tokens bind byte, tree, scalar and search continuations to their source and operation; a nonzero offset without one is flagged unverified to retain random-access and older client compatibility. Directory symlinks are not traversed. The semantic tree preserves ordered mixed and duplicate entries plus original byte offsets, but normalizes token text and may not represent every possible Clausewitz syntax. `read_save_bytes` returns bounded base64-encoded raw bytes from ZIP entries or the ZIP container itself, including syntax the parser does not understand. These tools read locally and do not make network requests; clients receive requested data and may have their own data-handling behavior. Do not upload saves to external services.

`get_fleet_templates` reports saved fleet template references, design/count entries and candidate queue and home-base links. Counts and reinforcement semantics, queue-array alignment and completion are unverified.

`get_council` reports saved council positions, leader references, agenda and ordered cooldown records. Parallel-list alignment and leader back-references are checked where available; agenda effects, cooldown semantics and eligibility remain unverified.

`get_leaders` and `get_council` cross-check saved ruler, heir, owned leader and council position references when available. Leader details retain saved experience and history values; matches do not establish undocumented gameplay semantics.

`get_factions` exposes saved country faction references and faction members. Legacy POP and current POP-group references are checked separately; ambiguous ID collisions are reported. Political effects, population units and positional arrays remain unverified.

`get_casus_belli` reports saved country diplomacy-module records and country ID references; type, timing and roles remain raw and do not establish an available declaration of war.

`get_economy_breakdown` separates saved balance categories from a computed income-minus-expenses net, for the saved current or last month. Missing tables are flagged; values and keys retain their saved precision.

`get_system_connections` exposes saved hyperlane, bypass and natural wormhole records with available reverse-ID checks. Duplicate root connection tables or system reference fields are flagged as unsupported rather than silently choosing one. Visibility, travel eligibility, value units and route costs remain unverified.

`get_system_contents` displays saved star, body, moon, deposit and ambient-object records, checking available body-origin, ambient-object origin and reciprocal moon references. It also displays nebula records with matching saved `galactic_object` values as candidates; nebula membership and visibility are unverified. Orbital values, coordinate units, ownership, current visibility and resource yields are not inferred.

`get_galaxy_metadata` displays root `galaxy` records and duplicate fields in saved order. Their status as configuration or current state, units and applicability are not inferred. Deep or mixed data remains available through the raw tools.

`get_galaxy_regions` pages through saved clusters and nebulae in root order. The `cluster_objects` and `nebula_objects` views page through their saved entries separately. Each continuation token is bound to the selected view and save revision. Object IDs show numeric candidates in the system, planet and country tables. Namespace, membership, coordinate, route and visibility semantics remain unverified. Empty and unsupported records retain their saved position. Use raw tree or byte tools for deep, large or mixed values.

`get_sectors` reports saved sector ownership, systems and capital references, checks available back-references, and flags incomplete or conflicting data. An ID match without a verified reverse link remains a candidate.

`get_notifications` lists saved war records, including those with end dates; it does not infer whether a war is active from a missing date.

`get_wars` exposes saved war participants, goals, exhaustion, force-peace fields and paginated battle records. Continuations bind to the save revision and war ID. Battle country and location matches remain candidates; battle type codes, loss and exhaustion units, and current war activity are unverified.

Situation records in `get_empire_assets` include country references, saved stages, durations, flags and targets. Explicit planet targets are checked against planet records; other target namespaces, opener IDs and duration units remain unverified.

`get_resource_market` reports saved economy-module resource values as stockpile candidates and filters monthly trade records by their saved trade_data.country reference. Resource units, stockpile identity, trade direction and whether orders are currently active are not inferred.

Market purchase, sale and internal fluctuation records preserve adjacent saved country and payload entries. Their array indices, currencies and periods remain unverified. Economy breakdowns also expose saved trade, extra and income high-water fields separately from calculated net values.

`get_policies_edicts` displays saved policy selections, edicts and policy flags in saved order. The meaning of dates and flags, policy effects and current edict activity are not inferred.

`get_starbases` counts the raw system reference value 4294967295 separately from other IDs without a decoded base record; its meaning is unverified. The survey spans all saved systems and warns when coverage is incomplete.

`get_starbases` displays saved starbase slots and candidate station, system, fleet and construction queue references. Matching IDs and queue ownership do not alone prove base ownership; unsupported shapes and unresolved links are reported.

`get_first_contacts` exposes saved country, location and leader references, events and completed-history entries, including country-side references checked against recorded owners. Object ID matches remain candidates; stage, status and timing are reported without inferring current completion or activity.

`get_traditions_ascension` retains saved tradition, perk and category keys and the last-picked tradition. Categories are not assumed to be completed trees. Unsupported field shapes are flagged, while other country data remain available.

`get_archaeology` exposes saved site, visibility, completion-history, event and log records. Fleet references are checked against the saved excavation order when available, and selected-country fleet orders without a reciprocal site reference are also reported; matching IDs alone do not establish current game activity. Event scope IDs are compared with saved fleet and site IDs; scope type codes, location enums and site completion semantics remain unverified. Raw tools provide deeper event data.

The typed parser identifies mixed keyed and anonymous blocks without treating them as ordinary lists or objects. Unknown mixed fields do not block unrelated known fields. A mixed known field may still cause its containing typed parse to fail; selected optional records support partial decoding with explicit warnings. Raw tree and byte tools remain available.

`get_espionage_operations` reports saved operations, network links and ordered logs. Operation index, timing, rolls and opener IDs remain raw values; their in-game meaning is not inferred. Unsupported records are flagged and remain accessible using raw tools.

`get_espionage` checks country-side network references and available leader location back-references, and exposes saved timed modifiers. Modifier durations and the gameplay meaning of saved values remain unverified.

Planet population, stability, crime, amenities and housing distinguish missing values from zero. Modern colony jobs are counted only when the planet/colony and colony/job references agree; missing or inconsistent references are reported. Positive workforce totals omit zero and negative values; direct colony output retains the raw values.

`get_armies` reports saved army references and checks reverse links to owned army lists, colony lists, ships and fleets when present. Health, morale and experience are raw values; unsupported records are flagged. Older `planet` references are reported separately, and ID matches alone do not establish hidden game mechanics. Leader ship locations resolve via the ship record and only name a fleet after reciprocal membership validation.

`get_construction` connects saved queues and their ordered items, reporting missing or inconsistent references. Saved progress, resources and numeric location types are not converted to UI values; a matching colony ID is only a candidate. Unsupported construction shapes are flagged and remain accessible through the raw tools.

`get_relics` exposes saved relic keys and last-activation/receipt fields in stored form. Their units, relic association and current activation eligibility remain unverified.

Typed ZIP analysis reads archive bytes into memory first and rejects missing or duplicate gamestate entries and CRC errors. This is not an atomic filesystem snapshot if another process writes concurrently.

`get_grand_archive` exposes saved exhibit and management-setting structures. Empty entries are not interpreted as specimen counts; specimen identity, ownership and setting effects are unverified. Deep or mixed structures remain available via raw tools.

`get_species_rights` exposes saved default, primary, built-species and species-specific records. Species ID matches are candidates; effective rights, inheritance and date meanings remain unverified.

`get_species` links species records to country and owned-planet references, including saved base species and home planet candidates, and displays traits and saved group-size totals. Incomplete or duplicate references are reported; groups referenced by multiple owned planets are counted once and their location is marked ambiguous. Non-planet carriers are not included. `get_colonies` exposes owned colony records, species information and raw carrier types and checks population group and job back-references without inventing a carrier identity. Non-planet carrier serialization has not yet been checked against a real Arkship save.

`get_pre_ftl` exposes countries with saved primitive type or pre-FTL age, capital candidates and observation fleet references. Planet and fleet back-references are checked when available; fleet registration and planet owner are reported separately. Age and awareness scales and present visibility remain unverified.

Duplicate or malformed colony carrier records are reported as unsupported and are not projected onto a planet. The meaning of non-planet carrier types and references remains unverified.

Limitations: non-planet carriers such as Nomad Arkships are not yet resolved as colonies; complex name templates, agreement terms and espionage require further work. Political faction support and approval are shown as saved, without unit conversion. A missing naval capacity is reported as unavailable. Do not assume every value is complete or that this covers all versions, DLCs, or modded saves.

Run `go test ./...` for synthetic tests. To additionally test a local save without modifying it, set `STELLARIS_TEST_SAVE` to an absolute `.sav` path before running `go test -v ./internal/gamestate`. Never commit or upload save files or personal save data.

`get_saved_messages` reads root `message` records in save order and keeps repeated records and fields. Pages contain up to 20 records and continuation tokens are bound to the save revision. The tool displays saved values as recorded. The meanings of reference fields, read status and current UI visibility are unverified. Use raw tree or byte tools for deep or mixed blocks.

`get_saved_flags` pages through root `flags` fields in save order, including duplicate keys and repeated roots. Empty and unsupported roots retain their positions. Continuation tokens are bound to the save revision. Flag names, numeric units, current state and clearing conditions remain unverified. Use raw tree or byte tools for deep, large or mixed values.

`get_trade_routes` reads saved `trade_routes` entries, including records outside the manager list and unsupported shapes. Use `kind=manager` to page through `trade_routes_manager` and see its numeric matches against route keys, including duplicates and missing keys. Page contexts are bound to the save revision and view. Country, system, planet, starbase and fleet key matches for `owner`, `from`, `to` and `path[].id` are numeric candidates. The identifier namespaces and current activity remain unverified. Use raw tree or byte tools for deep or mixed blocks.

`get_player_events` displays saved root player-event and selection-history records in order, optionally filtered by event or history, with page contexts bound to the save revision and filter. Country ID and same-save selection-to-root-event numeric matches are candidates only; identifier namespaces and current UI state, event availability, ownership and selection semantics remain unverified. Deep or mixed data requires raw tools.
