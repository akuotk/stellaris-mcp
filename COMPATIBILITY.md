# Compatibility notes

This fork adds preliminary support for Stellaris 4.5 colony and population records. It reads colony data through verified planet-to-colony references, connects population groups to species, ethics and factions, and reports population values in raw save units; it does not assume a conversion to the game's displayed POP counts. Mismatched distribution arrays are explicitly reported rather than paired.

`get_neighbors` reports saved contact and strength values, with a derived military ratio where both inputs permit it. It does not assign Stellaris UI strength categories. Missing country and fleet references are reported rather than treated as ID zero.

The military-class total only includes fleets whose `ship_class` is `shipclass_military`. Ship designs can have multiple sections and growth stages; fleet detail lists each saved design stage without claiming which stage the ship currently uses. Missing fleet and ship references are reported, and military-class totals are labeled incomplete when fleet records are missing. Spy network power is reported as a raw save value; infiltration conversion and operation availability are not established.

The `get_empire_assets` tool lists saved agreements, situations and megastructures matching the selected country. Agreement owner/target roles, discrete and resource terms, selected subject-specialization fields, and situation progress are shown as stored, without inferring their in-game meaning. It does not reconstruct all diplomacy, situation mechanics, or unrecorded game state.

`get_galactic_community` links the community's member and resolution IDs to the saved country and resolution records. `get_research_status` distinguishes missing and empty research queues and alternatives, and reports saved progress without rounding; it shows a completed technology level only when the corresponding `level` immediately follows that technology in the saved record; other levels are labeled unavailable. `get_research_potential` pages through saved internal potential entries. Their numerical meaning is not established, and they must not be treated as current research choices or probabilities. Stored research values are reported as saved.

`get_federations` and `get_notifications` expose indexed federation records, including members, leadership, laws and saved progression values. Cohesion and experience are reported as saved rather than converted into UI percentages.

`list_save_games` lists local saves and their path-based IDs. Pass a `save_id` to an analysis or raw exploration tool to select a listed save, or omit it for the newest. For country-specific tools, an omitted `country_id` selects the sole recorded player; multiplayer or absent player data requires an explicit ID. `inspect_save_archive`, `read_save_tree`, `search_save_tree` and `read_save_scalar` navigate entries using positional paths and a required archive revision. Directory symlinks are not traversed. The semantic tree preserves ordered mixed and duplicate entries plus original byte offsets, but normalizes token text and may not represent every possible Clausewitz syntax. `read_save_bytes` returns bounded base64-encoded raw bytes from ZIP entries or the ZIP container itself, including syntax the parser does not understand. These tools read locally and do not make network requests; clients receive requested data and may have their own data-handling behavior. Do not upload saves to external services.

`get_casus_belli` reports saved country diplomacy-module records and country ID references; type, timing and roles remain raw and do not establish an available declaration of war.

`get_economy_breakdown` separates saved balance categories from a computed income-minus-expenses net, for the saved current or last month. Missing tables are flagged; values and keys retain their saved precision.

`get_sectors` reports saved sector ownership, systems and capital references, checks available back-references, and flags incomplete or conflicting data. An ID match without a verified reverse link remains a candidate.

`get_notifications` lists saved war records, including those with end dates; it does not infer whether a war is active from a missing date.

`get_resource_market` reports saved economy-module resource values as stockpile candidates and filters monthly trade records by their saved trade_data.country reference. Resource units, stockpile identity, trade direction and whether orders are currently active are not inferred.

`get_policies_edicts` displays saved policy selections, edicts and policy flags in saved order. The meaning of dates and flags, policy effects and current edict activity are not inferred.

`get_starbases` counts the raw system reference value 4294967295 separately from other IDs without a decoded base record; its meaning is unverified. The survey spans all saved systems and warns when coverage is incomplete.

`get_starbases` displays saved starbase slots and candidate station, system, fleet and construction queue references. Matching IDs and queue ownership do not alone prove base ownership; unsupported shapes and unresolved links are reported.

`get_first_contacts` exposes saved country, location and leader references, events and completed-history entries. Object ID matches remain candidates; stage, status and timing are reported without inferring current completion or activity.

`get_traditions_ascension` retains saved tradition, perk and category keys and the last-picked tradition. Categories are not assumed to be completed trees. Unsupported field shapes are flagged, while other country data remain available.

`get_archaeology` exposes saved site, visibility, completion-history, event and log records. Location enums and site completion semantics are unverified, and matching planet or fleet IDs are candidate matches only. Event scopes remain opaque and can be explored through the raw tools.

The typed parser identifies mixed keyed and anonymous blocks without treating them as ordinary lists or objects. Unknown mixed fields do not block unrelated known fields. A mixed known field may still cause its containing typed parse to fail; selected optional records support partial decoding with explicit warnings. Raw tree and byte tools remain available.

`get_espionage_operations` reports saved operations, network links and ordered logs. Operation index, timing, rolls and opener IDs remain raw values; their in-game meaning is not inferred. Unsupported records are flagged and remain accessible using raw tools.

Planet population, stability, crime, amenities and housing distinguish missing values from zero. Modern colony jobs are counted only when the planet/colony and colony/job references agree; missing or inconsistent references are reported. Positive workforce totals omit zero and negative values; direct colony output retains the raw values.

`get_armies` reports saved army references and checks reverse links to owned army lists, colony lists, ships and fleets when present. Health, morale and experience are raw values; unsupported records are flagged. Older `planet` references are reported separately, and ID matches alone do not establish hidden game mechanics. Leader ship locations resolve via the ship record and only name a fleet after reciprocal membership validation.

`get_construction` connects saved queues and their ordered items, reporting missing or inconsistent references. Saved progress, resources and numeric location types are not converted to UI values; a matching colony ID is only a candidate. Unsupported construction shapes are flagged and remain accessible through the raw tools.

`get_species` links species records to owned planets' population groups and displays traits and saved group-size totals. Incomplete or duplicate references are reported; non-planet carriers are not included. `get_colonies` exposes owned colony records and raw carrier types and references without inventing a carrier identity. Non-planet carrier serialization has not yet been checked against a real Arkship save.

Limitations: non-planet carriers such as Nomad Arkships are not yet resolved as colonies; complex name templates, agreement terms and espionage require further work. Political faction support and approval are shown as saved, without unit conversion. A missing naval capacity is reported as unavailable. Do not assume every value is complete or that this covers all versions, DLCs, or modded saves.

Run `go test ./...` for synthetic tests. To additionally test a local save without modifying it, set `STELLARIS_TEST_SAVE` to an absolute `.sav` path before running `go test -v ./internal/gamestate`. Never commit or upload save files or personal save data.
