# Backend Direction: Obsidian / Bear as a Storage Backend (via their official CLIs)

**Status:** Investigation + plan (no code changes yet)
**Branch:** `claude/obsidian-bear-backend-yhh1ei`
**Scope:** Use Obsidian or Bear as a storage "backend" for ruin — driven through each app's
**official first-party CLI** — so those apps provide polish/rendering while ruin (plus a Mac
modal quick-capture) handles capture and generated/dynamic pages. Plan the five requested
features against that model.

> Primary integration surface = the two official CLIs:
> - **Obsidian CLI** — `obsidian …`, bundled in Obsidian **1.12+** (early access / Catalyst,
>   Feb 2026). **Drives the running app** (focused vault); notes are plain `.md` on disk.
>   Docs: <https://obsidian.md/cli>.
> - **BearCLI** — `bearcli …` (`/Applications/Bear.app/Contents/MacOS/bearcli`), bundled in
>   **Bear 2.8** (Apr 2026). **Operates directly on the local Bear database in place** (no
>   x-callback, no API token, no window flashing); also exposes `bearcli mcp-server`. macOS
>   only. Docs: <https://bear.app/faq/command-line-interface/>. *(This corrects earlier
>   analysis based on the old `bear://x-callback-url` scheme — the official `bearcli` is a
>   native DB tool and is far more capable and headless-friendly.)*

---

## 1. The vision

Today ruin *owns* a folder of Markdown files and treats its `.ruin/` indices as the hot-path
source of truth. The proposal inverts the relationship: let a first-class app (Obsidian or
Bear) own presentation and interaction, and reposition ruin as the **write/derivation
engine** that:

- captures input (CLI, and a Mac modal quick-capture),
- **bakes** structure in at write time (a parent set appends/prepends the child's text into
  the parent's section, rather than storing a pointer),
- **materializes** dynamic pages (`pick` / embeds become real notes that regenerate when
  their sources change),
- keeps plaintext tag/link files as **helpers**, not sources of truth (querying the backend
  CLI live where possible),
- **two-way syncs** `pick`/checkbox done-state between the materialized view and origin,
- and runs a **monitor** mode that watches the backend and refreshes generated artifacts.

## 2. What the two official CLIs actually give us

Both CLIs speak **structured output** (JSON/CSV/TSV) and cover create/read/search/append plus
tag management. ruin's job becomes "**shell out to `obsidian` / `bearcli`, parse the
structured output, and route writes through targeted subcommands.**" But the two differ in
important ways — and, correcting the earlier draft, **Bear is the more surgical and more
headless of the two CLIs**, while Obsidian brings frontmatter, semantic tasks, and `eval`.

| Capability ruin needs | **Obsidian CLI** (`obsidian`) | **BearCLI** (`bearcli`) |
|---|---|---|
| Execution model | **Drives the running app** (focused vault) | **Direct on the local DB in place** — no token, no window flash |
| Requires the app running | **Yes** | Not stated in help (works on the DB); concurrent-edit safety via `--base` |
| Platform | Where Obsidian runs (mac/win/linux) | **macOS only** |
| Structured output | `format=json` (also csv/plain) | `--format json\|csv\|tsv` (reads); **mutations are silent, exit-code only** |
| Create / read / list / search | ✅ | ✅ `create`, `cat`, `show`, `list`, `search`, `search-in` |
| Rich query language | `search` | ✅ `search` (`@todo @done @task @backlinks @wikilinks @tagged @date(...)` …) |
| **Append / prepend** | ✅ (whole file / position) | ✅ `append --position beginning\|end` |
| **Insert at a specific heading / line** | ❌ native (needs `eval` or read-modify-write) | ✅ **`edit --find "## H" --insert-after/-before`** (also `--replace`, `--all`, `--word`) |
| **Whole-note replace w/ concurrency guard** | (via `eval`/RMW) | ✅ **`overwrite --base <hash>`** (optimistic lock) + `--no-update-modified` |
| Tags: list + counts + manage | ✅ `tags counts` | ✅ `tags list/add/remove/rename/delete` (`--count`) |
| **Frontmatter / properties** | ✅ `property` get/set | ❌ Bear has **no frontmatter model** |
| **Tasks: list + completion** | ✅ `tasks` (semantic completion) | ⚠️ no task verb, but `show --fields todos,done` + `edit --find/--replace` toggles a box; `search @todo/@done/@task` |
| **Backlinks** | ✅ `backlinks file=…` | ✅ `search "@backlinks"` / `@wikilinks` |
| Change-detection primitive | file mtime (disk) | ✅ per-note **`hash`** + `modified` via `show`/`list` |
| **Arbitrary API escape hatch** | ✅ `eval "<JS>"` (full plugin API) | ✅ `bearcli mcp-server` (structured MCP interface) |
| Note identity | file / title / path (no UUID) | note **ID** or `--title` (case-insensitive) |
| Change events / push | ❌ (watch files) | ❌ (poll `hash`/`modified`) |
| Encrypted notes | n/a | inaccessible via the CLI |

**Secondary surface (Obsidian only):** Obsidian notes are plain `.md` on disk, so ruin can
read them directly and use `fsnotify` for monitor mode **when the app isn't running** — the
headless fallback for the Obsidian backend. Bear has no plain files; `bearcli` itself *is* the
headless surface for Bear.

### 2.1 The operational constraints, corrected

- **Obsidian CLI requires the running app.** Its headless fallback is direct file access
  (read `.md`, `fsnotify`). So for Obsidian, "app is closed" ⇒ ruin drops to the FS path.
- **BearCLI works on the database in place** — no token, no x-callback, no GUI flash. Whether
  the Bear app must be *concurrently* running isn't documented; the `overwrite --base <hash>`
  optimistic-lock and `--no-update-modified` exist precisely to make writes safe when Bear or
  another client is also editing. So for Bear, `bearcli` is both the interactive and the
  headless path.
- **Writes go through the tools, not around them.** ruin stops clobbering files/DB behind the
  app's back, which removes most cross-process races — and Bear even gives a first-class
  concurrency guard (`--base`).

### Recommendation

Build one **BackendCLI adapter** abstraction with three impls — **FS** (default, headless,
CI), **Obsidian** (`obsidian`), **Bear** (`bearcli`). Neither app CLI is strictly weaker;
they're differently shaped:

- **Obsidian** fits ruin's *data model* better (real **frontmatter** via `property`, which is
  where ruin keeps uuid/parent/tags/dates; semantic **tasks**; `eval` for anything missing;
  cross-platform; invisible `%%comments%%` for generated regions).
- **Bear** fits ruin's *write operations* better (surgical **`edit`** for heading/line-precise
  inserts and checkbox toggles; **`overwrite --base`** concurrency; per-note **`hash`** for
  change detection; no app-running requirement) — but it's macOS-only and has **no
  frontmatter**, so ruin's uuid/parent/inherited-tags model must be side-banded or expressed
  as Bear tags/body.

Ship both behind the same seam; let capability probing pick native paths vs fallbacks.

---

## 3. Architectural seams to introduce

### 3.1 `backend` config selector
A new field on the four-field `Config` struct (`internal/config/config.go`), following the
existing `*bool`-with-default + `RUIN_*` env + auto-surfaced-by-`config` pattern:
`backend: ruin | obsidian | bear` (env `RUIN_BACKEND`), plus optional `backend_cli_path`.
The value is a **profile** that also sets defaults for existing flags (e.g. both app backends
⇒ `tag_frontmatter:false` on Bear it's moot; disable `#spaced tag#`).

### 3.2 `BackendCLI` — the adapter seam (core of the design)
Implementations shell out to `obsidian` / `bearcli` (and the FS code) and parse structured
output. Methods, with the native mapping noted:

```
Create(note)                       // obsidian create ; bearcli create (returns id/hash)
Read(id) / List() / Search(q)      // obsidian read/search ; bearcli cat/show/list/search (JSON)
Append(id, text, position)         // obsidian append ; bearcli append --position
InsertAtHeading(id, heading, text, mode)
     // Bear:  bearcli edit --find "<heading>" --insert-after|--insert-before
     // Obs:   eval / read-modify-write (no native heading append yet)
ReplaceRegion(id, oldExact, new)   // Bear: bearcli edit --find --replace ; Obs: eval / RMW
Overwrite(id, content, baseHash?)  // Bear: bearcli overwrite --base ; Obs: create/replace
QueryTags() / QueryBacklinks(id)   // obsidian tags counts/backlinks ; bearcli tags list / search "@backlinks"
GetProperty/SetProperty(id,k,v)    // Obsidian only (property); Bear: emulate side-band
ListTasks(scope) / CompleteTask(r) // Obs: tasks ; Bear: show --fields todos,done + edit --replace
Hash(id) / Modified(id)            // Bear: show --fields hash,modified ; Obs: file hash/mtime
Eval(js)                           // Obsidian escape hatch
Available() / Capabilities()       // probe + advertise which of the above are native
```

Adapters **advertise capabilities**; ruin selects native vs fallback (heading-insert native
on Bear, `eval`/RMW on Obsidian; concurrency guard native on Bear, best-effort on Obsidian).

### 3.3 `MetadataProvider` — demote the indices, query live
Route the read paths that currently *trust* `.ruin` indices —
`search_engine.go` `prefilterPathsViaTitles`/`hydrateNoteTagsFromIndex` (deep coupling: it
skips opening matching files and trusts `titles.json`), `tags list` (reads only `tags.yml`),
and `RefreshLinkedCards`/`ResolveNote` — through `QueryTags/QueryBacklinks/ResolveTitle`.
Under the CLI backends this is answered **live**: `obsidian tags counts format=json` /
`obsidian backlinks`; `bearcli tags list --format json` / `bearcli search "@backlinks" --format json`.
`.ruin` indices survive only as an **offline cache** (Obsidian: built by disk scan when the
app is down; Bear: `bearcli` is always available, so the cache is largely optional).
`inherited-tags` stays ruin-computed — no backend supplies it.

### 3.4 `SourceWatcher` — the monitor seam
Neither CLI emits events, so watch the **store**: `FileWatcher` (fsnotify;
`fsnotify/fsevents` recursive on macOS) over Obsidian's `.md` files; for Bear, a
`PollingWatcher` that diffs per-note **`hash`/`modified`** via `bearcli list --fields id,modified,hash`
(cleaner and more official than scraping SQLite mtime). Both emit a backend-agnostic
`ChangeSet` into a shared regen pipeline that writes back through the CLI.

### 3.5 Persisted dependency graph + generated-region markers
`.ruin/deps.json`: `embed-id → {host note, directive, source ids, output hash}` + the reverse
index — the attribution data ruin already computes and discards
(`compose_dynamic.go:652 attributionEntry`, `compose_walker.go:389 sourceEntry`). Generated
content is delimited by fenced markers so ruin replaces only its own region:
- **Obsidian:** `%% ruin:embed id=… %% end` — comment syntax, invisible in preview.
- **Bear:** no invisible-comment syntax; use a distinct heading/marker line (e.g.
  `## ⚙︎ ruin:embed`) and replace the region via `edit`/`overwrite --base`. *(Open question:
  the least-intrusive Bear-visible marker.)*

---

## 4. Feature-by-feature plan

### 4.1 Parent baking (append/prepend child into the parent's section at write time)
**Today:** `n.Parent = parentNote.UUID` writes one scalar; the parent file is never touched.

**Plan — `BackendCLI.InsertAtHeading(parentID, heading, text, mode)`:**
- **Bear (native, clean):** `bearcli edit <parent> --find "## <heading>" --insert-after "\n<child body>"`
  (or `--insert-before`). This is exactly heading-targeted baking, first-class. Idempotency:
  include a fenced marker in the inserted text and re-bake via `edit --find "<marker-region>"
  --replace …` or `overwrite --base`.
- **Obsidian:** `append` has no heading target yet, so use `eval` (JS: locate heading, insert
  via `vault.process`/editor) or read-modify-write (`read format=json` → splice → write back).
  Switch to a native `heading=` flag if/when the CLI adds it (open feature request).
- **FS:** direct splice.

Wrap the block in a `<!-- ruin:baked <childUUID> -->…` fence (Obsidian) / heading marker
(Bear) for idempotent re-bake.

**Recommendation:** keep baking **opt-in**, not the default — the pointer model is strictly
more capable/reversible, and cycle detection + the inherited-tags cascade key off the UUID
pointer. Retain the child as provenance (`embedded-in: <parentUUID>` on Obsidian/FS; a
side-band map entry on Bear) rather than deleting. Surface as
`ruin note set <child> --parent <ref> --under "<heading>" --mode append|prepend` (+ on `log`).

**Breaking / risk:** parent-set now writes the *parent* note; Obsidian requires the app
running (Bear does not); `--no-parent` can't cleanly un-bake; cycle detection, the `--force`
guard, and the inherited-tags cascade must be redesigned or gated to the pointer mode.

### 4.2 Dynamic embeds → materialized notes
**Today:** embeds expand to stdout on demand and are discarded; the marker stays verbatim.

**Plan:** ruin evaluates the embed as now, then **writes the result through the backend CLI**
— a dedicated generated note (`create`) or a fenced region inside a host note. Regenerating a
region:
- **Bear:** `overwrite --base <hash>` (read `show --fields hash,content` → splice region →
  overwrite with the hash guard) or `edit --find <region> --replace`. `--no-update-modified`
  keeps the refresh from bumping `modified` and retriggering the monitor.
- **Obsidian:** `eval`/RMW to replace between `%% %%` sentinels.

Invalidate via `.ruin/deps.json`; regenerate on source change via the monitor with
**hash-guarded, idempotent writes** (Bear's `--base` + `hash` field make this first-class).
Exclude generated notes/regions from `pickCandidatePaths`/search/tag-counting. Add a **clock
tick** for time-dependent embeds (`@today`, dated `query:`).

**Breaking / risk:** embeds now create/modify notes; introduces the authored-vs-generated
boundary; `pick`/search must exclude generated content; new `.ruin/deps.json`; generated
writes must not bump `updated`/`modified` (use `--no-update-modified`) or re-run cascades.

### 4.3 Plaintext tags/links as helpers — query the CLI live
**Fully supported by both CLIs.** `obsidian tags counts format=json` / `obsidian backlinks`;
`bearcli tags list --format json` / `bearcli search "@backlinks|@wikilinks" --format json`.
Route the index-trusting read paths through `MetadataProvider` (§3.3). `.ruin/tags.yml`/
`titles.json` become a rebuildable offline cache (or, for Bear where `bearcli` is always
available, largely optional). `inherited-tags` stays ruin-computed from the `parent:` chain;
on Bear (no frontmatter) it's applied as real Bear tags via `tags add` and/or tracked
side-band.

**Breaking / risk:** search results and `tags list` counts change (the CLIs count differently
than ruin's per-note-deduped body count); `linked-cards` frontmatter may be retired where the
backend supplies links live; JSON-output consumers may see different numbers.

### 4.4 `pick` two-way done-sync
Meaningful once pick output is **materialized** (§4.2). Both CLIs can now do it:
- **Obsidian:** the `tasks` command has completion states — list and mark complete via the
  CLI. Identity is the crux (current `pick` uses positional line index, `pick.go:565`); anchor
  tasks with block-refs `^ruin-<id>` (or whatever `tasks` references) so a toggle routes to
  the right origin. `eval` is the fallback.
- **Bear:** `show --fields todos,done` reads state; toggle a specific box with
  `bearcli edit <id> --find "- [ ] <task text>" --replace "- [x] <task text>"`. Real per-line
  two-way sync — **no whole-note rewrite needed** (correcting the earlier draft). Identity is
  content-based (exact task text; ambiguous on duplicate lines — Bear has no block-ref anchor).
- Unify ruin's two done encodings (`#done` tag vs `[x]` checkbox).

**Breaking / risk:** `pick` becomes a *writer* and (on Obsidian) may inject anchors into source
bodies; `--toggle-todo` becomes cross-note; Bear line identity is content-based (fragile on
duplicates).

### 4.5 Monitor mode
**Plan:** `ruin monitor` watches the store (neither CLI emits events):
- **Obsidian:** `fsnotify` on the vault `.md` files → filter, ignore `.obsidian/` →
  **hybrid debounce** (~200ms quiet / ~500ms ceiling) → regenerate, writing back through the
  `obsidian` CLI when the app is up (or directly to disk when it's closed).
- **Bear:** poll `bearcli list --fields id,modified,hash` and diff against last-seen hashes
  (official, no SQLite scraping) → regenerate via `bearcli edit`/`overwrite --base`. Can run
  even if the app is closed. Use `--no-update-modified` so ruin's own writes don't bump
  `modified` and self-trigger.

**Loop safety is mandatory:** self-written id/path + hash suppression set, and — the strongest
guard — **content-hash idempotency** (Bear's per-note `hash` + `--base` make this exact). Plus
the clock tick for time-based embeds.

**Breaking / risk:** ruin gains a long-lived daemon (it has been one-shot only) → lifecycle,
locking vs foreground commands, macOS TCC; `versioning:true` would flood commits from regen
writes unless suppressed/batched.

---

## 5. Cross-cutting concerns

- **Data-model fit differs by backend.** Obsidian has **frontmatter** (`property` get/set) —
  a near-native home for ruin's `uuid`/`parent`/`tags`/`dates`. **Bear has no frontmatter**,
  so ruin's metadata must be side-banded (a `.ruin` map keyed by Bear note **ID**) or expressed
  as Bear **tags** (via `tags add`) and body markers. This is the single biggest Bear-specific
  design task.
- **Identity.** Obsidian: title/file/path (no UUID) — keep ruin's `google/uuid` with a
  translation table. Bear: stable note **ID** — map `uuid ↔ Bear id` side-band.
- **Concurrency & loop safety.** Bear gives first-class primitives: `overwrite --base <hash>`
  (optimistic lock), per-note `hash`, `--no-update-modified`. Obsidian has no documented
  concurrency guard — rely on idempotent hash-compare writes and self-write suppression.
- **App-running.** Obsidian CLI needs the app up (FS fallback when down); Bear's `bearcli`
  does not appear to. Adapters must probe and degrade.
- **`inherited-tags`.** No backend supplies it; stays ruin-computed. On Bear, materialize it
  as real tags; retain a shadow pointer if baking removes the `parent:` scalar.
- **Time-based staleness.** `@today`/dated `query:` embeds need a scheduled re-eval; a pure
  file/DB watch can't cover clock changes.
- **Git versioning noise.** Daemon regen writes must be suppressed/batched under
  `versioning:true` (only meaningful for the FS/Obsidian file backends).
- **Escape hatches.** Obsidian `eval` (JS) and `bearcli mcp-server` (MCP) each cover gaps —
  e.g. Obsidian heading-insert via `eval`; structured write-results on Bear via MCP (CLI
  mutations are silent).

---

## 6. Phased roadmap (incremental, ship value early)

1. **Backend seam, no behavior change.** Add the `backend` config field + profile defaults;
   extract `BackendCLI` / `MetadataProvider` interfaces with FS as the sole impl. Pure refactor.
2. **Read/metadata profiles (CLI).** Implement `obsidian` and `bearcli` read adapters
   (`search`, `read`/`cat`/`show`, `tags`, `backlinks`) parsing JSON; route `MetadataProvider`
   through them; demote `.ruin` indices to caches; add availability probing + FS fallback.
   Delivers live tag/link/backlink queries immediately.
3. **Backend writes: create + append.** `obsidian create/append`, `bearcli create/append`.
   Wire `ruin log` through `BackendCLI.Create` under a CLI backend.
4. **`InsertAtHeading` + opt-in parent baking.** Native on Bear (`edit --insert-after`),
   `eval`/RMW on Obsidian; fenced idempotent bake; provenance retained. Pointer stays default.
5. **Materialized embeds + `deps.json`.** Region markers, dependency graph, generated-content
   exclusion; region replace via `overwrite --base` (Bear) / `eval` (Obsidian). Explicit
   `ruin embed regen` first.
6. **`ruin monitor`.** fsnotify (Obsidian files) / `bearcli` hash-poll (Bear) + debounce +
   **loop guards** + clock tick; wires steps 2–5 into real-time refresh.
7. **Two-way done-sync.** Obsidian via `tasks` + `^ruin-id`; Bear via `edit --find/--replace`
   on checkbox lines + `show --fields todos,done`.
8. **Hardening.** Bear frontmatter-emulation/side-band map; concurrency edge cases; encrypted-
   note handling; document per-backend limits.

Steps 1–3 are low-risk and independently valuable. Steps 4–7 are the substance; each app CLI
has native support for most of it (Bear for surgical writes, Obsidian for frontmatter/tasks/
`eval`), so this is far less "degraded on Bear" than the earlier draft assumed.

---

## 7. Decisions needed before building

1. **Ship both, or Obsidian-first / Bear-first?** They're differently strong: Obsidian =
   cross-platform + frontmatter + tasks + `eval`; Bear = surgical edits + concurrency guard +
   no-app-required, but macOS-only + no frontmatter. Recommend building the seam once and
   enabling both, leading with whichever you use daily.
2. **Bear metadata model:** side-band `.ruin` map keyed by Bear ID (recommended) vs encoding
   ruin metadata as Bear tags/body markers?
3. **App-down behavior (Obsidian):** fall back to disk reads + queue writes, hard-error, or
   transparently use the FS backend? (Bear's `bearcli` sidesteps this.)
4. **Bake vs pointer:** opt-in coexistence (recommended) or replace the pointer model?
5. **Baked child fate:** delete (loses UUID identity) or keep as `embedded-in:`/side-band
   provenance stub (recommended)?
6. **Task/line identity:** Obsidian `^ruin-id` block-refs vs the `tasks` command's own
   references; Bear is content-hash only (no anchors) — accept duplicate-line ambiguity?
7. **Done-state canonical form:** `#done` tag vs `[x]` checkbox — which is authoritative, and
   does flipping one write the other?

---

## 8. Biggest risks

- **Obsidian CLI requires the running app and is early-access (1.12+, Catalyst)** — "commands
  and syntax are likely to change." Pin a version, probe capabilities at runtime, keep the
  `eval` and FS fallbacks.
- **Bear has no frontmatter and is macOS-only** — ruin's uuid/parent/inherited-tags model must
  be re-homed side-band or as tags, and there's no cross-platform/CI path on Bear. Bear line
  identity is content-based (no block refs), so duplicate task lines are ambiguous.
- **Watcher feedback loops** (daemon writes retriggering regen) — mitigated by idempotent
  hash-guarded writes + self-write suppression; Bear's `hash`/`--base`/`--no-update-modified`
  make this tractable, Obsidian relies on hash-compare.
- **Behavior/compat drift:** demoting the indices and flipping `tag_frontmatter` under the CLI
  profiles changes search results, `tags list` counts, and which frontmatter keys ruin writes
  — real breaking changes for downstream tooling (per the "avoid breaking changes" rule in
  `CLAUDE.md`).
- **Two CLIs, two shapes:** the abstraction must not paper over real divergences (Obsidian
  mutations return data / Bear mutations are silent exit-code-only; Bear `overwrite` needs
  `--base` via MCP but not via CLI; heading-insert native on Bear, `eval` on Obsidian).

---

*Investigation conducted via a 13-agent workflow (5 codebase readers, 3 research agents, 5
per-feature design agents), then revised against the two official CLIs — including the full
`bearcli help all` reference, which corrected the Bear analysis from the obsolete
`bear://x-callback-url` model to the native `bearcli` DB tool. Code references are current as
of this branch. CLI details should still be re-verified against `obsidian help` /
`bearcli help all` at implementation time, as both CLIs are new (2026) and evolving.*
