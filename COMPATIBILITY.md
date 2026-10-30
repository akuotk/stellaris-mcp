# Compatibility notes

This fork adds preliminary support for Stellaris 4.5 colony and population records. It reads colony data through verified planet-to-colony references and reports population values in raw save units; it does not assume a conversion to the game's displayed POP counts.

The military-class total only includes fleets whose `ship_class` is `shipclass_military`. Ship designs can have multiple sections. Spy network power is reported as a raw save value; infiltration conversion and operation availability are not established.

Limitations: non-planet carriers such as Nomad Arkships are not yet resolved as colonies; translated names, population ethic/faction distributions, war naming, federations and espionage require further work. A missing naval capacity is reported as unavailable. Do not assume every value is complete or that this covers all versions, DLCs, or modded saves.

Run `go test ./...` for synthetic tests. To additionally test a local save without modifying it, set `STELLARIS_TEST_SAVE` to an absolute `.sav` path before running `go test -v ./internal/gamestate`. Never commit or upload save files or personal save data.
