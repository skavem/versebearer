<!-- Generated: 2026-05-22 | Updated: 2026-05-25 -->

# versebearer

## Purpose
Wails3 desktop app for showing Bible verses and Christian song couplets on external screens during church services. Operator runs the Wails window (SvelteKit UI) to pick verses/songs/couplets. Picks are pushed to a separate "reciever" Svelte page via Server-Sent Events on `:9093`, which renders fullscreen text on projector windows. SQLite (GORM) stores Bible translations, books, chapters, verses, songs, and couplets.

## Key Files
| File | Description |
|------|-------------|
| `main.go` | Wails3 entrypoint. Wires `DbHandler` service, starts SSE goroutine on `:9093`, opens main webview window |
| `dbHandler.go` | Wails-exposed service. CRUD + show/hide for verses/couplets/QR/screens. Emits Wails events + pushes to SSE channels |
| `SSE.go` | r3labs SSE server on `:9093`. Serves embedded `reciever/dist` and broadcasts `show_verse`/`show_couplet`/`show_qr`/`sync` events |
| `Taskfile.yml` | Top-level Task runner. Delegates to per-OS files under `build/` |
| `go.mod` | Module name `changeme`. Wails v3 alpha, GORM + SQLite driver, r3labs/sse, godotenv |
| `Bible.json` | Seed Synodal translation (books/chapters/verses) consumed by `backend/filler` |
| `songs.json` | Seed song dump consumed by `backend/filler` (untracked, in `.gitignore`) |
| `test.db` | Local SQLite DB (created by `backend/inits` at startup, ignored) |
| `.env` | `DEV=true` toggles SSE static-file serving from disk vs embedded FS |

## Subdirectories
| Directory | Purpose |
|-----------|---------|
| `backend/` | Go layer: GORM models, DB init, JSON seeder (see `backend/AGENTS.md`) |
| `frontend/` | Operator SvelteKit UI compiled to `frontend/dist`, embedded via `//go:embed all:frontend/dist` (see `frontend/AGENTS.md`) |
| `reciever/` | Projector-output Svelte app compiled to `reciever/dist`, embedded into the Go binary and served on `:9093` (see `reciever/AGENTS.md`) |
| `build/` | Wails3 build pipeline: per-OS Taskfiles, NSIS installer, nfpm Linux packaging, AppImage, icons (see `build/AGENTS.md`) |

## For AI Agents

### Working In This Directory
- Module path is `changeme` — do not rename without updating every `changeme/...` import.
- Four independent event channels: `bibleChannel` (verses), `songChannel` (couplets), `qrChannel` (QR toggle), `styleChannel` (visual style + fonts). The `broadcaster[T]` instances on `DbHandler` (`verseB`, `coupletB`) are the only producers for verse/couplet — they push to the channel AND emit a Wails event in one `.show(v)`/`.hide()` call. QR stays as ad-hoc `chan *bool` (different shape, only 2 callsites). `styleChannel` carries `*StyleEvent{Type, Target, Style, Fonts}` — produced by `Update*Style` / `Reset*Style` / `UploadFont` / `DeleteFont`. `SSE.go:watchChannels` keeps a `lastVerseStyle` / `lastCoupletStyle` / `lastFonts` cache (seeded from DB on startup) so `sync` events include the current style without a DB hit.
- "Визуал" tab lets the operator customize verse and couplet projector styling independently (bg color/opacity, text color, font upload, border, padding, margin, text-shadow). Edits flow through `UpdateVerseStyle` / `UpdateCoupletStyle` (debounced 150ms in the UI), persist into the `GlobalState` row, and broadcast over `styleChannel` → SSE → receiver applies via `style:*` directives. See `backend/models/AGENTS.md` for the column layout.
- `broadcaster[T any]` (in `dbHandler.go`) is an unexported generic that owns: state pointer, channel, show/hide event names, and an `emit` callback. To add a new show/hide entity, construct another `broadcaster[YourType]` in `main.go` and wire it the same way as `verseB`/`coupletB`. Don't make broadcasters exported — Wails service reflection scans exported methods, not fields, so unexported is safe.
- `findByParent[T any](field, parentId, order)` (unexported package-level in `dbHandler.go`) is the generic GORM helper for child lookups. Each `getX` private method delegates to it. **Keep generics unexported.** Wails binding generation walks exported methods and needs concrete return types — exported generic methods would either fail to generate or produce `any`-typed JS classes.
- `main()` constructs channels and `DbHandler` *before* `application.New` then assigns `dbHandler.app = app` after — `app` is nil until then. The `emit(name, data)` wrapper on `DbHandler` guards `if g.app != nil` so it's safe to call during construction / tests without an app. Broadcasters take `dbHandler.emit` as a method-value callback (the binding picks up the assigned `app` once it's set).
- Wails v3 manager-pattern API in use: `app.Event` (`EventManager`), `app.Window` (`NewWithOptions` / `GetByName(name) (Window, bool)`). Do not revert to the pre-alpha.12 flat methods (`EmitEvent`, `NewWebviewWindowWithOptions`, `GetWindowByName`).
- `app.Event.Emit(name, single)` sets the JS `data` to `single` directly. JS handlers read `({data}) => ...`, NOT `data[0]`. Backend code that emits two+ args produces a JS array.
- New Wails-exposed methods on `DbHandler` need `wails3 generate bindings` (`task common:generate:bindings`) to refresh `frontend/src/lib/bindings/`.
- SSE port `9093` is hardcoded in `SSE.go`. The projector windows open `http://localhost:9093` directly (`DbHandler.ShowScreen`).
- `couplet.Number` is the order field. `CreateCouplet` shifts all `>=number` up by one before insert **scoped to the same `song_id`** — the where-clause includes both `song_id = ? AND number >= ?` (a missing song_id filter was a real bug, fixed 2026-05-23). `RemoveCouplet` re-numbers `1..n` after delete. `UpdateCouplet` does NOT renumber — UI swaps numbers pair-wise via two `UpdateCouplet` calls for reordering.
- `ReplaceCouplets(songId, []CoupletInput)` is the bulk-edit endpoint: atomically deletes all couplets for the song and re-inserts the supplied blocks with fresh `Number = i+1`. Hides the active shown couplet first if it belongs to the song (all IDs change, so the old ref can't survive). Emits `song_update` once at the end. `CoupletInput{Label, Text}` is the wire type — exported so Wails bindings produce a TS class.
- Song CRUD: `CreateSong(number, title) *models.Song` — returns the created song so the UI can set it active without waiting for `songs_update`. `RemoveSong(songId)` — hides the active couplet first if it belongs to the deleted song, then cascades couplet delete + song delete + `songs_update` emit. No song-number renumber (numbers are free-form, not contiguous).

### Testing Requirements
- Go smoke tests live in `dbHandler_test.go` (in-memory SQLite, no Wails dependency thanks to `emit` nil-safety). Run via `task test` (alias for `go test ./...`). Cover: `GetTranslations` preload depth, `CreateCouplet` song-scoped renumber, `RemoveCouplet` 1..n renumber.
- Manual UI: `wails3 dev` or `task dev` and exercise the three tabs + projector.
- Frontend type-check: `cd frontend && npm run check`.
- Reciever type-check: `cd reciever && npm run check`.

### Common Patterns
- Wails bindings live at `frontend/src/lib/bindings/changeme/...` — autogenerated, do not hand-edit.
- Go side uses uppercase exported method names; the binding gen produces matching TS functions.
- All public `DbHandler` methods that take numeric IDs use `float32` because the Wails JS bridge serializes JS numbers as float — cast to `uint` inside.

## Dependencies

### External (Go)
- `github.com/wailsapp/wails/v3` v3.0.0-alpha.95 — desktop shell. Requires Go toolchain ≥ 1.25 (auto-fetched via `go.mod` toolchain directive).
- `gorm.io/gorm` + `gorm.io/driver/sqlite` — ORM + SQLite (uses `mattn/go-sqlite3` CGO driver)
- `github.com/r3labs/sse/v2` — SSE server for reciever
- `github.com/joho/godotenv` — `.env` loader

### External (Build)
- `wails3` CLI — bindings, syso, icons, dev runner
- `task` (go-task) — build orchestration
- NSIS (Windows installer), nfpm (Linux pkgs), AppImage tools

<!-- MANUAL: -->
