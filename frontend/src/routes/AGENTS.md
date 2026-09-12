<!-- Parent: ../AGENTS.md -->
<!-- Generated: 2026-05-22 | Updated: 2026-05-23b -->

# routes

## Purpose
SvelteKit routes. The whole operator app lives at `/`, with five tab views imported as plain components (not nested SvelteKit routes).

## Key Files
| File | Description |
|------|-------------|
| `+layout.svelte` | Minimal — imports `app.css`, renders children |
| `+layout.ts` | `prerender = true`, `ssr = false` — static SPA |
| `+page.svelte` | Navbar + tab switcher. `activeTabIndex` default = 1 (Песни/Songs). Tab order: Библия, Песни, Экраны, Визуал, Звук |
| `Bible.svelte` | Bible navigation tab: translation/book/chapter selectors, verse list, recent-history sidebar |
| `Songs.svelte` | Song picker + couplet display tab + favorites list + QR toggle |
| `Screens.svelte` | Lists persisted `Output` entities (`outputStore`); each is assigned an OS monitor (`@wailsio/runtime` `Screens.GetAll()` as a picklist) and a theme, then started/stopped as a projector window |
| `Visual.svelte` | Projector styling tab (verse/couplet background, text, border, font) |
| `Audio.svelte` | "Звук" tab (Фонограммы feature): sub-view switch between media library and playlists, device select in the header, `MiniPlayer` transport pinned at the bottom, own keyboard map |

## For AI Agents

### Working In This Directory
- Each tab installs its own `document.addEventListener("keydown")` in `$effect` with cleanup. When you add new tabs, follow the same pattern — don't put global keyboard handlers in `+page.svelte`.
- Keyboard map: `Bible.svelte` — `Enter` shows verse, `Esc` hides, `↑/↓` next/prev verse, `←/→` prev/next chapter. `Songs.svelte` — `Enter` show couplet, `Esc` hide, `↑/↓` cycle couplets. `Audio.svelte` — `↑/↓` move the keyboard selection inside the active playlist (only while the "Плейлисты" sub-view is open), `Enter` plays the selected item, `Esc` stops with fade (`FadeOutStop`), `Space` pauses/resumes the loaded track (`preventDefault`, otherwise it also scrolls the list), `←/→` seek ∓5s, `Equal`/`NumpadAdd` and `Minus`/`NumpadSubtract` step volume by 5%. All by `e.code`, not `e.key` — Cyrillic layout makes `e.key` return the layout's own glyph (same lesson as `keyboard.ts`).
- **`Audio.svelte`'s mini-player volume/progress sliders are `<input type="range">`, which `keyboard.ts`'s `isTypingTarget` treats as a typing target** (it literally is an `HTMLInputElement`) — leaving focus on a dragged slider would silently swallow `Esc`/`Space`/arrow keys for the rest of the tab. Fixed locally: both sliders call `blur()` on their `change` handler right after acting (see `MiniPlayer.svelte`), not by narrowing the shared guard — narrowing it would break Bible/Songs search-field detection instead.
- The default tab is index `1` (Songs) because operators land there most often.
- `Bible.svelte` injects synthetic divider rows (ID = `-1`) into the books list to render section headings — `ListItem.svelte` checks `item.ID === -1` to switch styling.

### Common Patterns
- Components consume stores via `$derived(<storeName>.somePart)` and call `.active = item` setters that internally trigger fetches.
- Backend calls (e.g. `ShowVerse`, `HideCouplet`) are imported directly from generated bindings — there's no service-layer abstraction.
- `Songs.svelte` owns a local `songToDelete` `$state` for the delete confirmation modal — clicking the red trash button in the song list opens it; confirming calls `RemoveSong(id)`. The active-song fallback (previous → next → null) is computed **before** awaiting the backend so the store's `songs.list` setter preserves the choice when the `songs_update` event fires.
- `Songs.svelte` also owns a local `isEditSongTextOpen` `$state` driving `EditSongTextModal`. The button sits in the bottom action bar (after Показать/Скрыть + QR), disabled when there is no active song. The modal wipes-and-recreates couplets via `ReplaceCouplets` — the active couplet will reset to the first new one because all IDs change.

## Dependencies

### Internal
- `$lib/stores/BibleStore.svelte` (Bible tab), `$lib/stores/songsStore.svelte` (Songs tab), `$lib/stores/outputStore.svelte` (Screens tab), `$lib/stores/audioStore.svelte` (Audio tab).
- `$lib/components/*` for List/Select/CoupletsList/MiniPlayer/PlaylistPanel/etc.

<!-- MANUAL: -->
