# VerseBearer

A [Wails 3](https://v3alpha.wails.io/) desktop app for projecting **Bible verses** and **Christian song couplets** onto external screens during church services.

An operator drives the main window (SvelteKit UI) to pick verses, songs, and couplets. Selections are pushed over Server-Sent Events to a separate projector page that renders fullscreen text on one or more external monitors.

## How it works

```
┌─────────────────────┐        SSE (:9093)        ┌──────────────────────┐
│  Operator window     │  ── show_verse ─────────▶ │  Projector window(s)  │
│  (SvelteKit UI)      │  ── show_couplet ───────▶ │  (reciever Svelte app)│
│  Bible · Songs ·     │  ── show_qr ────────────▶ │  fullscreen output    │
│  Visual tabs         │  ── style / sync ───────▶ │  on external monitors │
└─────────────────────┘                           └──────────────────────┘
        │                                                    ▲
        └── SQLite (GORM): translations, books, ─────────────┘
            chapters, verses, songs, couplets
```

- The operator UI and the projector app are two separate frontends embedded into a single Go binary.
- The projector output is served on `http://localhost:9093` and broadcast to via SSE, so it stays in sync with the operator's picks and visual styling in real time.
- Content is stored in a local SQLite database, seeded from `Bible.json` (Synodal translation) and `songs.json`.

## Features

- **Bible & Songs tabs** — browse translations/books/chapters/verses and songs/couplets, and push any selection to the projector.
- **Multi-monitor projection** — open a projector window on any detected screen; the active operator screen is tracked so you don't project onto yourself.
- **Transparent overlay mode** — open a projector as a see-through, click-through window so verse/couplet text floats over whatever else is on that monitor.
- **`Ctrl+Shift+W` hotkey** — toggle projection on the secondary monitor (when exactly two screens are present), bypassing the confirm modal.
- **Visual tab** — customize verse and couplet styling independently (background colour/opacity, text colour, custom font upload, border, padding, margin, text-shadow). Changes are persisted and broadcast to the projector live.
- **QR display** — toggle a QR code on the projector output.
- **Whole-song bulk edit** — replace all couplets of a song at once via a text modal.

## Tech stack

| Layer | Technology |
|-------|-----------|
| Desktop shell | Wails 3 (alpha) |
| Backend | Go 1.25, GORM + SQLite (`mattn/go-sqlite3`, CGO) |
| Projector transport | `r3labs/sse` (SSE server on `:9093`) |
| Operator UI | SvelteKit |
| Projector UI | Svelte |

## Getting started

### Prerequisites

- **Go ≥ 1.25**
- **A C compiler** (e.g. `gcc` / MinGW-w64 on Windows) — the SQLite driver uses CGO, so builds require `CGO_ENABLED=1`.
- **[Wails 3 CLI](https://v3alpha.wails.io/)** — `wails3`
- **[Task](https://taskfile.dev/)** — `task` (build orchestration)
- **Node.js** — for the `frontend/` and `reciever/` builds

### Development

Run with hot-reload for both frontend and backend:

```
task dev
```

(equivalent to `wails3 dev`)

### Build

Produce a production executable in the `build/` directory:

```
wails3 build
```

### Test

```
task test          # Go smoke tests (in-memory SQLite)
cd frontend && npm run check   # operator UI type-check
cd reciever && npm run check   # projector UI type-check
```

## Project structure

| Path | Purpose |
|------|---------|
| `main.go` | Wails 3 entrypoint — wires the `DbHandler` service, starts the SSE server, opens the main window |
| `dbHandler.go` | Wails-exposed service: CRUD + show/hide for verses, couplets, QR, and screens |
| `sse.go` | SSE server on `:9093` — serves the embedded projector app and broadcasts events |
| `backend/` | GORM models, DB init, JSON seeder |
| `frontend/` | Operator SvelteKit UI (embedded into the binary) |
| `reciever/` | Projector output Svelte app (embedded, served on `:9093`) |
| `build/` | Per-OS build pipeline: Taskfiles, NSIS installer, Linux packaging, icons |
| `Bible.json` | Seed Synodal translation |
| `songs.json` | Seed song dump (local, git-ignored) |

> The module path is `changeme`; renaming it requires updating every `changeme/...` import.

More detail for each area lives in the per-directory `AGENTS.md` files.
