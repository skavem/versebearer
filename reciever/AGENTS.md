<!-- Parent: ../AGENTS.md -->
<!-- Generated: 2026-05-22 | Updated: 2026-05-22 -->

# reciever

## Purpose
Projector-output Svelte app (note the typo in the directory name — keep it). Compiles to `dist/`, which the Go binary embeds (`//go:embed reciever/dist` in `SSE.go`) and serves from `http://localhost:9093`. The operator's `DbHandler.ShowScreen` opens borderless always-on-top webview windows pointing at this URL on each projector display. Listens to `/sse?stream=main` for `show_verse`/`show_couplet`/`show_qr`/`sync` events and renders fullscreen scaled text + optional QR.

## Key Files
| File | Description |
|------|-------------|
| `package.json` | Plain Vite + Svelte 5 (not SvelteKit). Includes `@castlenine/svelte-qrcode` |
| `vite.config.ts` | Default Vite + `@sveltejs/vite-plugin-svelte` |
| `svelte.config.js` | Default Svelte config |
| `tsconfig.json`, `tsconfig.app.json`, `tsconfig.node.json` | TS configs (app vs build) |
| `index.html` | Static shell, loads two `centurygothic*.ttf` files from `/`, mounts `<div id="app">` |
| `bun.lockb` | Bun lockfile — this side uses bun (or any tool that respects it), not npm |
| `.gitignore`, `README.md` | Vite defaults |

## Subdirectories
| Directory | Purpose |
|-----------|---------|
| `src/` | Source: `App.svelte` + verse/couplet components + types. See `src/AGENTS.md` |
| `public/` | Static fonts (`centurygothic.ttf`, `centurygothic_bold.ttf`) served at `/` |
| `dist/` | Build output. Embedded into Go binary — do not edit |

## For AI Agents

### Working In This Directory
- This is a **separate Vite project** from `frontend/`. Don't mix dependencies. It does NOT use SvelteKit, Tailwind, or DaisyUI — plain Svelte + scoped CSS only.
- The Go side serves `dist/` from disk when `DEV=true` in `.env` (`SSE.go` uses `http.Dir("./reciever/dist")`), otherwise from the embedded FS. So in dev: run `bun run build` (or `npm run build`) in `reciever/` after edits, OR set up your own watcher. There is no live Vite dev server wired through Wails for the reciever.
- SSE endpoint: `GET /sse?stream=main` (handled by `r3labs/sse` in `SSE.go`). Events arrive as JSON with `type` field — see `App.svelte`. On connect the Go side also sends a `sync` event with the current `verse`, `couplet`, `qr` state.
- QR URL is read from `import.meta.env.VITE_QR_URL` — set it via env (`.env` in `reciever/`) at build time.
- Font: "Century Gothic" referenced in CSS, loaded via `<link href="/centurygothic.ttf">` in `index.html`. The Wails projector windows MUST point at `http://localhost:9093` (the Go server) — opening the file directly bypasses font loading.

### Testing Requirements
- Type check: `bun run check` (or `npm run check`).
- Manual: build it (`bun run build`), then run the parent Wails app and open a projector window from the Screens tab.

### Common Patterns
- Dynamic font scaling: `ShownCouplet` grows then shrinks the font in a binary-ish loop until `clientHeight` fits. `ShownVerse` does the same with `em` units. Don't try to replace this with CSS `clamp()` — the content is variable-length and the layout depends on it fitting exactly.

## Dependencies

### External
- `svelte` 5, `vite` 6, `@castlenine/svelte-qrcode`.

<!-- MANUAL: -->
