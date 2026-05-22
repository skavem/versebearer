<!-- Parent: ../AGENTS.md -->
<!-- Generated: 2026-05-22 | Updated: 2026-05-22 -->

# stores

## Purpose
Three rune-based factory stores backing the three tabs. Not Svelte stores — each `createXxxStore()` returns a plain object with getters/setters that mutate inner `$state` variables.

## Key Files
| File | Description |
|------|-------------|
| `BibleStore.svelte.ts` | Translation → Book → Chapter → Verse cascade. Side-effect on `set active` fetches next level. Subscribes to Wails `show_verse`/`hide_verse` events. Maintains a `history` (most recent first) of every shown verse |
| `songsStore.svelte.ts` | Songs + couplets + favorites + QR. Subscribes to `show_couplet`/`hide_couplet`/`songs_update`/`song_update`. Favorites are local-only (no DB), keyed by random `localId` so the same song can be queued multiple times |
| `screenStore.svelte.ts` | OS displays list (from `@wailsio/runtime` `Screens.GetAll()`) + locally-tracked `activeScreens: string[]` of opened projector window names |

## For AI Agents

### Working In This Directory
- Files use `.svelte.ts` extension so Svelte's preprocessor enables runes (`$state`, `$derived`) outside `.svelte` files.
- Each store is **constructed at module-init** by exporting `export const FooStore = createFooStore()`. Initial data fetch (`GetTranslations().then(...)` / `GetSongs().then(...)` / `Screens.GetAll().then(...)`) happens at module load — there is no explicit "init" call from the UI.
- Setting `.active` triggers downstream fetches (e.g. `books.active = b` → `GetChapters(b.ID)` → updates `chaptersList` + `activeChapter` + clears verses). When adding a new entity level, mirror this cascade.
- `BibleStore.history` is built in-memory only — closing the app loses it. `historyVerses.toReversed()` exposes most-recent-first to the UI.
- `songsStore.favorites` items are `Song & { localId: number }` where `localId = Math.random()`. Two entries for the same `ID` are valid and intentional. Use `localId` for remove/move, NOT `ID`.
- Wails event names from `dbHandler.go` (`show_verse`, `hide_verse`, `show_couplet`, `hide_couplet`, `songs_update`, `song_update`) — keep in sync with the Go side.
- `Events.On("show_verse", ({ data }: { data: ShownVerse[] }) => ...)` — the wails runtime wraps payload in `data[0]`. Single payload always sits at index 0.

### Common Patterns
- Each store exposes sub-objects (`translations`, `books`, `chapters`, `verses`, `history` etc.) so consumers do `BibleStore.verses.next()` rather than `BibleStore.nextVerse()`.
- `next()`/`prev()` find the active index and clamp to bounds — no wrap-around.

## Dependencies

### Internal
- `$lib/bindings/changeme/dbhandler` for backend calls.
- `$lib/bindings/changeme/backend/models` for types.
- `$lib/bindings/changeme` for `ShownVerse`/`ShownCouplet` aggregate types.

### External
- `@wailsio/runtime` `Events` (Bible/Songs) + `Screens` (Screens).

<!-- MANUAL: -->
