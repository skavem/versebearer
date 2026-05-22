<!-- Parent: ../AGENTS.md -->
<!-- Generated: 2026-05-22 | Updated: 2026-05-22 -->

# backend

## Purpose
Go backend layer. Holds GORM model definitions, the global `*gorm.DB` connection initialized via package-init side effect, and a standalone seeder binary that imports `Bible.json` + `songs.json` into the SQLite DB.

## Subdirectories
| Directory | Purpose |
|-----------|---------|
| `models/` | GORM struct definitions (Translation/Book/Chapter/Verse/Song/Couplet/Screen/GlobalState). See `models/AGENTS.md` |
| `inits/` | DB connection + AutoMigrate, runs via `init()`. See `inits/AGENTS.md` |
| `filler/` | Standalone `package main` that seeds SQLite from `Bible.json`/`songs.json`. See `filler/AGENTS.md` |

## For AI Agents

### Working In This Directory
- Module imports use `changeme/backend/...` because `go.mod` is `module changeme`.
- The `inits` package opens `test.db` in the CWD via `init()` — any package that imports it triggers DB open + AutoMigrate. That's intentional: `main.go`, `dbHandler.go`, and `filler/fillDb.go` all rely on it.
- Adding a new model: add struct to `models/`, then register it in `inits/db.go`'s `AutoMigrate` call. Skip the AutoMigrate registration and the table won't exist.

### Common Patterns
- All entity structs embed `gorm.Model` — gives `ID`, `CreatedAt`, `UpdatedAt`, `DeletedAt`.
- Foreign keys are named `<Parent>Id` (uint), matching GORM conventions. JSON tags use camelCase for the frontend.

## Dependencies

### Internal
- Consumed by root-level `dbHandler.go` (uses `models` + `inits.DB`).

### External
- `gorm.io/gorm`, `gorm.io/driver/sqlite`.

<!-- MANUAL: -->
