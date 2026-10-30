# Compatibility notes

This fork adds preliminary support for Stellaris 4.5 colony and population records. It reads colony data through verified planet-to-colony references, connects population groups to species, ethics and factions, and reports population values in raw save units; it does not assume a conversion to the game's displayed POP counts. Mismatched distribution arrays are explicitly reported rather than paired.

The military-class total only includes fleets whose `ship_class` is `shipclass_military`. Ship designs can have multiple sections. Spy network power is reported as a raw save value; infiltration conversion and operation availability are not established.

The `get_empire_assets` tool lists saved agreements, situations and megastructures matching the selected country. Agreement owner/target roles and situation progress are shown as stored, without inferring their in-game meaning. The tool does not attempt to reconstruct all diplomacy, situation mechanics, or unrecorded game state.

Limitations: non-planet carriers such as Nomad Arkships are not yet resolved as colonies; complex name templates, federations, agreement terms and espionage require further work. Political faction support and approval are shown as saved, without unit conversion. A missing naval capacity is reported as unavailable. Do not assume every value is complete or that this covers all versions, DLCs, or modded saves.

Run `go test ./...` for synthetic tests. To additionally test a local save without modifying it, set `STELLARIS_TEST_SAVE` to an absolute `.sav` path before running `go test -v ./internal/gamestate`. Never commit or upload save files or personal save data.
