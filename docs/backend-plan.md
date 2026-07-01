# Backend Direction: Obsidian / Bear as a Storage Backend

**Status:** Investigation + plan (no code changes yet)
**Branch:** `claude/obsidian-bear-backend-yhh1ei`
**Scope:** Evaluate using Obsidian or Bear as a storage "backend" for ruin, so those
apps provide polish/rendering while ruin (plus Mac modal input) handles capture and
generated/dynamic pages. Plan the five requested features against that model.

---

## 1. The vision

Today ruin *owns* a folder of Markdown files and treats its `.ruin/` indices as the
hot-path source of truth. The proposal inverts the relationship: let a first-class app
(Obsidian or Bear) own presentation and interaction, and reposition ruin as the
**write/derivation engine** that:

- captures input (CLI, and a Mac modal quick-capture),
- **bakes** structure in at write time (a parent set appends/prepends the child's text
  into the parent's section, rather than storing a pointer),
- **materializes** dynamic pages (`pick` / embeds become real files that regenerate when
  their sources change),
- keeps plaintext tag/link files as **helpers**, not sources of truth (and reads live
  from the backend where possible),
- **two-way syncs** `pick`/checkbox done-state between the materialized view and origin,
- and runs a **monitor** mode that watches the backend and refreshes generated artifacts.

## 2. The single most important finding

**Obsidian and Bear are not symmetric options.** They sit at opposite ends of the
"how much does ruin own the store" axis, and that difference decides the whole design.

| Capability ruin needs | **Obsidian** | **Bear** |
|---|---|---|
| Note storage | Plain `.md` on disk (ruin already owns exactly this) | Opaque Core-Data **SQLite** DB (`database.sqlite`); no per-note files |
| Works **headless** (no app running) | ✅ read + write files directly | ⚠️ **read-only** via SQLite; **all writes need Bear running** |
| Cross-platform / CI | ✅ | ❌ **macOS/iOS only**, per-platform API token |
| Append/prepend **under a heading** | ✅ direct file splice (or REST `PATCH` when app up) | ✅ `x-callback add-text&header=&mode=` — section-granular, but GUI round-trip |
| Atomic **single-line** write (checkbox toggle) | ✅ file edit; ✅ REST `PATCH Target-Type:block` when app up | ❌ finest write is whole-note `replace_all` |
| **Stable line identity** | ✅ block refs `^id` | ❌ none — must hash line text |
| Live **tags** | scan files (or REST `GET /tags/` when app up) | ✅ `ZSFNOTETAG` + `Z_*TAGS` (read-only, works closed) |
| Live **backlinks** | ❌ not persisted — must scan the vault | ✅ `ZSFNOTEBACKLINK` (Bear actually stores them) |
| **Change detection** for monitor mode | ✅ `fsnotify` real-time | ⚠️ **poll** `database.sqlite` mtime + `ZMODIFICATIONDATE` cursor |
| Writes without disrupting the user | ✅ silent file writes | ❌ every write **launches/flashes** a Bear window |

**Consequence:** Obsidian is essentially *ruin's current filesystem model with the indices
demoted to caches* — every requested feature works headless, with no plugin and no running
app for the core path. Bear structurally **cannot** do headless materialization, atomic
line writes, real-time watching, or clean two-way sync; its only write path is a
GUI-mediated, whole-note/whole-section, racy round-trip.

### Recommendation

Build one backend abstraction, but ship **Obsidian as the first-class backend** and treat
**Bear as an opt-in, read-mostly, macOS-only degraded profile**:

- **Obsidian / ruin-FS:** full feature set — bake, materialize, two-way sync, real-time
  monitor — all headless on plain files. (The existing ruin vault *is* an Obsidian-shaped
  backend; Obsidian support and "ruin native" support are largely the same code path.)
- **Bear:** read everything from read-only SQLite (tags, backlinks, bodies, checkbox
  state, change-polling); write *only* through `x-callback add-text` for explicit,
  foreground, best-effort actions (create, append/prepend-under-header). **No background
  writes, no automatic two-way sync into Bear**, documented as macOS-only and racy.

The rest of this plan is written Obsidian-first; Bear notes are called out per feature.

---

## 3. Architectural seams to introduce

Five new seams carry the whole plan. Each is small and independently shippable.

### 3.1 `backend` config selector
A single new field on the four-field `Config` struct (`internal/config/config.go`),
following the existing `*bool`-with-default + `RUIN_*` env + auto-surfaced-by-`config`
pattern: `backend: ruin | obsidian | bear` (env `RUIN_BACKEND`). This value becomes a
**profile** that sets sensible defaults for the existing flags rather than toggling each
independently — e.g. `obsidian` implies `tag_frontmatter:false` and disables the
`#spaced tag#` format; `bear` additionally switches identity/read/write strategy.

### 3.2 `NoteStore` — the write/persistence seam
Abstracts the two hard-wired layers today (`note.Save`/`os.WriteFile` and
`vault.CreateNote/SaveNote/DeleteNote`). Methods:
`Create`, `Read`, `Update`, `List`, `Delete`, and crucially **`AppendToSection(id, heading, mode, text)`**
— which ruin lacks entirely today (all writes are full-file overwrites). This is the
primitive that "bake into parent section" and "materialize into a region" both need.

- **FS/Obsidian impl:** load → locate heading by normalized text → splice → hash-guarded
  full-file write. Reuses existing machinery.
- **Bear impl:** `Create`/`AppendToSection` → `x-callback add-text` (`header=`, `mode=`);
  `Read`/`List` → read-only SQLite; `Update` (region replace) → whole-note `replace_all`
  round-trip, flagged best-effort.

### 3.3 `MetadataProvider` — demote the indices
Routes the three read paths that currently *trust* `.ruin` indices through an interface:
`QueryTags()`, `QueryLinks(id)`, `QueryBacklinks(id)`, `ResolveTitle(title)`.
The deep coupling is `search_engine.go` `prefilterPathsViaTitles`/`hydrateNoteTagsFromIndex`
(it skips opening matching files and trusts `titles.json`); also `tags list` (reads only
`tags.yml`) and `RefreshLinkedCards`/`ResolveNote`.

- **FS/Obsidian:** derive live from bodies (backlinks always by scan — Obsidian persists
  none); `.ruin` indices remain a rebuildable **speed cache**, never the sole authority,
  so an Obsidian edit no longer yields stale search results. REST `GET /tags/` and
  `POST /search/` are an *optional accelerator* only when the app+plugin are present.
- **Bear:** answer tags from `ZSFNOTETAG`/`Z_*TAGS` and links **and backlinks** from
  `ZSFNOTEBACKLINK`, read-only.

`inherited-tags` stays ruin-computed under every backend — no backend supplies it; it is
derived from the frontmatter `parent:` chain (which Obsidian preserves).

### 3.4 `SourceWatcher` — the monitor seam
`FileWatcher` (fsnotify; `fsnotify/fsevents` for recursive on macOS) for Obsidian/ruin,
`PollingWatcher` (SQLite mtime high-water + `ZMODIFICATIONDATE` cursor) for Bear. Both
emit a backend-agnostic `ChangeSet` into one shared regen pipeline, hiding push-vs-poll.

### 3.5 Persisted dependency/attribution graph + generated-region markers
`.ruin/deps.json`: `embed-id → {host note, directive, source UUIDs, output hash}` plus the
reverse `source → dependent embeds` index. This is **exactly the attribution data ruin
already computes and throws away** (`compose_dynamic.go:652 attributionEntry{UUID,LineOffset}`,
`compose_walker.go:389 sourceEntry`). Generated content is spliced between fenced markers
(Obsidian comment syntax `%% ruin:embed id=… %% end`, invisible in preview) so ruin can
replace only its own region and never clobber authored text — the **authored-vs-generated
boundary** the on-demand model never needed.

---

## 4. Feature-by-feature plan

### 4.1 Parent baking (append/prepend child into parent's section at write time)
**Today:** `n.Parent = parentNote.UUID` writes one scalar; the parent file is *never*
touched; the tree is materialized lazily by `compose`/`tree`/`children` via
`ChildrenMap` inversion of `titles.json`.

**Plan:** `NoteStore.AppendToSection(parentID, heading, mode, text)`.
- Obsidian/FS: resolve parent → locate heading by normalized text → heading-shift the
  child body by parent depth (reuse `compose_walker` normalization) → insert
  (prepend = after heading; append = before next same/higher heading or EOF) → wrap in a
  `<!-- ruin:baked <childUUID> -->…<!-- /ruin:baked -->` fence for idempotent re-bake →
  hash-guarded save.
- Bear: `add-text&header=&mode=append|prepend` — clean for **first** insert; re-bake needs
  a whole-note `replace_all` (racy), so **forbid re-bake on Bear** rather than duplicate.

**Recommendation:** make baking **opt-in**, not the default — the pointer model is strictly
more capable and reversible. Keep the child note as provenance (a namespaced
`embedded-in: <parentUUID>` key) rather than deleting it, so `inherited-tags`, cycle
detection, and un-bake still have something to key on. Surface as
`ruin note set <child> --parent <ref> --under "<heading>" --mode append|prepend` (+ on `log`).

**Breaking / risk:** parent-set now writes the *parent* file (never happened before);
`--no-parent` can no longer cleanly remove a baked relationship; cycle detection, the
`--force` overwrite guard, and the inherited-tags cascade all key off the UUID pointer and
must be redesigned or gated to the pointer mode.

### 4.2 Dynamic embeds → materialized files
**Today:** `![[pick:]]`/`![[search:]]`/`![[query:]]`/`![[compose:]]` are expanded on
demand to stdout and discarded; the literal marker stays verbatim in the body.

**Plan (Obsidian/FS, primary):** keep the `![[pick: …]]` directive authoritative; on regen,
splice the expansion between `%% ruin:embed … %%` sentinels via the `NoteStore` region
writer; drive invalidation from `.ruin/deps.json`; regenerate on source change via the
monitor with **hash-guarded, idempotent writes**. Exclude generated regions from
`pickCandidatePaths`/search/tag-counting so ruin never re-ingests its own output.
Add a **clock tick** for time-dependent embeds (`@today`, dated `query:`) — file events
never fire at midnight.

**Bear:** read/evaluate from SQLite is fine headless; materialization is a whole-note
`replace_all` per regen (GUI flash, poll-only trigger, lost-update race). Ship Bear as
**read/evaluate-only**; do not auto-materialize into Bear.

**Breaking / risk:** embeds now modify files; introduces the authored-vs-generated
boundary; `pick`/search must exclude generated regions; a new `.ruin/deps.json`; generated
writes must **not** bump `updated` or re-run cascades.

### 4.3 Plaintext tags/links as helpers, not source of truth
**Plan:** route the index-trusting read paths through `MetadataProvider` (§3.3). FS keeps
writing `tags.yml`/`titles.json` as a rebuildable cache; Obsidian re-derives live from
bodies (backlinks by scan); Bear answers from SQLite. External tools that read `.ruin/*`
still work under the `ruin`/`obsidian` FS profile but must treat it as optional under Bear.

**Breaking / risk:** search results can shift when the live provider corrects a stale
index; `tags list` counts differ across providers (Bear counts DB associations, Obsidian
counts metadataCache, ruin counts per-note-deduped body occurrences); `linked-cards`
frontmatter may be retired where the backend supplies links live.

### 4.4 `pick` two-way done-sync
Only meaningful once pick output is **materialized** (§4.2) — today the pick line *is* the
source line. Needs (a) stable identity, (b) the persisted attribution index, (c) a
loop-safe writer/trigger.

**Plan (Obsidian):** on first pick, inject a block-ref `^ruin-<id>` on the **source** line
(edit/reorder-stable, unlike today's positional `pick.go:565` indexing); the materialized
line back-points via `[[Note#^ruin-id]]` or an invisible comment. Toggling either side
routes through a generalized `note set --toggle-todo` (single-note today) to flip the
origin, then re-derives the view. When Obsidian is running, REST `PATCH Target-Type:block`
flips one line atomically (no full-file race). Unify the two done encodings ruin currently
conflates (`#done` tag vs `[x]` checkbox); optionally honor the Tasks-plugin
`✅ YYYY-MM-DD`/recurrence lifecycle.

**Bear:** read side clean (SQLite); write side is a whole-note `replace_all` with
content-hash identity (no block refs) — **opt-in, best-effort, off by default**.

**Breaking / risk:** `pick` becomes a *writer* and mutates source bodies (injecting
`^ruin-id`); `--toggle-todo` becomes cross-note.

### 4.5 Monitor mode
**Plan (Obsidian/FS):** `ruin monitor` = `fsnotify` on the vault → filter `.md`, ignore
`.obsidian/` → **hybrid debounce** (~200ms quiet, ~500ms ceiling; per-path coalescing map
via `time.AfterFunc`) → incremental regen reusing existing
`RefreshTags`/`RefreshDates`/`RefreshLinkedCards`/`ComputeInheritedTags` + `vault.SaveNote`
(essentially an event-driven, file-scoped `doctor`) → materialized-embed refresh via
`deps.json`. **Loop safety is mandatory and built in from day one:** self-written
path+mtime suppression set, a mute window, and — the strongest guard — **content-hash
idempotency** (skip byte-identical writes). Plus the clock tick for time-based embeds.

**Bear:** poll-based, **one-way** (SQLite → regenerate ruin-owned side artifacts only).
Never background-write into Bear.

**Breaking / risk:** ruin gains a long-lived daemon (it has been one-shot only) → process
lifecycle, locking vs foreground commands, TCC/Automation prompts; git `versioning:true`
would flood commits from regen writes unless suppressed/batched; concurrent
ruin+app writers replace the current single-process load-mutate-save with a real race.

---

## 5. Cross-cutting concerns

- **Identity.** ruin keys on `google/uuid` in frontmatter. Obsidian has no UUID (map to
  vault-relative filename, or the Advanced-URI `uid=` frontmatter key); Bear uses
  `ZUNIQUEIDENTIFIER`. Decide: keep ruin UUID side-band, adopt backend id, or maintain a
  translation table (`uuid ↔ backend-id`). Recommended: keep ruin UUID as the internal key
  with a translation table per backend.
- **Loop safety.** The #1 failure mode for §4.2/§4.4/§4.5. Idempotent hash-guarded writes +
  self-write suppression are non-negotiable and must land with the first daemon code.
- **Time-based staleness.** `@today`/dated `query:` embeds need a scheduled re-eval; a pure
  file/DB watch is provably insufficient.
- **`inherited-tags`.** No backend supplies it; it stays ruin-computed from the `parent:`
  chain. If baking removes the pointer (§4.1), retain a shadow pointer (or `embedded-in:`)
  solely to keep the cascade working.
- **Git versioning noise.** Daemon regen writes must be suppressed, batched, or squashed
  under `versioning:true`.
- **Bear schema drift.** `Z_*TAGS` junction name varies by Bear version (`Z_5TAGS` vs
  `Z_7TAGS`); the reader needs version detection + fallback, and read-only WAL-aware
  connections.

---

## 6. Phased roadmap (incremental, ship value early)

1. **Backend seam, no behavior change.** Add the `backend` config field + profile defaults;
   extract `NoteStore` / `MetadataProvider` interfaces with the current FS code as the sole
   implementation. Pure refactor; everything still passes. *(Highlight: internal seam only.)*
2. **Obsidian read/metadata profile.** `MetadataProvider` re-derives live from bodies;
   demote `.ruin` indices to caches; `backend: obsidian` flips `tag_frontmatter`/tag-format
   defaults. Fixes stale-index-after-external-edit. *(Breaking: search/`tags list`
   semantics under the obsidian profile.)*
3. **`AppendToSection` + opt-in parent baking (FS/Obsidian).** New primitive; `--under`
   flag; fenced idempotent bake; provenance retained. Pointer model remains the default.
4. **Materialized embeds + `deps.json` (FS/Obsidian).** Region markers, dependency graph,
   generated-region exclusion. Still refreshed only by explicit `ruin embed regen` at first.
5. **`ruin monitor` (FS/Obsidian).** fsnotify + debounce + **loop guards** + clock tick;
   wires steps 2–4 into real-time refresh.
6. **Two-way done-sync (Obsidian).** `^ruin-id` anchors, cross-note toggle, done-encoding
   unification; optional REST `PATCH` fast-path when the app is running.
7. **Bear profile (macOS, opt-in, degraded).** Read-only SQLite provider + poll watcher +
   `x-callback` create/append; explicitly no background writes, no auto two-way sync.

Steps 1–2 are low-risk and independently valuable (Obsidian coexistence + no more stale
index). Steps 3–6 are the substance and only touch the FS/Obsidian path. Step 7 is
strictly additive and can be deferred or dropped without affecting 1–6.

---

## 7. Decisions needed before building

1. **Is Bear in scope for v1, or Obsidian-first?** Bear's macOS-only, app-must-be-running,
   GUI-flashing, racy write model caps it at read-mostly. Recommend Obsidian-first; Bear as
   a later opt-in profile (step 7) or explicitly out of scope.
2. **Bake vs pointer:** opt-in coexistence (recommended) or replace the pointer model?
   Replacing loses reversible reparenting, cycle detection, and the inherited-tags cascade.
3. **Baked child fate:** delete the child (destructive merge, loses UUID identity) or keep
   as an `embedded-in:` provenance stub (recommended)?
4. **Injecting `^ruin-id` block refs into user source bodies** for stable line identity —
   acceptable? It mutates authored content and is impossible on Bear.
5. **Daemon + git:** how to handle auto-versioning churn and single-writer locking between
   `ruin monitor` and foreground commands.
6. **Done-state canonical form:** `#done` tag vs `[x]` checkbox — which is authoritative,
   and does flipping one write the other?

---

## 8. Biggest risks

- **Watcher feedback loops** (daemon's own writes retriggering regen). Mitigated only by
  idempotent hash-guarded writes + self-write suppression — build in from day one.
- **Cross-process write races** once an app and ruin both write the same files (Obsidian
  external re-read; Bear iCloud sync mid-`replace_all`). Needs a reconcile/last-write-wins
  policy instead of today's single-process model.
- **Bear is structurally a poor backend** for this vision: no files, no atomic writes,
  poll-only, GUI-bound, macOS-locked. Anything implying Obsidian/Bear feature parity will
  disappoint.
- **Behavior/compat drift:** demoting the indices and flipping `tag_frontmatter` under the
  obsidian profile changes search results, `tags list` counts, and which frontmatter keys
  ruin writes — real breaking changes for existing downstream tooling (call out per the
  "avoid breaking changes" rule in `CLAUDE.md`).

---

*Investigation conducted via a 13-agent workflow (5 codebase readers, 3 backend/tooling
research agents, 5 per-feature design agents). Code references above are current as of this
branch.*
