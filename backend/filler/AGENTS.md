<!-- Parent: ../AGENTS.md -->
<!-- Generated: 2026-05-22 | Updated: 2026-05-22 -->

# filler

## Purpose
Standalone seeder binary (own `package main`). Run once on a fresh DB to import the Synodal Bible translation from `Bible.json` and the song book from `songs.json`. Also creates one default `Screen` row and a `GlobalState` row pointing at it.

## Key Files
| File | Description |
|------|-------------|
| `fillDb.go` | `func main()`. Decodes `Bible.json` → `Translation`/`Book`/`Chapter`/`Verse`. Decodes `songs.json` → `Song`/`Couplet`. Inserts seed `Screen` + `GlobalState` |

## For AI Agents

### Working In This Directory
- This is a **separate `package main`** from the root app. Run with `go run ./backend/filler` from the repo root. Both binaries import `backend/inits`, so they share the same `test.db` (CWD-relative). Run the filler from the same directory you'll run `wails3 dev` from, or you'll seed the wrong file.
- The seeder appends rather than upserts — running it twice creates duplicate "Синодальный" translation + duplicate books. Wipe `test.db` between runs.
- `Bible.json` structure: array of books with `name` (short), `fullName`, `content[][]string` (chapters → verses). `songs.json` structure: array of songs with numeric `label` (parsed via `strconv.Atoi`, skipped on parse failure) and `couplets[]{label,text,index}`.
- `songs.json` is gitignored (see root `.gitignore` / `git status`) — operator-specific song lists are not committed. Each install seeds from a locally provided file.
- `Bible.json` is committed and ships with the repo.

## Dependencies

### Internal
- `changeme/backend/inits` (DB), `changeme/backend/models` (schema).

### External
- stdlib only (`encoding/json`, `os`, `strconv`, `log`).

<!-- MANUAL: -->
