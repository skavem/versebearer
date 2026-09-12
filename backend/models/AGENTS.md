<!-- Parent: ../AGENTS.md -->
<!-- Generated: 2026-05-22 | Updated: 2026-05-25 -->

# models

## Purpose
GORM struct definitions used both as DB rows and as serialized payloads sent to the Svelte frontend via Wails bindings.

## Key Files
| File | Description |
|------|-------------|
| `models.go` | All entity structs: `Translation`, `Book`, `Chapter`, `Verse`, `Song`, `Couplet`, `Screen`, `GlobalState`, `Font` |

## For AI Agents

### Working In This Directory
- All structs embed `gorm.Model` (gives `ID`/`CreatedAt`/`UpdatedAt`/`DeletedAt uint/Time`). Frontend bindings expose `ID` as `number`.
- Hierarchy: `Translation → Books → Chapters → Verses` and `Song → Couplets`. Each level has `<Parent>Id uint` + a `<Children> []Child` slice. The slice is only populated when explicitly `Preload`-ed or hand-assigned (see `dbHandler.go` `GetTranslations`/`GetBooks`).
- `Book.DividerBefore *string` is a pointer because nil = no section divider; the frontend (`Bible.svelte`) inserts a synthetic divider list-item when present.
- `Couplet.Number` is the ordering key inside a song. The list ordering in DB queries uses `ORDER BY number ASC` (see `addAscByNumber` in `dbHandler.go`).
- `Screen` is scaffolding (not populated — screen list comes from `@wailsio/runtime` `Screens.GetAll()`).
- `GlobalState` is a single-row table (`ID=1`, `FirstOrCreate` in `backend/inits`) holding the projector visual style. 10 flat columns per target with `verse_*` / `couplet_*` prefix: `bg_color`, `bg_opacity`, `text_color`, `font_id`, `border_color`, `border_width`, `border_radius`, `border_style`, `padding`, `margin`, `text_shadow`. Plus `version` for staged defaults seeding (`<"2"` seeds full style, `<"3"` patches margin). Verse and couplet styles are independent — never share columns.
- `Font` holds uploaded `.woff2`/`.ttf` BLOBs (`Data []byte` with `json:"-"` so binary is NEVER sent over the Wails bridge). Frontend bindings expose `name`, `mimeType`, `sizeBytes`, `ID`; actual font bytes are served by the SSE process at `/font/{id}.{ext}` (see `sse.go`). Style FK is `<target>_font_id *uint` — pointer because nil = default ("Century Gothic").
- `AudioTrack` — a media-library row for the "Звук" tab (Фонограммы feature). The file itself lives on disk under `paths.MediaDir` as `<hash>.<ext>`; only that name is stored (`FileName`), never the blob — unlike `Font`/`Image`, an audio file is tens of megabytes and GORM would otherwise pull it fully into memory on every list read. `Hash` (sha256 of the source, first 16 bytes hex) is indexed and is what import dedup checks against. `TrimStartMs`/`TrimEndMs` (`0` = play to end) and `GainDb` live on the track, not on `PlaylistItem`: silence at the head of a soundtrack, or its measured loudness, is a property of the file and identical in every playlist that references it.
- `Playlist` covers both an ordinary playlist and a "pre-service background" list — they differ only in flags (`AutoAdvance`, `Loop`, `FadeMs`), so there is no separate entity for the latter. `Shuffle` was deliberately never added (user decision, see `.omc/plans/audio-playlist.md`).
- `PlaylistItem` is the join row between a `Playlist` and an `AudioTrack`. `Position` orders it within the playlist (1..n, same convention as `Couplet.Number`). `PlaylistId`/`TrackId` are indexed — playlist loading queries by the former, `RemoveTrack` cleanup by the latter.

### Common Patterns
- JSON tags are camelCase (e.g. `shortName`, `dividerBefore`) — keep that when adding fields, or the TS bindings will mismatch the Svelte stores.
- Numeric "kind" fields are plain `int`. Time-style is `gorm.Model`'s defaults only.

## Dependencies

### Internal
- Imported by `backend/inits` (AutoMigrate), `backend/filler` (seeding), root `dbHandler.go`.

### External
- `gorm.io/gorm` — only for `gorm.Model`.

<!-- MANUAL: -->
