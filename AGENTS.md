<!-- Generated: 2026-05-22 | Updated: 2026-05-25 -->

# versebearer

## Purpose
Wails3 desktop app for showing Bible verses and Christian song couplets on external screens during church services. Operator runs the Wails window (SvelteKit UI) to pick verses/songs/couplets. Picks are pushed to a separate "reciever" Svelte page via Server-Sent Events on `:9093`, which renders fullscreen text on projector windows. SQLite (GORM) stores Bible translations, books, chapters, verses, songs, and couplets.

## Key Files
| File | Description |
|------|-------------|
| `main.go` | Wails3 entrypoint. Wires `DbHandler` service, starts SSE goroutine on `:9093`, opens main webview window |
| `dbHandler.go` | Wails-exposed service. CRUD + show/hide for verses/couplets/QR/screens. Emits Wails events + pushes to SSE channels |
| `sse.go` | r3labs SSE server on `:9093`. Serves embedded `reciever/dist` and broadcasts `show_verse`/`show_couplet`/`show_qr`/`sync` events |
| `Taskfile.yml` | Top-level Task runner. Delegates to per-OS files under `build/` |
| `go.mod` | Module name `changeme`. Wails v3 alpha, GORM + SQLite driver, r3labs/sse, godotenv |
| `db_import.go` | Translation import/removal exposed to the UI: native file dialog picker, file inspection, transactional import, cascade delete, translation list for settings |
| `db_search.go` | Wails-exposed full-text search over verses and couplets (`SearchVerses`, `SearchCouplets`, `RebuildSearchIndex`) plus the index-sync hooks called from song/import mutations |
| `db_reference.go` | Reference parser: turns «Ин 3:16», «1 Кор. 13», «быт1:1» into a book/chapter/verse jump. Independent of the full-text index |
| `search.bleve` | Bleve index directory (created at startup, ignored). Fully derived from `test.db` — safe to delete, rebuilds on next launch |
| `translations/` | Distributable translation files for the in-app importer. **Only public-domain or freely licensed texts belong here** — currently Synodal, both Elizabethan editions (public domain) and the Open Bible (CC BY-SA 4.0). Copyrighted translations must not be committed: source sites publish them under a permission granted to that site alone. See README for the per-translation rights table |
| `Bible.json` | Seed Synodal translation (books/chapters/verses) consumed by `backend/filler` |
| `songs.json` | Seed song dump consumed by `backend/filler` (untracked, in `.gitignore`) |
| `test.db` | Local SQLite DB (created by `backend/inits` at startup, ignored) |
| `.env` | Optional, dev-only. Holds `DEV=false` by default. `DEV=true` makes SSE serve the receiver from `./reciever/dist` on disk instead of the embedded FS. The `dev` task (`Taskfile.yml`) sets `DEV=true` in-env, and `godotenv.Load()` is non-overriding so that wins over `.env` — i.e. `task dev` serves the receiver from disk (live with the `common:dev:reciever` watcher), while a normal `task build`/packaged run never sets `DEV` and keeps the embedded copy. Absent in production builds — `godotenv.Load()` is best-effort, a missing file is not fatal |

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
- Four independent event channels: `bibleChannel` (verses), `songChannel` (couplets), `qrChannel` (QR toggle), `styleChannel` (visual style + fonts). The `broadcaster[T]` instances on `DbHandler` (`verseB`, `coupletB`) are the only producers for verse/couplet — they push to the channel AND emit a Wails event in one `.show(v)`/`.hide()` call. QR stays as ad-hoc `chan *bool` (different shape, only 2 callsites). `styleChannel` carries `*StyleEvent{Type, Target, Style, Fonts}` — produced by `Update*Style` / `Reset*Style` / `UploadFont` / `DeleteFont`. `sse.go:watchChannels` keeps a `lastVerseStyle` / `lastCoupletStyle` / `lastFonts` cache (seeded from DB on startup) so `sync` events include the current style without a DB hit.
- "Визуал" tab lets the operator customize verse and couplet projector styling independently (bg color/opacity, text color, font upload, border, padding, margin, text-shadow). Edits flow through `UpdateVerseStyle` / `UpdateCoupletStyle` (debounced 150ms in the UI), persist into the `GlobalState` row, and broadcast over `styleChannel` → SSE → receiver applies via `style:*` directives. See `backend/models/AGENTS.md` for the column layout.
- `broadcaster[T any]` (in `dbHandler.go`) is an unexported generic that owns: state pointer, channel, show/hide event names, and an `emit` callback. To add a new show/hide entity, construct another `broadcaster[YourType]` in `main.go` and wire it the same way as `verseB`/`coupletB`. Don't make broadcasters exported — Wails service reflection scans exported methods, not fields, so unexported is safe.
- `findByParent[T any](field, parentId, order)` (unexported package-level in `dbHandler.go`) is the generic GORM helper for child lookups. Each `getX` private method delegates to it. **Keep generics unexported.** Wails binding generation walks exported methods and needs concrete return types — exported generic methods would either fail to generate or produce `any`-typed JS classes.
- `main()` constructs channels and `DbHandler` *before* `application.New` then assigns `dbHandler.app = app` after — `app` is nil until then. The `emit(name, data)` wrapper on `DbHandler` guards `if g.app != nil` so it's safe to call during construction / tests without an app. Broadcasters take `dbHandler.emit` as a method-value callback (the binding picks up the assigned `app` once it's set).
- Wails v3 manager-pattern API in use: `app.Event` (`EventManager`), `app.Window` (`NewWithOptions` / `GetByName(name) (Window, bool)`). Do not revert to the pre-alpha.12 flat methods (`EmitEvent`, `NewWebviewWindowWithOptions`, `GetWindowByName`).
- `app.Event.Emit(name, single)` sets the JS `data` to `single` directly. JS handlers read `({data}) => ...`, NOT `data[0]`. Backend code that emits two+ args produces a JS array.
- New Wails-exposed methods on `DbHandler` need `wails3 generate bindings` (`task common:generate:bindings`) to refresh `frontend/src/lib/bindings/`.
- Translation import (`db_import.go`) reads the JSON **on the Go side**: `PickTranslationFile` opens the native dialog, inspects the file and returns only path + counts; `ImportTranslationFromFile(path, name, shortName)` does the write. A full translation is ~6 MB — deliberately never sent through the webview bridge, unlike `UploadFont`/`UploadImage` which pass base64 for their much smaller payloads. `ImportTranslation(name, shortName, data []byte)` stays the byte-level core (what the tests drive).
- Import file layout: `{name, shortName, source, books:[{dividerBefore, name, fullName, content[chapter][verse]}]}`. A bare array of books (the original `Bible.json`) is still accepted — then the name must come from the caller. Empty strings inside `content` are verses the translation lacks: they are skipped on insert but keep the surrounding verse numbers correct, so `content` index +1 always equals `Verse.Number`.
- The whole import runs in one transaction — an aborted import leaves nothing behind. Duplicate translation **names** are rejected up front (there is no unique index on the column; the check is in `ImportTranslation`).
- `RemoveTranslation` refuses to delete the last remaining translation and hides the shown verse first if it belongs to the doomed translation. Deletion is manual cascade (verses → chapters → books → translation) because the models carry no FK constraints.
- Import/removal emit `translations_update` (no payload) and per-book `import_progress` (`{name, done, total, book}`). `BibleStore` reloads on the former; `TranslationsSettings.svelte` drives its progress bar off the latter.
- SSE port `9093` is hardcoded in `sse.go`. The projector windows open `http://localhost:9093` directly (`DbHandler.ShowScreen`).
- `couplet.Number` is the order field. `CreateCouplet` shifts all `>=number` up by one before insert **scoped to the same `song_id`** — the where-clause includes both `song_id = ? AND number >= ?` (a missing song_id filter was a real bug, fixed 2026-05-23). `RemoveCouplet` re-numbers `1..n` after delete. `UpdateCouplet` does NOT renumber — UI swaps numbers pair-wise via two `UpdateCouplet` calls for reordering.
- `ReplaceCouplets(songId, []CoupletInput)` is the bulk-edit endpoint: atomically deletes all couplets for the song and re-inserts the supplied blocks with fresh `Number = i+1`. Hides the active shown couplet first if it belongs to the song (all IDs change, so the old ref can't survive). Emits `song_update` once at the end. `CoupletInput{Label, Text}` is the wire type — exported so Wails bindings produce a TS class.
- Song CRUD: `CreateSong(number, title) *models.Song` — returns the created song so the UI can set it active without waiting for `songs_update`. `RemoveSong(songId)` — hides the active couplet first if it belongs to the deleted song, then cascades couplet delete + song delete + `songs_update` emit. No song-number renumber (numbers are free-form, not contiguous).

### Dev mode (`Taskfile.yml` + `build/config.yml` + `frontend/vite.config.ts`)
- `task dev` builds via **`build:dev`**, not `build` — the only difference is `PRODUCTION: "false"`, and it is what makes frontend hot-reload work at all. Without the `production` tag Wails compiles the dev branch of its assetserver, which reads `FRONTEND_DEVSERVER_URL` (set by `wails3 dev`) and reverse-proxies the frontend to vite. With the tag, that same function returns `""`, the window serves the `frontend/dist` embedded in the binary, and Svelte edits only appear after a Go rebuild. The global `PRODUCTION: "true"` var still governs `task build` and the NSIS installer, so release paths are unaffected.
- **Vite must listen on IPv4.** Wails' dev proxy forces `tcp4` when the dev-server host is `localhost`, while vite defaults to `[::1]` — the mismatch shows up as `HTTP ERROR 502` in the window and `dial tcp4 127.0.0.1:9245: connectex` in the log. Hence `server.host: '127.0.0.1'`. Wails' own "Connected to frontend dev server!" check does *not* catch this: it uses a normal client that happily reaches IPv6.
- **HMR bypasses Wails.** The page loads from `wails.localhost:9245` (Wails rewrites the start URL to carry the vite port), but the assetserver answers WebSocket upgrades with 501, so `server.hmr` points the client straight at `127.0.0.1:9245`.
- The dev watcher only watches `*.go` (`dev_mode.watched_extension`). That is correct *because* the frontend is live over vite — Go changes are the only ones needing a rebuild. Touching a `.go` file's mtime does not trigger it; the watcher wants a content change.

### Full-text search (`backend/search`)
- The index is **derived state**: everything in `search.bleve` can be rebuilt from SQLite. `openSearchIndex` (called from `main` *after* `dbHandler.app` is assigned, so progress events aren't swallowed) rebuilds in a goroutine whenever the index is empty. `RebuildSearchIndex` is also exposed to the UI.
- **Russian morphology is done by symmetric expansion, not lemmatisation.** `gomorphy` has no lemma API — only `WordForms`, and it is *not* invariant to the input form: `WordForms("любовь")` misses "любви" while `WordForms("любви")` contains "любовь". So every word is expanded into the snowball stems of all its word forms, on **both** sides — at index time and at query time — and a match is a non-empty intersection. Any change that expands only one side silently breaks the most common words. See the package doc in `morph.go`.
- Words outside the OpenCorpora dictionary (proper names, «рече», «глаголющий») degrade to plain stemming rather than disappearing. Suppletion across *derivation* («говорить»→«сказал», «верить»→«уверовал») and Church-Slavonic archaisms are **not** covered by design — that needs a synonym dictionary, which the project deliberately doesn't have.
- Two fields per document: `morph` (expanded stems, the actual retrieval field) and `text` (normalised original, used only for the exact-word and exact-phrase boosts). Both are non-stored — result text is read back from SQLite by ID. Without those boosts ranking is visibly wrong: OpenCorpora puts participles in the verb's paradigm, so «возлюбил» ties with «возлюбленному».
- Bleve analyzers register via **blank imports** (`analysis/analyzer/simple`, `.../keyword`). Dropping them fails at index *creation* with «no analyzer with name 'simple' registered».
- `search.Match` offsets are in **runes, not bytes** — they cross into JS, which slices by UTF-16 units. All translation text is BMP, where rune == UTF-16 unit. Highlighting is computed in Go (`Highlight`) rather than by Bleve's highlighter, because the index holds stems and Bleve would highlight «возлюб» instead of the live word.
- `Normalize` applies NFC before casefolding: the Open Bible stores «й»/«ё» decomposed (и + U+0306), and without it plain Russian queries miss visually identical text. Never normalise a whole text before computing offsets — NFC changes string length and shifts every following position.
- **Any new couplet mutation must call the index hooks** (`indexCoupletSilent` / `unindexCoupletSilent` / `reindexSongSilent`), and any that deletes rows must collect the ids *before* deleting. The hooks are deliberately silent: a stale search row must never fail the operator's actual action.

### Testing Requirements
- Go smoke tests live in `dbHandler_test.go` (in-memory SQLite, no Wails dependency thanks to `emit` nil-safety). Run via `task test` (alias for `go test ./...`). Cover: `GetTranslations` preload depth, `CreateCouplet` song-scoped renumber, `RemoveCouplet` 1..n renumber.
- Search tests live in `backend/search/search_test.go` (temp-dir Bleve index, no SQLite). They pin the behaviours that are easy to break silently: symmetric expansion on «любовь»/«любви» and «человек»/«людей», rune-based highlight offsets, exact-form ranking, translation/kind filtering, layout and typo fallbacks. `TestArchaicFormsAreNotLinked` records a known limitation rather than a bug.
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
- `github.com/blevesearch/bleve/v2` — pure-Go full-text index (BM25). No CGO, no build tags — unlike SQLite FTS5, which would have needed `-tags sqlite_fts5` in every per-OS Taskfile
- `github.com/jus1d/gomorphy` — Russian morphology, OpenCorpora dictionary embedded at compile time (~8.8 MB of the binary)
- `github.com/kljensen/snowball` — Russian stemmer, applied on top of the word forms
- `github.com/r3labs/sse/v2` — SSE server for reciever
- `github.com/joho/godotenv` — `.env` loader

### External (Build)
- `wails3` CLI — bindings, syso, icons, dev runner
- `task` (go-task) — build orchestration
- NSIS (Windows installer), nfpm (Linux pkgs), AppImage tools

<!-- MANUAL: -->
