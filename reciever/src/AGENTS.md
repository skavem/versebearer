<!-- Parent: ../AGENTS.md -->
<!-- Generated: 2026-05-22 | Updated: 2026-05-22 -->

# src

## Purpose
Reciever Svelte source. Mounts a single `App` that opens an SSE connection and renders verse + couplet components based on incoming events.

## Key Files
| File | Description |
|------|-------------|
| `main.ts` | Mounts `App` onto `#app` via Svelte 5 `mount()` |
| `App.svelte` | Holds `verse`/`couplet`/`qr` `$state`. Opens `new EventSource("/sse?stream=main")`, switches on `data.type`, renders `<ShownVerse>` + `<ShownCouplet>` |
| `types.ts` | `IShownVerse` (text, number, Book.shortName, Chapter.number) and `IShownCouplet` (text). Local-only — does not import from Go bindings |
| `vite-env.d.ts` | Vite client types |

## Subdirectories
| Directory | Purpose |
|-----------|---------|
| `lib/` | Components: `ShownVerse.svelte`, `ShownCouplet.svelte`. See `lib/components/AGENTS.md` |

## For AI Agents

### Working In This Directory
- `types.ts` is intentionally a hand-written subset — it mirrors only the fields the reciever actually reads from the Go payload. The Go side sends the full `ShownVerse`/`ShownCouplet` (with `gorm.Model` fields, `Translation`, etc.), but the reciever ignores everything except what's typed here.
- SSE message types: `show_verse`, `hide_verse`, `show_couplet`, `hide_couplet`, `show_qr`, `hide_qr`, `sync`. The `sync` event fires on every new client connect (server pushes current state); handle it the same as the individual show events.
- `App.svelte` has no reconnect logic — if the SSE connection drops, the projector page stays stale until reload. The Wails parent app stays alive long enough that this is usually fine.

### Common Patterns
- `$effect` opens the EventSource on mount; there is no cleanup. The reciever runs in a window owned by the OS — when the window closes the JS context dies.
- `console.log(data)` in the message handler is intentional for debugging — keep it unless you wire a real logger.

## Dependencies

### Internal
- `./lib/components/*`, `./types`.

<!-- MANUAL: -->
