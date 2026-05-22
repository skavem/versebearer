<!-- Parent: ../AGENTS.md -->
<!-- Generated: 2026-05-22 | Updated: 2026-05-22 -->

# routes

## Purpose
SvelteKit routes. The whole operator app lives at `/`, with three tab views imported as plain components (not nested SvelteKit routes).

## Key Files
| File | Description |
|------|-------------|
| `+layout.svelte` | Minimal — imports `app.css`, renders children |
| `+layout.ts` | `prerender = true`, `ssr = false` — static SPA |
| `+page.svelte` | Navbar + tab switcher. `activeTabIndex` default = 1 (Песни/Songs) |
| `Bible.svelte` | Bible navigation tab: translation/book/chapter selectors, verse list, recent-history sidebar |
| `Songs.svelte` | Song picker + couplet display tab + favorites list + QR toggle |
| `Screens.svelte` | Lists OS displays from `@wailsio/runtime` `Screens.GetAll()`; toggle projector windows |

## For AI Agents

### Working In This Directory
- Each tab installs its own `document.addEventListener("keydown")` in `$effect` with cleanup. When you add new tabs, follow the same pattern — don't put global keyboard handlers in `+page.svelte`.
- Keyboard map: `Bible.svelte` — `Enter` shows verse, `Esc` hides, `↑/↓` next/prev verse, `←/→` prev/next chapter. `Songs.svelte` — `Enter` show couplet, `Esc` hide, `↑/↓` cycle couplets.
- The default tab is index `1` (Songs) because operators land there most often.
- `Bible.svelte` injects synthetic divider rows (ID = `-1`) into the books list to render section headings — `ListItem.svelte` checks `item.ID === -1` to switch styling.

### Common Patterns
- Components consume stores via `$derived(<storeName>.somePart)` and call `.active = item` setters that internally trigger fetches.
- Backend calls (e.g. `ShowVerse`, `HideCouplet`) are imported directly from generated bindings — there's no service-layer abstraction.

## Dependencies

### Internal
- `$lib/stores/BibleStore.svelte` (Bible tab), `$lib/stores/songsStore.svelte` (Songs tab), `$lib/stores/screenStore.svelte` (Screens tab).
- `$lib/components/*` for List/Select/CoupletsList/etc.

<!-- MANUAL: -->
