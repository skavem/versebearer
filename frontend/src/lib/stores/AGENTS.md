<!-- Parent: ../AGENTS.md -->
<!-- Generated: 2026-05-22 | Updated: 2026-05-23c -->

# stores

## Purpose
Three rune-based factory stores backing the three tabs. Not Svelte stores — each `createXxxStore()` returns a plain object with getters/setters that mutate inner `$state` variables.

## Key Files
| File | Description |
|------|-------------|
| `BibleStore.svelte.ts` | Translation → Book → Chapter → Verse cascade. Side-effect on `set active` fetches next level. Subscribes to Wails `show_verse`/`hide_verse` events. Maintains a `history` (most recent first) of every shown verse |
| `songsStore.svelte.ts` | Songs + couplets + favorites + QR. Subscribes to `show_couplet`/`hide_couplet`/`songs_update`/`song_update`. Favorites are local-only (no DB), keyed by random `localId` so the same song can be queued multiple times |
| `screenStore.svelte.ts` | OS displays list (from `@wailsio/runtime` `Screens.GetAll()`) + locally-tracked `activeScreens: string[]` of opened projector window names |
| `cycle.ts` | `cycleIndex<T extends {ID: number}>(list, active, delta)` — shared helper for `next/prev` across chapters/verses/couplets. Returns `undefined` at bounds, no wrap-around |

## For AI Agents

### Working In This Directory
- Files use `.svelte.ts` extension so Svelte's preprocessor enables runes (`$state`, `$derived`) outside `.svelte` files.
- Each store is **constructed at module-init** by exporting `export const FooStore = createFooStore()`. Initial data fetch (`GetTranslations().then(...)` / `GetSongs().then(...)` / `Screens.GetAll().then(...)`) happens at module load — there is no explicit "init" call from the UI.
- Setting `.active` triggers downstream fetches (e.g. `books.active = b` → `GetChapters(b.ID)` → updates `chaptersList` + `activeChapter` + clears verses). When adding a new entity level, mirror this cascade.
- `BibleStore.history` is built in-memory only — closing the app loses it. `historyVerses.toReversed()` exposes most-recent-first to the UI.
- `songsStore.favorites` items are `Song & { localId: number }` where `localId = Math.random()`. Two entries for the same `ID` are valid and intentional. Use `localId` for remove/move, NOT `ID`.
- `songsStore.songs.list` setter preserves the current `active` when the song still exists in the new list (so `songs_update` from create/delete does not clobber the user's selection). It also drops `favorites` entries whose underlying song was deleted. `songs.active` setter is null-safe (passing `null` clears couplets without firing `GetCouplets`).
- Callsites pre-select the next active BEFORE awaiting `RemoveSong`/`CreateSong`: `Songs.svelte#confirmDelete` picks `list[idx-1] ?? list[idx+1]` when the active song is the one being removed; `CreateSongModal#submit` sets `songs.active = created` after the call. The setter then keeps that choice when `songs_update` arrives.
- Wails event names from `dbHandler.go` (`show_verse`, `hide_verse`, `show_couplet`, `hide_couplet`, `songs_update`, `song_update`) — keep in sync with the Go side.
- `Events.On("show_verse", ({ data }: { data: ShownVerse }) => ...)` — Wails v3 `Event.Emit(name, single)` delivers `data` as the value itself (no array wrap). If backend emits multiple args (`Emit(name, a, b)`), `data` is `[a, b]`.

### Common Patterns
- Each store exposes sub-objects (`translations`, `books`, `chapters`, `verses`, `history` etc.) so consumers do `BibleStore.verses.next()` rather than `BibleStore.nextVerse()`.
- `next()`/`prev()` delegate to the shared `cycleIndex(list, active, delta)` helper in `./cycle.ts` — finds active index by `ID`, clamps to bounds, no wrap-around. `favorites.moveUp/moveDown` is NOT a `cycleIndex` consumer (it mutates the array via swap, different semantics).

## Dependencies

### Internal
- `$lib/bindings/changeme/dbhandler` for backend calls.
- `$lib/bindings/changeme/backend/models` for types.
- `$lib/bindings/changeme` for `ShownVerse`/`ShownCouplet` aggregate types.

### External
- `@wailsio/runtime` `Events` (Bible/Songs) + `Screens` (Screens).

<!-- MANUAL: -->
