# Backend Direction: Obsidian / Bear as a Storage Backend (via their official CLIs)

**Status:** Investigation + plan (no code changes yet)
**Branch:** `claude/obsidian-bear-backend-yhh1ei`
**Scope:** Use Obsidian or Bear as a storage "backend" for ruin — driven through each app's
**official first-party CLI** — so those apps provide polish/rendering while ruin (plus a Mac
modal quick-capture) handles capture and generated/dynamic pages. Plan the five requested
features against that model.

> This revision assumes the two official CLIs as the primary integration surface:
> - **Obsidian CLI** — `obsidian …`, bundled in Obsidian **1.12+** (early access / Catalyst,
>   Feb 2026). Docs: <https://obsidian.md/cli>.
> - **BearCLI** — `bearcli …` (`/Applications/Bear.app/Contents/MacOS/bearcli`), bundled in
>   **Bear 2.8** (Apr 2026). Docs: <https://bear.app/faq/command-line-interface/>.

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

Both CLIs are **app-mediated** (they drive the running app, they are not headless file
tools), both speak **JSON**, and both cover create/read/append/search/tags. That symmetry is
the big change from earlier analysis: ruin's job becomes "**shell out to the backend CLI and
parse `--json`**," with a small set of capability gaps to paper over.

| Capability ruin needs | **Obsidian CLI** (`obsidian`) | **BearCLI** (`bearcli`) |
|---|---|---|
| Distribution | Bundled, Obsidian 1.12+ (early access) | Bundled, Bear 2.8 |
| **Requires the app running** | **Yes** (drives the focused vault) | **Yes** (bundled in the app) |
| Platform | Where Obsidian runs (mac/win/linux) — app must be up | **macOS only** |
| Structured output | `format=json` (also csv/plain) | `--json` on every command |
| create / read / append / search | ✅ | ✅ (`create`, `read`, `search`, `add-text`) |
| Tags (with counts) | ✅ `tags counts` | ✅ `tags` |
| **Backlinks** | ✅ `backlinks file=…` | ⚠️ via search/tags; DB has `ZSFNOTEBACKLINK` |
| **Frontmatter properties** | ✅ `property` get/set | ❌ Bear has no frontmatter model |
| **Tasks: list + completion** | ✅ `tasks …` (completion states) | ❌ no checkbox/task command |
| **Append under a specific heading** | ❌ not native yet (whole file/section) — *open feature request*; doable via `eval` | ❌ whole-note only (`--mode append\|prepend\|replace\|replace_all`) |
| **Arbitrary API escape hatch** | ✅ `eval "…JS…"` (full plugin API: editor, vault.process, metadataCache) | ❌ (but Bear 2.8 also ships an MCP server + Claude connector) |
| Note identity | file / title / path (no UUID) | `--id` / `--title` |
| Change events / push | ❌ (watch files / poll) | ❌ (poll) |

**Complementary non-CLI surfaces (secondary, for when the app is closed):**
- **Obsidian:** notes are plain `.md` on disk → ruin can still read directly and use
  `fsnotify` for monitor mode when the app isn't running.
- **Bear:** read-only **SQLite** (`database.sqlite`; `ZSFNOTE`, `ZSFNOTETAG`,
  `ZSFNOTEBACKLINK`) → headless reads + mtime polling for monitor mode.

### 2.1 The one constraint that drives everything

**Both CLIs require their app to be running.** Earlier analysis treated Obsidian as headless
(direct files) and Bear as the only app-bound backend; assuming the CLIs makes **both**
app-bound. Consequences:

- ruin's **headless/CI/scripted** story can no longer rely on the CLI. It must either (a) use
  the FS backend (ruin's own vault), (b) fall back to the secondary read surfaces
  (Obsidian files / Bear SQLite) for **reads**, and (c) **queue or fail clearly** on writes
  when the app is down.
- Every backend adapter must **probe availability** ("is `obsidian`/`bearcli` present and the
  app up?") and degrade predictably.
- On the upside, because writes go *through the app*, ruin stops clobbering files behind the
  app's back — the app mediates, which **reduces the cross-process write races** that a
  direct-file approach would create (especially for Obsidian).

### Recommendation

Build one **BackendCLI adapter** abstraction, ship **Obsidian first-class** (richer CLI:
properties, tasks-with-completion, backlinks, `eval`), and **Bear as an opt-in, macOS-only
profile** whose CLI is capable for capture/read but weak for the line-level, task-sync, and
heading-precise operations several features need. Keep the existing **FS backend** as the
default and the headless/CI path. The rest of this plan is written CLI-first, Obsidian
leading, with Bear and the secondary surfaces called out per feature.

---

## 3. Architectural seams to introduce

Five seams carry the whole plan; each is small and independently shippable.

### 3.1 `backend` config selector
A new field on the four-field `Config` struct (`internal/config/config.go`), following the
existing `*bool`-with-default + `RUIN_*` env + auto-surfaced-by-`config` pattern:
`backend: ruin | obsidian | bear` (env `RUIN_BACKEND`), plus optional `backend_cli_path`
and `backend_vault` (which Obsidian vault / Bear is implicit). The value is a **profile** that
also sets sensible defaults for existing flags (e.g. `obsidian`/`bear` ⇒ `tag_frontmatter:false`,
disable the `#spaced tag#` format).

### 3.2 `BackendCLI` — the adapter seam (the core of this revision)
An interface whose implementations **shell out to `obsidian` / `bearcli` and parse JSON**,
replacing the two hard-wired layers today (`note.Save`/`os.WriteFile` and
`vault.CreateNote/SaveNote/DeleteNote`). Methods map onto the CLIs:

```
Create(note) / Read(id) / List() / Search(query)     // both CLIs, JSON
Append(id, text, mode)                                // obsidian append ; bearcli add-text --mode
AppendToSection(id, heading, text, mode)              // see 4.1 — emulated where not native
QueryTags() / QueryBacklinks(id)                      // obsidian tags counts / backlinks ; bearcli tags
GetProperty/SetProperty(id, key, val)                 // obsidian property (Obsidian only)
ListTasks(scope) / CompleteTask(ref)                  // obsidian tasks (Obsidian only)
Eval(js)                                              // obsidian eval (Obsidian escape hatch)
Available() (bool, reason)                            // probe: CLI present + app running
Capabilities() CapSet                                 // advertise which of the above exist
```

Adapters **advertise capabilities** so ruin can pick a native path or a fallback (e.g.
heading-append native vs read-modify-write; backlinks native vs computed by scan). Three
impls: **FS** (today's code, headless), **Obsidian** (`obsidian` CLI), **Bear** (`bearcli`).

### 3.3 `MetadataProvider` — demote the indices, query live
Routes the read paths that currently *trust* `.ruin` indices —
`search_engine.go` `prefilterPathsViaTitles`/`hydrateNoteTagsFromIndex` (the deep coupling:
it skips opening matching files and trusts `titles.json`), `tags list` (reads only
`tags.yml`), and `RefreshLinkedCards`/`ResolveNote` — through `QueryTags/QueryBacklinks/ResolveTitle`.
Under the CLI backends this is answered **live** (`obsidian tags counts format=json`,
`obsidian backlinks`, `bearcli tags --json`), directly satisfying "keep the plaintext files
as helpers, and read live from the app where possible." `.ruin` indices survive only as an
**offline cache** (populated by disk scan / SQLite when the app is down). `inherited-tags`
stays ruin-computed under every backend — no backend supplies it.

### 3.4 `SourceWatcher` — the monitor seam
Neither CLI emits events, so watch the **store**, not the CLI: `FileWatcher` (fsnotify;
`fsnotify/fsevents` recursive on macOS) over Obsidian's `.md` files; `PollingWatcher` (SQLite
mtime + `ZMODIFICATIONDATE` cursor) for Bear. Both emit a backend-agnostic `ChangeSet` into a
shared regen pipeline that then writes updates back **through the CLI**.

### 3.5 Persisted dependency graph + generated-region markers
`.ruin/deps.json`: `embed-id → {host note, directive, source ids, output hash}` + the reverse
`source → dependent embeds` index. This is **exactly the attribution data ruin already
computes and throws away** (`compose_dynamic.go:652 attributionEntry{UUID,LineOffset}`,
`compose_walker.go:389 sourceEntry`). Generated content is delimited by fenced markers (for
Obsidian, comment syntax `%% ruin:embed id=… %%`, invisible in preview) so ruin replaces only
its own region — the **authored-vs-generated boundary** the on-demand model never needed.

---

## 4. Feature-by-feature plan

### 4.1 Parent baking (append/prepend child into the parent's section at write time)
**Today:** `n.Parent = parentNote.UUID` writes one scalar; the parent file is *never* touched;
the tree is materialized lazily by `compose`/`tree`/`children` via `ChildrenMap` inversion of
`titles.json`.

**Plan — `BackendCLI.AppendToSection(parentID, heading, text, mode)`:**
- **Obsidian:** native `append`/`prepend` is whole-file today (heading targeting is an open
  feature request), so implement section-precision via **`eval`** — run JS that finds the
  heading in the parent and inserts under it through `vault.process`/editor API — or via
  **read-modify-write** (`obsidian read format=json` → splice under the heading → write back).
  When the CLI gains a native `heading=` flag, switch to it behind the capability probe.
- **Bear:** `bearcli add-text --title <parent> --mode append|prepend` is whole-note; for
  heading precision, **read-modify-write** (`bearcli read --json` → splice → `add-text --mode
  replace_all`). All GUI-mediated; Bear must be running.
- **FS:** direct splice (as in the previous plan).

Wrap the inserted block in a `<!-- ruin:baked <childUUID> -->…<!-- /ruin:baked -->` fence so
re-bake replaces rather than duplicates.

**Recommendation:** keep baking **opt-in**, not the default — the pointer model is strictly
more capable/reversible, and cycle detection + the inherited-tags cascade key off the UUID
pointer. Retain the child as provenance (`embedded-in: <parentUUID>`) instead of deleting.
Surface as `ruin note set <child> --parent <ref> --under "<heading>" --mode append|prepend`
(+ on `log`).

**Breaking / risk:** parent-set now writes the *parent* note (never happened before); on
Obsidian/Bear it depends on the app running; `--no-parent` can't cleanly un-bake; cycle
detection, the `--force` overwrite guard, and the inherited-tags cascade must be redesigned or
gated to the pointer mode.

### 4.2 Dynamic embeds → materialized notes
**Today:** `![[pick:]]`/`![[search:]]`/`![[query:]]`/`![[compose:]]` expand to stdout on
demand and are discarded; the literal marker stays verbatim.

**Plan:** ruin evaluates the embed as it does now, then **writes the result through the
backend CLI** — either as a dedicated generated note (`obsidian create` / `bearcli create`)
or as a fenced region inside a host note (`AppendToSection` / `eval` / read-modify-write).
Invalidate via `.ruin/deps.json`; regenerate on source change via the monitor with
**hash-guarded, idempotent writes**. Exclude generated notes/regions from
`pickCandidatePaths`/search/tag-counting so ruin never re-ingests its own output. Add a
**clock tick** for time-dependent embeds (`@today`, dated `query:`) — no file/DB event fires
at midnight.

**Bear:** `create`/`add-text` can write the generated note, but every write is a GUI-mediated,
whole-note operation; regeneration flashes a window and needs Bear running. Recommend Bear be
**evaluate-and-write-once**, not a background-refreshed target.

**Breaking / risk:** embeds now create/modify notes; introduces the authored-vs-generated
boundary; `pick`/search must exclude generated content; new `.ruin/deps.json`; generated
writes must **not** bump `updated` or re-run cascades.

### 4.3 Plaintext tags/links as helpers — query the CLI live
**This is now directly supported.** Both CLIs return the data live as JSON:
`obsidian tags counts format=json`, `obsidian backlinks file=…`, `bearcli tags --json`,
`bearcli search --json`. Route the index-trusting read paths through `MetadataProvider`
(§3.3); under a CLI backend, answer from the CLI. `.ruin/tags.yml`/`titles.json` remain a
**rebuildable offline cache** (built by disk scan / SQLite when the app is down), never the
sole authority. Obsidian exposes real backlinks via `backlinks`; Bear via `ZSFNOTEBACKLINK`
(the CLI itself has no backlinks command).

**Breaking / risk:** search results and `tags list` counts change (the CLIs count differently
than ruin's per-note-deduped body count); `linked-cards` frontmatter may be retired where the
backend supplies links live; JSON-output consumers may see different numbers.

### 4.4 `pick` two-way done-sync
Only meaningful once pick output is **materialized** (§4.2). Needs stable identity, the
persisted attribution index, and a loop-safe writer/trigger.

**Obsidian (strong):** the CLI has a **`tasks` command with completion states**, so ruin can
list tasks and mark one complete/incomplete via the CLI rather than rewriting files. Identity
is the crux — the current `pick` keys off *positional* line index (`pick.go:565`), which
breaks on reorder; anchor tasks with block-refs `^ruin-<id>` (or whatever stable reference
`tasks` accepts) so a toggle routes to the right origin. `eval` is the fallback for anything
`tasks` can't express. Unify ruin's two done encodings (`#done` tag vs `[x]` checkbox).

**Bear (weak):** no task/checkbox command — a toggle is a whole-note `add-text --mode
replace_all` round-trip with content-hash identity (no block refs). **Opt-in, best-effort,
off by default.**

**Breaking / risk:** `pick` becomes a *writer* and (on Obsidian) mutates source bodies to add
anchors; `--toggle-todo` becomes cross-note; behavior depends on the app running.

### 4.5 Monitor mode
**Plan:** `ruin monitor` watches the **store** (neither CLI emits events):
- **Obsidian:** `fsnotify` on the vault `.md` files → filter, ignore `.obsidian/` →
  **hybrid debounce** (~200ms quiet / ~500ms ceiling) → regenerate derived artifacts and
  materialized embeds, writing back **through the `obsidian` CLI** (or directly to disk when
  the app is closed). This works even when Obsidian isn't running (files on disk), and uses
  the CLI to write when it is.
- **Bear:** poll `database.sqlite` mtime + `ZMODIFICATIONDATE`; regenerate ruin-owned side
  artifacts. **One-way only** — do not background-write into Bear (GUI flashes, whole-note
  round-trips, racy).

**Loop safety is mandatory and built in from day one:** self-written path/id + hash
suppression set, a mute window, and — the strongest guard — **content-hash idempotency** (skip
byte-identical writes). Plus the clock tick for time-based embeds.

**Breaking / risk:** ruin gains a long-lived daemon (it has been one-shot only) → lifecycle,
locking vs foreground commands, macOS TCC/Automation prompts; `versioning:true` would flood
commits from regen writes unless suppressed/batched.

---

## 5. Cross-cutting concerns

- **App-must-be-running (both backends).** The defining operational constraint now. Every
  adapter probes availability and degrades: FS backend for headless/CI; secondary read
  surfaces (Obsidian files / Bear SQLite) when the app is down; clear errors (or a write
  queue) on writes while the app is closed.
- **Identity.** Obsidian CLI addresses notes by title/file/path (no UUID); Bear by `--id`/
  `--title`. Keep ruin's `google/uuid` as the internal key with a per-backend translation
  table (`uuid ↔ file/title` or `uuid ↔ Bear id`).
- **Loop safety.** #1 failure mode for §4.2/§4.4/§4.5 — idempotent hash-guarded writes +
  self-write suppression are non-negotiable.
- **Time-based staleness.** `@today`/dated `query:` embeds need a scheduled re-eval; a pure
  file/DB watch can't cover clock changes.
- **`inherited-tags`.** No backend/CLI supplies it; stays ruin-computed from the `parent:`
  chain. If baking removes the pointer (§4.1), retain a shadow pointer to keep the cascade.
- **Git versioning noise.** Daemon regen writes must be suppressed/batched under
  `versioning:true`.
- **`eval` is the Obsidian escape hatch.** Wherever the CLI lacks a native verb (heading
  append, block-ref writes, richer queries), `obsidian eval "<JS>"` reaches the full plugin
  API. Bear has no equivalent (its extensibility is the separate MCP server / Claude
  connector, out of scope here).

---

## 6. Phased roadmap (incremental, ship value early)

1. **Backend seam, no behavior change.** Add the `backend` config field + profile defaults;
   extract `BackendCLI` / `MetadataProvider` interfaces with the current FS code as the sole
   implementation. Pure refactor; everything still passes.
2. **Obsidian read/metadata profile (CLI).** Implement the `obsidian`-CLI adapter for reads:
   `search`, `read`, `tags counts`, `backlinks`, `property` — parse `format=json`. Route
   `MetadataProvider` through it; demote `.ruin` indices to caches; add availability probing
   + FS fallback. Immediately delivers live tag/link/backlink queries.
3. **Backend writes (CLI): create + append.** `obsidian create/append`, `bearcli
   create/add-text`. Wire `ruin log` through `BackendCLI.Create` under a CLI backend.
4. **`AppendToSection` + opt-in parent baking.** Native where available, else `eval` /
   read-modify-write; fenced idempotent bake; provenance retained. Pointer stays the default.
5. **Materialized embeds + `deps.json`.** Region markers, dependency graph, generated-content
   exclusion. Refreshed by explicit `ruin embed regen` first.
6. **`ruin monitor`.** fsnotify (Obsidian files) / SQLite poll (Bear) + debounce + **loop
   guards** + clock tick; wires steps 2–5 into real-time refresh.
7. **Two-way done-sync.** Obsidian via the `tasks` command + `^ruin-id` anchors; Bear opt-in
   whole-note round-trip, off by default.
8. **Bear profile hardening (macOS).** `bearcli` adapter + SQLite read fallback + poll watcher;
   document the no-background-writes / no-auto-sync limits.

Steps 1–3 are low-risk and independently valuable (live queries + CLI capture). Steps 4–7 are
the substance; Obsidian leads because its CLI is richer. Bear rides the same seam but is
capped by its CLI (no tasks, no frontmatter, whole-note writes, macOS-only).

---

## 7. Decisions needed before building

1. **Obsidian-first, or full Bear parity?** Bear's CLI lacks tasks/frontmatter/heading-precise
   writes and is macOS-only; several features (two-way sync, baking precision) are degraded on
   it. Recommend Obsidian-first, Bear as an opt-in capture/read profile.
2. **App-down behavior:** for a CLI backend with the app closed, do we (a) fall back to reads
   via disk/SQLite and **queue writes**, (b) hard-error, or (c) transparently fall back to the
   FS backend? (Recommend a + clear errors on write.)
3. **Bake vs pointer:** opt-in coexistence (recommended) or replace the pointer model?
4. **Baked child fate:** delete (destructive merge, loses UUID identity) or keep as
   `embedded-in:` provenance stub (recommended)?
5. **Task identity on Obsidian:** rely on the `tasks` command's own referencing, or inject
   `^ruin-id` block-refs into source bodies for stable round-tripping?
6. **Done-state canonical form:** `#done` tag vs `[x]` checkbox — which is authoritative, and
   does flipping one write the other?

---

## 8. Biggest risks

- **Both CLIs require the running app** — the headless/CI story now rests entirely on the FS
  backend and the secondary read surfaces; a naive CLI-only design breaks scripting/automation.
- **Obsidian CLI is early-access (Catalyst, 1.12+)** — "commands and syntax are likely to
  change." Pin to a known version, probe capabilities at runtime, and keep the `eval` fallback.
- **Watcher feedback loops** (daemon writes retriggering regen) — mitigated only by idempotent
  hash-guarded writes + self-write suppression; build in from day one.
- **Bear is macOS-only with a coarse CLI** (whole-note writes, no tasks/frontmatter) — feature
  parity with Obsidian is not achievable; anything implying it will disappoint.
- **Behavior/compat drift:** demoting the indices and flipping `tag_frontmatter` under the
  CLI profiles changes search results, `tags list` counts, and which frontmatter keys ruin
  writes — real breaking changes for downstream tooling (per the "avoid breaking changes" rule
  in `CLAUDE.md`).

---

*Investigation conducted via a 13-agent workflow (5 codebase readers, 3 backend/tooling
research agents, 5 per-feature design agents), then revised against the two official CLIs
(<https://obsidian.md/cli>, <https://bear.app/faq/command-line-interface/>). Code references
are current as of this branch. CLI capability details (esp. heading-targeted append, task
identity, exact JSON flags) should be re-verified against `obsidian help` / `bearcli help all`
at implementation time, as both CLIs are new (2026) and evolving.*
