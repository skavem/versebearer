<!-- Parent: ../AGENTS.md -->
<!-- Generated: 2026-05-22 | Updated: 2026-05-22 -->

# backend

## Purpose
Go backend layer. Holds GORM model definitions, the global `*gorm.DB` connection (opened explicitly from `main()`, see `inits/`), the user-data-directory resolver (`paths/`), and a standalone seeder binary that imports `Bible.json` + `songs.json` into the SQLite DB.

## Subdirectories
| Directory | Purpose |
|-----------|---------|
| `models/` | GORM struct definitions (Translation/Book/Chapter/Verse/Song/Couplet/Screen/GlobalState). See `models/AGENTS.md` |
| `inits/` | `Open()`/`OpenAt()` + AutoMigrate + legacy-`test.db` migration, called explicitly from `main()`/`filler`. See `inits/AGENTS.md` |
| `paths/` | Resolves the user data directory (SQLite DB, Bleve index, imported audio) — `%LOCALAPPDATA%\versebearer` by default, overridable via `VERSEBEARER_DATA`. No `AGENTS.md` of its own; see the package doc comment in `paths.go` |
| `filler/` | Standalone `package main` that seeds SQLite from `Bible.json`/`songs.json`. See `filler/AGENTS.md` |

## For AI Agents

### Working In This Directory
- Module imports use `changeme/backend/...` because `go.mod` is `module changeme`.
- The `inits` package has **no package `init()`** — importing it has zero side effects. The DB is opened by an explicit `inits.Open()` call, made once from `main()` (before `application.New`) and once from `filler/fillDb.go`'s own `main()`. Both call the same function; neither relies on import order.
- Adding a new model: add struct to `models/`, then register it in `inits/db.go`'s `AutoMigrate` call (inside `OpenAt`). Skip the AutoMigrate registration and the table won't exist.

### Common Patterns
- All entity structs embed `gorm.Model` — gives `ID`, `CreatedAt`, `UpdatedAt`, `DeletedAt`.
- Foreign keys are named `<Parent>Id` (uint), matching GORM conventions. JSON tags use camelCase for the frontend.

## Dependencies

### Internal
- Consumed by root-level `dbHandler.go` (uses `models` + `inits.DB`).

### External
- `gorm.io/gorm`, `gorm.io/driver/sqlite`.

<!-- MANUAL: -->
