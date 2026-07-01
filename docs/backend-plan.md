# Backend Direction: Bear as the v1 Storage Backend (via `bearcli`)

**Status:** Investigation + plan (no code changes yet)
**Branch:** `claude/obsidian-bear-backend-yhh1ei`
**Scope:** Use **Bear** as ruin's storage "backend" for v1, driven through Bear's official
`bearcli`, so Bear provides polish/rendering/sync while ruin (plus a Mac modal quick-capture)
handles capture and generated/dynamic pages. Design the backend seam so **Obsidian can be
added later** behind the same interface, but do not build it in v1.

> **`bearcli`** — `/Applications/Bear.app/Contents/MacOS/bearcli`, bundled in **Bear 2.8**
> (Apr 2026). **Operates directly on the local Bear database in place — the Bear app does not
> need to be running** — with no `x-callback`, no API token, and no window flashing. Reads emit
> structured JSON; writes are surgical (`edit --find/--insert-after`, `overwrite --base`).
> Also ships `bearcli mcp-server`. **macOS only.**
> Docs: <https://bear.app/faq/command-line-interface/>.

---

## 1. The vision

Today ruin *owns* a folder of Markdown files and treats its `.ruin/` indices as the hot-path
source of truth. v1 inverts that: **Bear owns the notes** (storage, rendering, sync, mobile),
and ruin becomes the **write/derivation engine** on top of `bearcli` that:

- captures input (CLI, and a Mac modal quick-capture),
- **bakes** structure in at write time — setting a parent inserts the child's text under the
  parent's heading, rather than storing a pointer,
- **materializes** dynamic pages — `pick` / embeds become real Bear notes that regenerate when
  their sources change,
- keeps plaintext tag/link files as **helpers**, not sources of truth — querying `bearcli`
  live instead,
- **two-way syncs** `pick`/checkbox done-state between the materialized view and origin,
- and runs a **monitor** mode that watches Bear and refreshes generated notes.

macOS-only is acceptable for v1: Bear is a macOS/iOS app, and `bearcli` is the point of
integration. ruin's existing **filesystem vault stays the default** for headless/CI/non-mac
use; the Bear backend is opt-in via config.

## 2. Why Bear fits well as v1

The official `bearcli` turns out to be a strong backend target — it's a native DB tool, not
the old GUI-mediated `x-callback` scheme. Concretely, everything the five features need has a
first-class `bearcli` verb:

| ruin needs | `bearcli` provides |
|---|---|
| Headless operation (no app, no token, no window flash) | Runs directly on the DB; Bear need not be running |
| Create / read / list / search | `create`, `cat`, `show`, `list`, `search`, `search-in` (all `--format json`) |
| Rich queries (tasks, links, dates, tags) | `search "@todo @done @task @backlinks @wikilinks @tagged @date(...)"` |
| **Insert under a specific heading** (baking) | `edit --find "## Heading" --insert-after "…"` / `--insert-before` |
| Find/replace a line (checkbox toggle) | `edit --find "- [ ] x" --replace "- [x] x"` (`--all`, `--word`) |
| **Safe region rewrite** (materialize refresh) | `overwrite --base <hash>` (optimistic lock) + `--no-update-modified` |
| Change detection (monitor) | per-note **`hash`** + `modified` via `show`/`list --fields` |
| Task state | `show --fields todos,done` |
| Tag list + management | `tags list/add/remove/rename/delete` (`--count`, JSON) |
| Escape hatch for structured write results | `bearcli mcp-server` (MCP over stdio) |
| Note identity | stable note **ID** or `--title` (case-insensitive) |

Two alignments make Bear an especially natural v1:

- **Baking fits Bear's grain.** Bear has no frontmatter and no notion of a `parent:` pointer,
  but it *does* have `edit --find "## H" --insert-after` — so the original vision ("setting a
  parent bakes the text in at write time") is arguably *more* native on Bear than ruin's
  current pointer model.
- **Headless + concurrency-safe.** `bearcli` needs no running app, and `overwrite --base`
  plus `--no-update-modified` give real primitives for safe rewrites and loop-free monitoring
  even while Bear/iCloud sync is active.

## 3. The one hard problem: Bear has no frontmatter

ruin keeps almost all of its state in YAML frontmatter: `uuid`, `created`, `updated`, `tags`,
`inline-tags`, `inherited-tags`, `parent`, `order`, `linked-cards`, `url`, `aliases`, `dates`.
**Bear notes have none of this** — a note is a title (first heading), a Markdown body with
inline `#tags`, and Bear-managed metadata (ID, created, modified, pins, todos). So v1's
central design task is deciding where each field lives:

| ruin field | v1 home on Bear |
|---|---|
| `uuid` | Bear note **ID** is the storage identity; keep a side-band `uuid ↔ Bear-ID` map so existing ruin tooling keyed on uuid still resolves |
| `created` / `updated` | Bear-native (`show --fields created,modified`); stop writing our own |
| `tags` / `inline-tags` | Native Bear `#tags` in the body; classify inline-vs-global from body text as today |
| `inherited-tags` | Apply as **real Bear tags** via `tags add`, recorded side-band so they can be recomputed/removed |
| `parent` | Baked (inlined) by default on Bear; provenance kept in the side-band map (+ optional native `[[Parent]]` wikilink) |
| `order` | Side-band map |
| `linked-cards` | **Retired** — Bear has native `[[wikilinks]]` and `@backlinks`; query live |
| `url` / `aliases` / `dates` | Side-band map (dates also re-derivable from body) |

**New v1 artifact: `.ruin/bear-meta.json`** — a side-band store keyed by Bear note ID holding
the non-native fields (uuid mapping, parent, order, inherited-tag provenance, url, aliases).
It is rebuildable by re-scanning Bear (`bearcli list --fields all`) the way `doctor` rebuilds
`titles.json` today. This is the Bear analogue of ruin's frontmatter-as-cache model.

---

## 4. Architecture seams (v1 builds the Bear + FS impls)

The seam is designed so Obsidian can slot in later (see §9), but v1 ships only **FS** (today's
code, the default) and **Bear** (`bearcli`).

### 4.1 `backend` config selector
A new field on the four-field `Config` struct (`internal/config/config.go`), following the
existing `*bool`-with-default + `RUIN_*` env + auto-surfaced-by-`config` pattern:
`backend: ruin | bear` for v1 (`obsidian` reserved), env `RUIN_BACKEND`, optional
`backend_cli_path` (default the bundled `bearcli`). Selecting `bear` also sets sensible
profile defaults (tag-frontmatter is moot; disable ruin-only formatting that Bear won't render,
e.g. `#spaced tag#` if it clashes — verify against Bear's tag rules).

### 4.2 `BackendStore` — the adapter seam (core of v1)
An interface with an **FS** impl (today's `note.Save`/`vault.*`) and a **Bear** impl that shells
out to `bearcli` and parses JSON. Methods and their Bear mapping:

```
Create(note) -> id,hash          // bearcli create --format json (returns id, hash)
Read(id) / Show(id)              // bearcli cat / show --fields all,content --format json
List() / Search(query)          // bearcli list / search --format json
Append(id, text, position)      // bearcli append --position beginning|end
InsertAtHeading(id, h, text, m) // bearcli edit --find "## h" --insert-after|--insert-before
ReplaceExact(id, old, new)      // bearcli edit --find old --replace new [--all]
Overwrite(id, content, base)    // bearcli overwrite --base <hash> [--no-update-modified]
QueryTags() / TagsOf(id)        // bearcli tags list [id] --format json
Backlinks(id) / Links(id)       // bearcli search "@backlinks"/"@wikilinks" --format json
Tasks(id) -> todos,done         // bearcli show --fields todos,done
Hash(id) / Modified(id)         // bearcli show --fields hash,modified
Trash/Archive/Restore/Pin/Tags* // bearcli trash|archive|restore|pin|tags add/remove/...
```

Notes on the Bear impl:
- **Mutations are silent** (exit-code only). Capture post-write state with a follow-up
  `show --fields hash` when needed; `create` and reads return JSON directly. Use
  `bearcli mcp-server` only if a structured write-response is genuinely required.
- **Respect Bear's body rules:** title derives from the first heading; `--tags` inserts at the
  Bear-configured position; attachment references must be preserved on `overwrite` (or pass
  `--force` deliberately). ruin's writers must not duplicate the H1 or drop attachment links.

### 4.3 `MetadataProvider` — demote the indices, query Bear live
Route the read paths that currently *trust* `.ruin` indices —
`search_engine.go` `prefilterPathsViaTitles`/`hydrateNoteTagsFromIndex`, `tags list`
(`tags.go:61`, reads only `tags.yml`), and `RefreshLinkedCards`/`ResolveNote` — through
`QueryTags/Backlinks/ResolveTitle`. Under the Bear backend these are answered **live** by
`bearcli` (`tags list --format json`, `search "@backlinks"`, `show`). Because `bearcli` is
always available (no app needed), `.ruin/tags.yml`/`titles.json` are largely **optional** on
Bear — kept only as a convenience/offline cache. `inherited-tags` stays ruin-computed (applied
as Bear tags). Note resolution maps title → `bearcli show --title`; uuid → `bear-meta.json` →
Bear ID.

### 4.4 `SourceWatcher` — the monitor seam
Bear is a single DB, so there are no per-note file events: use a **PollingWatcher** that runs
`bearcli list --fields id,modified,hash --format json` on an interval and diffs against
last-seen hashes to produce a `ChangeSet`. Regeneration writes back via `edit`/`overwrite
--base` with `--no-update-modified` so ruin's own writes don't advance `modified` and
self-trigger. Works with Bear closed. (The FS impl uses `fsnotify`; the interface hides
poll-vs-watch.)

### 4.5 Dependency graph + generated-region markers
`.ruin/deps.json`: `embed-id → {host Bear-ID, directive, source IDs, output hash}` + the
reverse index — the attribution data ruin already computes and discards
(`compose_dynamic.go:652`, `compose_walker.go:389`). Bear has no invisible-comment syntax, so a
generated region is delimited by a **visible marker** — a distinct heading such as
`## ⚙︎ ruin:<embed-id>` (open question: least-intrusive marker) — and refreshed by
`overwrite --base` or `edit` between markers.

---

## 5. Feature-by-feature plan (Bear-native)

### 5.1 Parent baking
`ruin note set <child> --parent <ref> --under "<heading>" --mode append|prepend` →
`bearcli edit <parent-id> --find "## <heading>" --insert-after "\n<child body>"`
(or `--insert-before` for prepend). Wrap the inserted block in a fenced marker so re-bake
replaces rather than duplicates. Provenance (child ↔ parent) recorded in `bear-meta.json`;
optionally add a native `[[Parent Title]]` wikilink for Bear navigation.

Because Bear has no pointer, **baking is the primary parent mechanism on the Bear backend**
(unlike FS, where the reversible pointer stays default). Keep the child note as its own note
for provenance, or fold it in — see decisions §7.

*Risk:* `--no-parent`/reparent means cutting the baked region from one note and re-inserting in
another; cycle detection and the inherited-tags cascade can't key off a pointer and must use
`bear-meta.json`.

### 5.2 Dynamic embeds → materialized Bear notes
ruin evaluates the embed as today, then writes the result to Bear: a dedicated generated note
via `create`, or a fenced region inside a host note. Refresh a region by reading
`show --fields hash,content`, splicing, and `overwrite --base <hash> --no-update-modified`
(hash guard prevents clobbering a concurrent Bear/iCloud edit). Invalidate via `.ruin/deps.json`
on source change; exclude generated notes/regions from `pickCandidatePaths`/search/tag-counting
so ruin never re-ingests its own output. Add a **clock tick** for time-dependent embeds
(`@today`, dated `query:`).

### 5.3 Plaintext tags/links as helpers — query `bearcli` live
`bearcli tags list --format json` (counts via `--count`), `bearcli search "@backlinks"` /
`"@wikilinks"`, `show --fields todos,done`. Route through `MetadataProvider` (§4.3); `.ruin`
indices become optional. `linked-cards` frontmatter is retired on Bear (native links +
`@backlinks`). *Risk:* `tags list` counts differ from ruin's per-note-deduped body count —
JSON consumers may see different numbers.

### 5.4 `pick` two-way done-sync
Read state with `show --fields todos,done` (or `search "@todo"`). Toggle a specific box with
`bearcli edit <id> --find "- [ ] <task text>" --replace "- [x] <task text>"` — real per-line
sync, **no whole-note rewrite**. Identity is content-based (exact task text); Bear has no
block-ref anchor, so duplicate identical task lines are ambiguous — dedupe by surrounding
context or a short injected marker. Unify ruin's two done encodings (`#done` tag vs `[x]`).
*Risk:* `pick` becomes a writer; `--toggle-todo` becomes cross-note.

### 5.5 Monitor mode
`ruin monitor` polls `bearcli list --fields id,modified,hash` on an interval, diffs hashes, and
for changed notes: re-derives ruin metadata into `bear-meta.json`, refreshes dependent
materialized embeds (via `deps.json`), and propagates checkbox toggles. **Loop safety is
mandatory:** self-written-ID + expected-hash suppression set, and content-hash idempotency
(skip no-op writes) — `bearcli`'s per-note `hash`, `overwrite --base`, and `--no-update-modified`
make this exact. Add the clock tick for time-based embeds. Runs headless (Bear can be closed).
*Risk:* a new long-lived daemon (ruin has been one-shot only) → lifecycle, single-writer
locking vs foreground commands, poll-interval latency vs CPU.

---

## 6. Cross-cutting concerns (v1)

- **Side-band metadata store (`.ruin/bear-meta.json`)** — the heart of v1; keyed by Bear ID,
  holds uuid mapping, parent/order, inherited-tag provenance, url/aliases; rebuildable from
  `bearcli list --fields all`.
- **Identity & resolution** — Bear ID is storage identity; `ResolveNote` maps title →
  `show --title`, uuid → `bear-meta.json`. Bear titles come from the first heading, so title
  collisions are possible; prefer IDs internally.
- **Concurrency & loop safety** — `overwrite --base <hash>` (re-read on rejection), per-note
  `hash`, `--no-update-modified`. iCloud sync can change notes underneath; treat base-hash
  rejection as "re-read and retry."
- **Silent mutations** — `edit`/`overwrite`/`append`/`tags` print nothing; the exit code is the
  signal. Structured write results (if needed) come from a follow-up `show` or the MCP server.
- **Encrypted/locked notes** — content inaccessible via `bearcli`; ruin must skip them.
- **macOS-only** — accepted for v1; the FS backend remains the cross-platform/CI/headless-ruin
  path. Don't imply Bear works off macOS.
- **`inherited-tags`** — no Bear equivalent; materialize as real tags and track provenance so a
  recompute can add/remove them cleanly.
- **Bear body conventions** — first-heading title, `#hashtags`, attachment link preservation on
  overwrite; ruin's serializer must honor them.

---

## 7. Decisions needed before building

1. **Baked child fate:** keep the child as its own Bear note (provenance + independent
   editing) or fold it entirely into the parent (true merge, child ceases to exist)?
   Recommend keeping it and recording the relationship in `bear-meta.json`.
2. **How much ruin metadata to preserve vs adopt Bear-native:** e.g. drop ruin `uuid` in favor
   of Bear ID entirely, or keep the `uuid ↔ ID` map for backward-compatible tooling?
   (Recommend keeping the map in v1.)
3. **Generated-region marker on Bear** (no invisible comments): a dedicated heading, a fenced
   code block, or a footnote — least intrusive while reliably machine-locatable?
4. **Task/line identity** without block-refs: content-hash of the line, or inject a short
   trailing marker (e.g. `⟨r:ab12⟩`) to disambiguate duplicates?
5. **Parent on Bear = baking only, or also a side-band pointer** (so `compose`/`tree`/`children`
   and reparenting still work like today)? Recommend side-band pointer *plus* optional baking.
6. **Monitor poll interval** and single-writer coordination between `ruin monitor` and
   foreground `ruin` commands sharing `bear-meta.json`.

---

## 8. Biggest risks (v1)

- **No frontmatter** is the defining constraint: all of ruin's state must move to
  `bear-meta.json` and/or Bear-native tags/links. Get this model right first — everything else
  depends on it.
- **Content-based line identity** (no block refs) makes duplicate task lines ambiguous for
  two-way sync; needs a disambiguation scheme.
- **macOS-only, opaque store** — no plain files, no CI path on Bear; the FS backend must remain
  first-class for those cases.
- **Monitor feedback loops** — mitigated by `hash`/`--base`/`--no-update-modified` + self-write
  suppression; build the guard in from day one.
- **iCloud sync races** — Bear may rewrite notes underneath ruin; rely on `--base` and re-read,
  don't assume single-writer.
- **Behavior/compat drift** — demoting `.ruin` indices and retiring `linked-cards` changes
  search results and `tags list` counts (per the "avoid breaking changes" rule in `CLAUDE.md`);
  gate behind the `bear` profile.

---

## 9. Later: Obsidian (design for it, don't build it yet)

The seams above (`BackendStore`, `MetadataProvider`, `SourceWatcher`, `deps.json`, generated-
region markers) are backend-agnostic, so Obsidian can be added later as a third impl without
reworking the core. What Obsidian would add when we want it:

- **Cross-platform** and **plain `.md` on disk** (headless read + `fsnotify` monitor without
  the app; the `obsidian` CLI itself requires the running app).
- **Native frontmatter** via `obsidian property` get/set — a closer home for ruin's uuid/parent/
  tags/dates than Bear's side-band map (potentially no `*-meta.json` needed).
- **Semantic tasks** (`obsidian tasks` with completion) and **`eval`** (arbitrary plugin-API JS)
  as an escape hatch — e.g. heading-targeted insert, which the Obsidian CLI lacks natively.
- **Invisible `%%…%%` markers** for generated regions.

Trade-offs to revisit then: the Obsidian CLI needs the app running (FS fallback when closed),
it's early-access (1.12+, syntax may change), and it has no native heading-insert or concurrency
guard (Bear's `edit`/`--base` are stronger there). Net: Bear is the better *write engine* and
the better *headless* target for v1; Obsidian is the better *data-model fit* and *cross-platform*
option to layer on afterward.

---

*Investigation conducted via a 13-agent workflow (5 codebase readers, 3 research agents, 5
per-feature design agents), then focused on Bear as v1 against the full `bearcli help all`
reference. Code references are current as of this branch. Re-verify `bearcli` specifics against
`bearcli help all` at implementation time, as the CLI is new (2026) and evolving.*
