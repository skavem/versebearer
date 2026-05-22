<!-- Parent: ../AGENTS.md -->
<!-- Generated: 2026-05-22 | Updated: 2026-05-22 -->

# inits

## Purpose
Opens the SQLite DB at `test.db` (CWD-relative) and runs `AutoMigrate` for every model. Exports the singleton `DB *gorm.DB` consumed everywhere.

## Key Files
| File | Description |
|------|-------------|
| `db.go` | `var DB *gorm.DB` + `init()` that opens `test.db` and AutoMigrates all models |

## For AI Agents

### Working In This Directory
- The DB is opened in a package `init()` — importing this package has the side effect of opening/creating `test.db` in the current working directory. That means dev runs and the filler binary all touch the *same* file relative to wherever they're invoked from.
- New models added under `backend/models/` MUST be added to the `AutoMigrate` call here; otherwise GORM won't create their tables and queries will fail at runtime.
- Failure to open the DB calls `panic("failed to connect database")` — there is no retry/fallback. Don't add error suppression; failing fast is intentional.
- DB file path is hardcoded. If you need a configurable path, plumb it through env (`os.Getenv("DB_PATH")`) and update both this file and `backend/filler/fillDb.go`.

## Dependencies

### Internal
- `changeme/backend/models` for `AutoMigrate` arguments.

### External
- `gorm.io/gorm`, `gorm.io/driver/sqlite`.

<!-- MANUAL: -->
