<!-- Parent: ../AGENTS.md -->
<!-- Generated: 2026-05-22 | Updated: 2026-05-22 -->

# inits

## Purpose
Opens the SQLite DB at `paths.DBPath()` (`<DataDir>/versebearer.db`, see `backend/paths`), migrating a legacy `test.db` found next to the program on first run, and runs `AutoMigrate` + version-numbered migration blocks for every model. Exports the singleton `DB *gorm.DB` consumed everywhere.

## Key Files
| File | Description |
|------|-------------|
| `db.go` | `var DB *gorm.DB`, `Open()` (path + migration + `OpenAt`), `OpenAt(path)` (AutoMigrate + `versionLT` blocks) |
| `migrate.go` | Legacy `test.db` discovery and copy-migration (`legacyDBCandidates`, `findLegacyDB`, `looksLikeOurDB`, `migrateLegacyDB`, `copyLegacyDB`, `verifyCopy`, `openReadPreferRO`) and the typed, `DialogBody()`-carrying errors (`AmbiguousCandidatesError`, `CandidateUnreadableError`, `CorruptDestinationError`, `MigrationFailedError`, `OpenFailedError`) |

## For AI Agents

### Working In This Directory
- **No package `init()`.** Importing this package has zero side effects — opening the DB is an explicit call (`Open()` from `main()`, or `OpenAt(path)` directly in tests). This is a deliberately enforced invariant: `TestNoInitFunc` (`migrate_test.go`) parses every non-test `*.go` file in this directory via `go/parser` and fails on any top-level `func init()`. A naive guard like snapshotting `DB` at package-var init time does **not** catch a reintroduced `init()` — Go initializes package-level vars before any `init()` runs, so such a snapshot is always the pre-init value regardless.
- New models added under `backend/models/` MUST be added to the `AutoMigrate` call in `OpenAt`; otherwise GORM won't create their tables and queries will fail at runtime.
- `Open()`/`OpenAt()` return errors instead of panicking — `main.go` renders the error as a native message box (`fatalDialog`, see `fatal_windows.go`/`fatal_other.go`) and exits; `backend/filler/fillDb.go` just `log.Fatal`s it. Don't reintroduce `panic` here — the caller decides how to surface the failure.
- **Migration of the old `test.db`** (see `migrate.go`) runs once, inside `Open()`, before `OpenAt`. It looks for `test.db` in the process's CWD then the executable's directory (in that order — CWD wins because that's the only launch mode where the two diverge, see `legacyDBCandidates`), copies it via `VACUUM INTO` into a PID-suffixed `.part` file, verifies the copy, then renames into place. The original is opened read-only and is renamed aside (`.migrated`, numbered `.migrated.2`, `.migrated.3`, ... up to a bounded number of attempts if earlier names are taken — never overwriting one) only *after* the copy is committed — never deleted, never written to except the one sanctioned fallback (a hot `-journal` from a crashed prior run, which read-only can't replay; that fallback is logged, and the same read-then-write-fallback path is shared between candidate inspection and the actual copy via `openReadPreferRO`).
- **Candidate verdict is three states, not two** (`candidateVerdict` in `migrate.go`): `candidateNotOurs` (not SQLite, or SQLite without our tables — silently skipped, a file merely named `test.db` is never fatal), `candidateOurs` (valid, goes in the list), and `candidateUnknown` (couldn't even read it — locked by another process, permission denied). The third state is fatal (`*CandidateUnreadableError`) and must stay fatal: collapsing it into "not ours" (as an earlier version of this code did) means a live database held open by an antivirus/leftover process at the wrong moment gets silently skipped, a fresh empty database gets created in its place, and on the *next* launch that empty database is a valid destination — migration can never run again, and the operator sees an empty program with no error anywhere. More than one valid candidate also refuses to guess and returns `*AmbiguousCandidatesError` — picking wrong here is equally irreversible.
- **`verifyCopy` compares only the tables that actually exist in the source**, not a hardcoded list unconditionally — an archival/older-build source predating `playlists`/`playlist_items`/`audio_tracks`/`outputs` doesn't have those tables at all, and `Count` on a table that doesn't exist returns a SQL error, not zero. The three identifying tables (`ourTables`: `verses`/`couplets`/`global_states`) are still required to be present, so the comparison set can't degenerate to nothing and let a hollow copy through.
- An existing, non-empty destination is used as-is without inspecting its tables (that's the normal fresh-install path, `AutoMigrate` fills it in) — it's inspected only for being valid SQLite at all; a non-SQLite file (or a directory) at the destination path is `*CorruptDestinationError`, fatal, with wording that differs for the directory case.
- Failure to open/migrate the schema at the *chosen* path (`OpenAt`) is `*OpenFailedError`, not a bare `fmt.Errorf` — its dialog text looks for a `.migrated` file next to any legacy candidate location and points the operator at it if the new database turned out to be broken after a successful migration.
- UNC paths (`\\server\share\...`) are handled separately from `filepath.Abs` in `readOnlyDSN`: `filepath.IsAbs` returns `false` for them on Windows, so `Abs` silently substitutes the current drive letter — a real failure mode for a database on a network share, not hypothetical.
- `sweepForeignParts` only removes `.part.*` files older than an hour (`sweepPartMinAge`) — a fresh one could belong to another instance of the program that started moments earlier and is still writing to it; on Windows removing it has no effect on the writer either way, but on linux/darwin an `unlink` on an open file succeeds immediately, and the writer would silently finish writing into a detached inode and then fail the final rename with no useful error.
- **`VERSEBEARER_DATA`** (read by `backend/paths`, see that package's AGENTS.md) is the only way to point `Open()` at a sandbox during dev/tests — set it before the first call in the process (package-level `sync.Once` in `paths`, not resettable via exported API).
- Failure inside a `versionLT` block is the single most expensive class of bug in this package: a lost `gs.Version = "N"` assignment or a reordered block silently resets the operator's theme/Output/screen on every subsequent open, with no error anywhere (see the warning at the top of `db.go`). `TestOpenAtOnUpgradedDBIsNoOp` (`db_test.go`) guards the *no-op-on-already-upgraded-DB* invariant; `TestMigrateFreshInstall` (`migrate_test.go`) separately pins the *content* the version-4/5 blocks are supposed to produce (the seeded default theme and its `IsDefault`/`ActiveThemeId` wiring, the single "Экран" `Output`) — a guard on the invariant alone doesn't catch every block miscounting its own output on the very first pass.

## Dependencies

### Internal
- `changeme/backend/models` for `AutoMigrate` arguments.
- `changeme/backend/paths` for `DBPath()` (used only by `Open()`, never by `migrate.go`'s own functions — those take paths as explicit parameters so migration logic is testable against `t.TempDir()` without ever touching a real `%LOCALAPPDATA%`).

### External
- `gorm.io/gorm`, `gorm.io/driver/sqlite`.

<!-- MANUAL: -->
