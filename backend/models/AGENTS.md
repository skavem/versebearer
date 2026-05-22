<!-- Parent: ../AGENTS.md -->
<!-- Generated: 2026-05-22 | Updated: 2026-05-22 -->

# models

## Purpose
GORM struct definitions used both as DB rows and as serialized payloads sent to the Svelte frontend via Wails bindings.

## Key Files
| File | Description |
|------|-------------|
| `models.go` | All entity structs: `Translation`, `Book`, `Chapter`, `Verse`, `Song`, `Couplet`, `Screen`, `GlobalState` |

## For AI Agents

### Working In This Directory
- All structs embed `gorm.Model` (gives `ID`/`CreatedAt`/`UpdatedAt`/`DeletedAt uint/Time`). Frontend bindings expose `ID` as `number`.
- Hierarchy: `Translation → Books → Chapters → Verses` and `Song → Couplets`. Each level has `<Parent>Id uint` + a `<Children> []Child` slice. The slice is only populated when explicitly `Preload`-ed or hand-assigned (see `dbHandler.go` `GetTranslations`/`GetBooks`).
- `Book.DividerBefore *string` is a pointer because nil = no section divider; the frontend (`Bible.svelte`) inserts a synthetic divider list-item when present.
- `Couplet.Number` is the ordering key inside a song. The list ordering in DB queries uses `ORDER BY number ASC` (see `addAscByNumber` in `dbHandler.go`).
- `Screen` and `GlobalState` exist as models but are not actively populated — screen list comes from `@wailsio/runtime` `Screens.GetAll()` on the frontend, not the DB. Treat them as scaffolding for a future persisted-layout feature.

### Common Patterns
- JSON tags are camelCase (e.g. `shortName`, `dividerBefore`) — keep that when adding fields, or the TS bindings will mismatch the Svelte stores.
- Numeric "kind" fields are plain `int`. Time-style is `gorm.Model`'s defaults only.

## Dependencies

### Internal
- Imported by `backend/inits` (AutoMigrate), `backend/filler` (seeding), root `dbHandler.go`.

### External
- `gorm.io/gorm` — only for `gorm.Model`.

<!-- MANUAL: -->
