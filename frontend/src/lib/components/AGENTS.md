<!-- Parent: ../AGENTS.md -->
<!-- Generated: 2026-05-22 | Updated: 2026-05-22 -->

# components

## Purpose
Reusable Svelte 5 components used by the three tab views. All in Svelte 5 runes mode.

## Key Files
| File | Description |
|------|-------------|
| `List.svelte` | Virtualized list (generic over `{ID: number}`). Computes visible window via 44px row height + scrollTop. Auto-scrolls to active item |
| `ListItem.svelte` | Generic row used by `List.svelte`. Handles divider items (`ID === -1`) with separator styling |
| `VerseList.svelte` | Non-virtualized list of verses (chapter is short enough). Uses `{#each (verse.ID)}` keying |
| `VerseItem.svelte` | Single verse row with number badge and `visibility` icon when shown |
| `CoupletsList.svelte` | Couplets for the active song + side toolbar (move up/down via paired `UpdateCouplet` calls, edit, delete, add) |
| `CoupletItem.svelte` | Row used inside `CoupletsList.svelte` — supports multiline text |
| `Select.svelte` | DaisyUI dropdown + text filter, max 20 results, dynamic max-height |
| `SongsSelect.svelte` | Thin wrapper over `Select.svelte` for songs (display `"<number> - <title>"`, search by both) |
| `CreateSongModal.svelte` | Modal: number (auto-incremented from last) + title → `CreateSong` |
| `CreateEditCoupletModal.svelte` | Modal for both create and edit. Quick-fill buttons "Куплет"/"Припев"/"Бридж" for the label |
| `ScreenToggler.svelte` | Button per OS display. Toggles a `ShowScreen`/`CloseScreen` pair; uses `screen <ID>` as window name |
| `MuiIcon.svelte` | Renders a `<span class="material-icons">` with a typed `name` prop |
| `MuiIcon.ts` | `const iconNames = [...] as const; export type MuiIconNames = (typeof iconNames)[number]` — huge string literal union of valid icon names |

## For AI Agents

### Working In This Directory
- All components use Svelte 5 runes (`$props`, `$state`, `$derived`, `$bindable`, `$effect`). No `export let`, no `$:` reactive labels.
- Generic components use `<script lang="ts" generics="T extends {ID: number}">`.
- `List.svelte` virtualization assumes fixed 44px row height — if you change padding/border, update both the `44` in `shown` derivation and the absolute-positioned `top` in `ListItem.svelte`.
- Reordering couplets in `CoupletsList.svelte` is done by **two `UpdateCouplet` calls** that swap the `number` fields — there is no dedicated reorder endpoint. Keep that pattern when wiring new reorder UI.
- Modal pattern: parent component owns `isModalOpen = $state(false)` and binds it via `bind:isModalOpen` to the modal child. The child closes by setting `false`.
- `MuiIcon.ts` is enormous (2k+ entries). When adding a new icon usage, the name must already exist in that list; otherwise the type-check fails. Don't extend the list ad-hoc.

### Common Patterns
- Buttons use `btn btn-neutral btn-sm` (DaisyUI). Outline variants for toggles.
- Hover-only action buttons inside list rows use `hidden group-hover/item:block` Tailwind pattern combined with `class="group/item"` on the row container.
- `class={[..., cond && "x"]}` array syntax everywhere for conditional classes.

## Dependencies

### Internal
- `$lib/bindings/changeme/dbhandler` for `Show*`/`Hide*`/`Create*`/`Update*`/`Remove*` calls.
- `$lib/stores/*` for reactive state.

### External
- `@wailsio/runtime` `Screens` namespace (ScreenToggler).
- Svelte transitions (`fade`/`fly`).

<!-- MANUAL: -->
