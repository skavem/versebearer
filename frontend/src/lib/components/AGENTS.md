<!-- Parent: ../AGENTS.md -->
<!-- Generated: 2026-05-22 | Updated: 2026-05-23b -->

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
| `CreateSongModal.svelte` | Modal: number (auto-incremented from last) + title → `CreateSong`. Sets the returned song as active so the list scrolls to it |
| `CreateEditCoupletModal.svelte` | Modal for both create and edit. Quick-fill buttons "Куплет"/"Припев"/"Бридж" for the label |
| `EditSongTextModal.svelte` | Whole-song bulk-edit modal: textarea showing all couplets serialized via `$lib/songText#serializeSongText` (label-then-text, blocks separated by blank line). Save parses the text via `parseSongText` and calls `ReplaceCouplets(songId, blocks)` which atomically wipes and recreates the song's couplets |
| `OutputCard.svelte` | Card per persisted `Output` entity. Inline-editable name, monitor/theme `PopoverSelect` dropdowns, transparent toggle (disabled while live), Start/StopOutput button, inline delete confirm |
| `TranslationsSettings.svelte` | Bible translation manager inside `SettingsModal`: installed list (short label, книги/стихи, "сейчас на экране" flag), native-dialog file pick → preview → editable name/short label → import with a per-book progress bar, and delete-with-confirm. Owns no store — reads `ListTranslationSummaries` directly and lets the `translations_update` event refresh `BibleStore` |
| `MuiIcon.svelte` | Renders a `<span class="material-icons">` with a typed `name` prop |
| `MuiIcon.ts` | `const iconNames = [...] as const; export type MuiIconNames = (typeof iconNames)[number]` — huge string literal union of valid icon names |
| `AudioDeviceSelect.svelte` | Output-device picker for the "Звук" tab header. Same popup-list shape as `PopoverSelect.svelte` but with its own option markup — it needs to flag a `Missing` device (selected in `GlobalState` but absent from the live `ListDevices()` result) with distinct styling, which a plain hint string can't carry |
| `EditTrackModal.svelte` | Modal for one `AudioTrack`'s metadata: title/artist, trim start/end (typed as `м:сс`, parsed both ways), gain (dB range slider). "Взять текущую позицию" buttons are only enabled while this exact track is the one loaded in the player, and compute the absolute source-file position as `track.trimStartMs + PlayerState.positionMs` — `positionMs` is already relative to `TrimStartMs` (see `backend/AGENTS.md`'s Audio subsystem note), so adding the raw value again would double-count the offset |
| `PlaylistPanel.svelte` | Two-column playlist editor: playlist list + create/rename/delete on the left, active playlist's items (add, native HTML5 drag-reorder, remove) on the right. Reordering computes the target array order client-side and sends it whole via `ReorderPlaylist`, unlike `CoupletsList.svelte`'s pairwise-swap reorder. Takes a bindable `selectedItemId` prop so `Audio.svelte`'s keyboard handler can drive a row highlight independent from "currently playing" (blue = keyboard selection, amber = on air — two different facts about a row, never the same color) |
| `MiniPlayer.svelte` | Transport bar for the "Звук" tab: prev/play-pause/next/stop, a seekable progress range with a "-remaining" label (highlighted red in the last 10s), a volume range, and a peak-level meter. Never picks a track itself — starting one stays with the media library / playlist rows (И5, "exactly one active track" — track selection lives in exactly one place). The level meter's peak-hold is a local `requestAnimationFrame` exponential decay layered over the polled `PlayerState.peak`, not a second backend channel |

## For AI Agents

### Working In This Directory
- All components use Svelte 5 runes (`$props`, `$state`, `$derived`, `$bindable`, `$effect`). No `export let`, no `$:` reactive labels.
- Generic components use `<script lang="ts" generics="T extends {ID: number}">`.
- `List.svelte` virtualization assumes fixed 44px row height — if you change padding/border, update both the `44` in `shown` derivation and the absolute-positioned `top` in `ListItem.svelte`.
- `List.svelte` re-runs its auto-scroll effect on **both** `activeItem` change AND `items.length` change. The latter is what makes the list scroll to the (unchanged) active row after a sibling is deleted, or to a freshly created row inserted into a long list. If you add a new "trigger a re-scroll" condition, extend that effect — don't add a parallel one.
- Reordering couplets in `CoupletsList.svelte` is done by **two `UpdateCouplet` calls** that swap the `number` fields — there is no dedicated reorder endpoint. Keep that pattern when wiring new reorder UI.
- Modal pattern: parent component owns `isModalOpen = $state(false)` and binds it via `bind:isModalOpen` to the modal child. The child closes by setting `false`.
- `MuiIcon.ts` is enormous (2k+ entries). When adding a new icon usage, the name must already exist in that list; otherwise the type-check fails. Don't extend the list ad-hoc.
- **Any bare `<input type="range">` living directly on a tab body (not inside a `.modal`) must call `.blur()` on its `change` handler.** `$lib/keyboard.ts`'s `isTypingTarget` guard treats any `HTMLInputElement` as a typing target, range inputs included — a slider left focused after a drag would silently eat that tab's `Esc`/`Space`/arrow-key shortcuts. `MiniPlayer.svelte`'s progress and volume sliders do this locally; `EditTrackModal.svelte`'s gain slider does not need it because everything inside `.modal` is already excluded by the `isFromModal` guard the tab-level handlers check first.

### Common Patterns
- Buttons use `btn btn-neutral btn-sm` (DaisyUI). Outline variants for toggles.
- Hover-only action buttons inside list rows use `hidden group-hover/item:block` Tailwind pattern combined with `class="group/item"` on the row container.
- `class={[..., cond && "x"]}` array syntax everywhere for conditional classes.

## Dependencies

### Internal
- `$lib/bindings/changeme/dbhandler` for `Show*`/`Hide*`/`Create*`/`Update*`/`Remove*` calls.
- `$lib/stores/*` for reactive state.

### External
- `@wailsio/runtime` `Screens` namespace (via `outputStore`, consumed by `OutputCard`/`ScreenMiniMap`).
- Svelte transitions (`fade`/`fly`).

<!-- MANUAL: -->
