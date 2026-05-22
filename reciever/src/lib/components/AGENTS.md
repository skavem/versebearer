<!-- Parent: ../../AGENTS.md -->
<!-- Generated: 2026-05-22 | Updated: 2026-05-22 -->

# components

## Purpose
Two fullscreen presentation components for the projector output: Bible verse (bottom half) and song couplet (full screen, with optional QR overlay).

## Key Files
| File | Description |
|------|-------------|
| `ShownVerse.svelte` | Bottom-half overlay. Black rounded card with verse text + reference (`<Book.shortName> <Chapter.number>:<verse.number>`). Auto-scales font (em units, 4 → 8) to fit |
| `ShownCouplet.svelte` | Full-screen black overlay. Auto-scales font in pixel steps (50 → 120, then shrinks until fits). Renders QR code (data from `VITE_QR_URL`) below the text when `qr` prop is true |

## For AI Agents

### Working In This Directory
- Auto-scale loops measure overflow imperatively via `bind:this` refs + `getBoundingClientRect()`/`clientHeight`/`scrollHeight`. `$effect` re-runs whenever the input prop changes.
- `ShownCouplet` uses pixel `font-size` + matching `line-height` so per-line spacing scales together. `ShownVerse` uses `em` on the container so children inherit.
- `getElHeight` (in ShownCouplet) subtracts padding + border because `clientHeight` includes padding — needed to compare against child sizes.
- `{#key couplet.text}` / `{#key verse}` forces a fresh mount with `fly` transition every time text changes. Don't remove — without it the transition only plays on first show.
- QR rendering uses `@castlenine/svelte-qrcode` with `data={import.meta.env.VITE_QR_URL}`. Set this at build time; runtime changes require a rebuild.
- CSS is scoped (single `<style>` block per component). Both rely on the "Century Gothic" font being loaded by `index.html`'s `<link rel="stylesheet">` tags.

## Dependencies

### Internal
- `../../types` for `IShownVerse`/`IShownCouplet`.

### External
- `@castlenine/svelte-qrcode` (ShownCouplet only).
- Svelte transitions (`fade`, `fly`).

<!-- MANUAL: -->
