# Stellaris MCP

Read-only MCP server over local Stellaris saves. The goal of this fork is that everything a save records can be reached through a tool, with raw saved values kept apart from anything inferred.

## Ground rules
- Never modify the game, mods or saves. Every tool is read-only and must say so (`register_test.go` enforces the annotations and that each save-reading tool exposes `save_id`).
- Nothing derived from a real save (values, names, IDs, file names) goes into commits, fixtures, docs or issues. Tests use synthetic fixtures only.
- Keep these apart in output and wording: raw saved value / static game definition / equal number in a table (a *candidate*) / relation confirmed by a back-reference / guessed meaning or unit. Do not merge "missing", "zero", "duplicate" and "unsupported shape" into one case.
- Do not add a resident cache of parsed saves or arbitrary size limits; raw access with revision and CRC checks must stay complete.

## Build & Test
- `go build ./...` — build all packages
- `go build -trimpath -o bin/stellaris-mcp.exe ./cmd/stellaris-mcp` — build the binary
- `go test -p 1 -count=1 ./...` and `go vet -p 1 ./...` — required before any change is called done; also `GOARCH=386 go vet -p 1 ./...`
- `gofmt -l .` must print nothing (check on an LF checkout; `core.autocrlf=true` makes every file look unformatted)
- Optional local coverage, all read-only and skipped when unset:
  - `STELLARIS_TEST_SAVE_DIR` (a save directory) and `STELLARIS_TEST_SAVE` (one `.sav`) enable the survey/integration tests
  - `STELLARIS_LOC_DIR` in `.env` (see `.env.example`) enables the localization tests
  - `internal/clausewitz/testdata/gamestate` (gitignored) enables the upstream fixture tests
- 32-bit builds reject saved IDs above 2147483647 with an explicit error instead of wrapping; full support needs int64 IDs (not done).

## Architecture
- `cmd/stellaris-mcp/` — entry point. CLI: `stellaris-mcp <save-dir> <loc-dir>`; both are required, `loc-dir` is Steam's `localisation/english/`.
- `internal/clausewitz/` — two parsers over the same format:
  - typed: `Parse` → `Value`, `DecodeValue`/`Unmarshal` into structs via `clausewitz:"key"` tags. Duplicate keys on a non-slice field keep the first value; a block mixing keyed and anonymous entries becomes `Mixed` and cannot be decoded.
  - raw: `ParseRaw` → `RawNode` tree that keeps order, duplicates, mixed blocks, quote/escape flags and source byte ranges; honours context cancellation.
- `internal/gamestate/` — save catalog (`ListSaveFiles`, stable `save_id` = SHA-256 of the relative path), typed `GameState` (`LoadFromZip`), `Load…FromArchive` readers built on the raw tree, country selection, localization, static game-script indexes.
- `internal/tools/` — one file per tool: `registerXxx(s)` + `handleXxx(ctx, req)`, listed in `register.go`. Two families:
  - text tools on the typed model: `loadSave(req)` + `getCountry(gs, req)`, render with a `strings.Builder`.
  - JSON tools on the raw tree: `openRawSave` + a `gamestate.Load…FromArchive` reader, positional paths, `page_context` tokens from `rawPageToken`, results via `rawJSON`.
- Each tool call re-reads and re-parses the selected save; there is no state between calls except the game-script indexes built at startup (restart the server to pick up game updates).

## Shared helpers (`internal/tools/common.go`, `raw_save.go`)
- `requireStringArgs(req, keys...)` — reject non-string values before `GetString`.
- `rawInteger(req, key, default, min, max)` with `maxRecordID` for ID bounds (fits `int` and an exact JSON number).
- `savedInt` / `savedFloat` / `optionalIntRefText` — print optional saved numbers as `not recorded` instead of `0`.
- `l(key)` / `ll(keys)` / `name(n)` — localization. Don't name a variable `l` in a handler; it shadows the helper.
- `toolError(err)` — every handler returns tool errors, never a Go error.
- `gamestate.uniqueSavedFields` (typed) and `gamestate.uniqueRawField` (raw) — detect duplicate keys instead of silently taking the first.

## Key Gotchas
- Clausewitz maps contain `none` entries (e.g. `1=none` in districts) — `decodeMap` skips them.
- Planet `Owner` decodes to `0` when absent, which is also a valid country ID. Use the pointer fields (`OwnerID`) and `isColonized()`.
- Since 4.x `country.owned_planets` holds colony IDs when a `colony` table exists; older saves hold planet IDs. Tools must say which table they resolved against.
- `4294967295` appears in many saved reference fields. Report it raw; its meaning is not verified.
- Names use templates (`PLANET_NAME_FORMAT`, `%ADJECTIVE%`, `$var$`) resolved recursively in `LocalizedName.Display()`; localization values carry markup (`£icon£`, `§X…§!`) removed by `stripMarkup()`.
- Sum floats in a fixed key order. Map iteration order changes the last digits and makes output differ between runs.
- Keep responses paged or summary/detail; large clients cap a response at about 1 MB.

## Adding or changing a tool
1. Create `internal/tools/myfeature.go` (and a `gamestate` reader if needed) and register it in `register.go`.
2. Annotate read-only / non-destructive and expose `save_id`; add `country_id` when the tool is country-scoped.
3. Add synthetic tests for missing / zero / duplicate / malformed / mixed input and for paging tokens.
4. Update the tool table in `README.md` and the limits in `COMPATIBILITY.md`.
5. `sample_outputs/examples.md` is the upstream author's sample for the original tools; do not regenerate it from a private save.
