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

Today ruin *owns* a folder of Markdown files and treats its `.ruin/` indices and note
frontmatter as the hot-path source of truth. v1 inverts that: **Bear owns the notes** (storage,
rendering, sync, mobile), and ruin becomes the **write/derivation engine** on top of `bearcli`
that:

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
the old GUI-mediated `x-callback` scheme. Everything the five features need has a first-class
`bearcli` verb:

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

- **Baking fits Bear's grain.** Bear has no `parent:` pointer, but it *does* have
  `edit --find "## H" --insert-after` — so "setting a parent bakes the text in at write time"
  is arguably *more* native on Bear than ruin's current pointer model.
- **Headless + concurrency-safe.** `bearcli` needs no running app, and `overwrite --base`
  plus `--no-update-modified` give real primitives for safe rewrites and loop-free monitoring
  even while Bear/iCloud sync is active.

## 3. Frontmatter: just omit it in Bear mode

ruin keeps most of its state in YAML frontmatter (`uuid`, `created`, `updated`, `tags`,
`inline-tags`, `inherited-tags`, `parent`, `order`, `linked-cards`, `url`, `aliases`, `dates`).
Bear notes have none of that natively — but this is **not a problem**, for two reasons:

1. We *could* write a YAML block into the note body and Bear would just render it as text.
   Harmless if we ever need it — not our problem how Bear displays it.
2. We mostly **don't need it**. ruin's frontmatter was always a *derived cache*
   (`note.Parse` re-derives tags from the body on every load; `doctor` rebuilds every index
   from bodies). Once we query `bearcli` live, that cache stops being the source of truth — so
   **in Bear mode ruin simply omits frontmatter** and uses Bear-native metadata instead.

What each field maps to when frontmatter is dropped:

| ruin frontmatter field | In Bear mode |
|---|---|
| `uuid` | **Bear note ID** is the identity — no ruin uuid minted |
| `created` / `updated` | Bear-native (`show --fields created,modified`) |
| `tags` / `inline-tags` | Native Bear `#tags` in the body; queried via `tags list`, classified from body text |
| `inherited-tags` | Applied as **real Bear tags** (`tags add`) or computed live from the parent relationship |
| `parent` | **Baked** (inlined under the parent heading) — no pointer to store |
| `linked-cards` | Retired — Bear has native `[[wikilinks]]` + `@backlinks` |
| `order` / `aliases` / `url` / `dates` | Dropped — re-derivable from body/links, or expressed Bear-natively (pins, wikilinks, inline dates) |

**Net:** Bear-backed notes are clean, native Markdown with no ruin bookkeeping in them. The
only persistent side-band state ruin still needs is the **dynamic-embed dependency graph**
(`.ruin/deps.json`, §4.5) — which is about *generated content*, not frontmatter, and would
exist regardless of backend. If we later find one specific field we must persist, the
frontmatter-in-body escape hatch (or a tiny side-band map) is available — but the default is
to carry nothing.

---

## 4. Architecture seams (v1 builds the Bear + FS impls)

The seam is designed so Obsidian can slot in later (§9); v1 ships only **FS** (today's code,
the default) and **Bear** (`bearcli`).

### 4.1 `backend` config selector
A new field on the four-field `Config` struct (`internal/config/config.go`), following the
existing `*bool`-with-default + `RUIN_*` env + auto-surfaced-by-`config` pattern:
`backend: ruin | bear` for v1 (`obsidian` reserved), env `RUIN_BACKEND`, optional
`backend_cli_path` (default the bundled `bearcli`). Selecting `bear` implies "omit frontmatter,
query live" and turns off the frontmatter/index writers.

### 4.2 `BackendStore` — the adapter seam (core of v1)
An interface with an **FS** impl (today's `note.Save`/`vault.*`, writing frontmatter as now) and
a **Bear** impl that shells out to `bearcli`, parses JSON, and **writes no frontmatter**.
Methods and their Bear mapping:

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
- **Mutations are silent** (exit-code only). `create` and reads return JSON; capture post-write
  state with a follow-up `show --fields hash` when needed; use `bearcli mcp-server` only if a
  structured write-response is genuinely required.
- **Respect Bear's body rules:** title derives from the first heading; `--tags` inserts at the
  Bear-configured position; attachment references must be preserved on `overwrite` (or pass
  `--force` deliberately). ruin's Bear serializer emits plain Markdown — **no YAML block, no
  ruin-managed keys.**

### 4.3 `MetadataProvider` — demote the indices, query Bear live
Route the read paths that currently *trust* `.ruin` indices / frontmatter —
`search_engine.go` `prefilterPathsViaTitles`/`hydrateNoteTagsFromIndex`, `tags list`
(`tags.go:61`, reads only `tags.yml`), and `RefreshLinkedCards`/`ResolveNote` — through
`QueryTags/Backlinks/ResolveTitle`. Under the Bear backend these are answered **live** by
`bearcli` (`tags list --format json`, `search "@backlinks"`, `show`). Because `bearcli` is
always available, ruin **doesn't maintain `.ruin/tags.yml`/`titles.json` at all in Bear mode**
(a thin cache is optional, purely for speed). `inherited-tags` is computed live (or applied as
Bear tags). Note resolution: title → `bearcli show --title`; internal references use the Bear
**ID**.

### 4.4 `SourceWatcher` — the monitor seam
Bear is a single DB, so there are no per-note file events: a **PollingWatcher** runs
`bearcli list --fields id,modified,hash --format json` on an interval and diffs against
last-seen hashes to produce a `ChangeSet`. Regeneration writes back via `edit`/`overwrite
--base` with `--no-update-modified` so ruin's own writes don't advance `modified` and
self-trigger. Works with Bear closed. (The FS impl uses `fsnotify`; the interface hides
poll-vs-watch.)

### 4.5 Dependency graph + generated-region markers
`.ruin/deps.json` (the one persistent side-band): `embed-id → {host Bear-ID, directive, source
IDs, output hash}` + the reverse index — the attribution data ruin already computes and
discards (`compose_dynamic.go:652`, `compose_walker.go:389`). Bear has no invisible-comment
syntax, so a generated region is delimited by a **visible marker** — a distinct heading such as
`## ⚙︎ ruin:<embed-id>` (open question: least-intrusive marker) — and refreshed by
`overwrite --base` or `edit` between markers.

---

## 5. Feature-by-feature plan (Bear-native)

### 5.1 Parent baking
`ruin note set <child> --parent <ref> --under "<heading>" --mode append|prepend` →
`bearcli edit <parent-id> --find "## <heading>" --insert-after "\n<child body>"`
(or `--insert-before` for prepend). Wrap the inserted block in a fenced marker so re-bake
replaces rather than duplicates. Because Bear has no pointer and no frontmatter, **baking is
the parent mechanism on Bear** (unlike FS, where the reversible pointer stays default);
relationships that matter for navigation are expressed natively as `[[wikilinks]]`/backlinks.
*Risk:* reparent/un-bake means cutting the baked region and re-inserting elsewhere; cycle
detection and inherited-tag propagation can't use a pointer (compute from the current baked
structure / wikilinks instead).

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
`"@wikilinks"`, `show --fields todos,done`. Route through `MetadataProvider` (§4.3); ruin stops
writing `.ruin` indices in Bear mode. `linked-cards` frontmatter is retired (native links +
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
for changed notes refreshes dependent materialized embeds (via `deps.json`) and propagates
checkbox toggles. **Loop safety is mandatory:** self-written-ID + expected-hash suppression set,
and content-hash idempotency (skip no-op writes) — `bearcli`'s per-note `hash`, `overwrite
--base`, and `--no-update-modified` make this exact. Add the clock tick for time-based embeds.
Runs headless (Bear can be closed). *Risk:* a new long-lived daemon (ruin has been one-shot
only) → lifecycle, single-writer coordination vs foreground commands, poll-interval latency.

---

## 6. Cross-cutting concerns (v1)

- **No frontmatter, no ruin indices in Bear mode** — notes are clean native Markdown; ruin's
  state is either Bear-native (ID, tags, links, timestamps) or, for generated content only,
  the `.ruin/deps.json` side-band.
- **Identity** — the Bear note **ID** is the identity. `ResolveNote` maps title →
  `show --title`, and internal refs use the ID. Bear titles come from the first heading, so
  title collisions are possible — prefer IDs internally. (Bookmarks/`parents.yml` become
  name → Bear-ID.)
- **Concurrency & loop safety** — `overwrite --base <hash>` (re-read on rejection), per-note
  `hash`, `--no-update-modified`. iCloud sync can change notes underneath; treat base-hash
  rejection as "re-read and retry," don't assume single-writer.
- **Silent mutations** — `edit`/`overwrite`/`append`/`tags` print nothing; exit code is the
  signal. Structured write results come from a follow-up `show` or the MCP server.
- **Encrypted/locked notes** — content inaccessible via `bearcli`; ruin skips them.
- **macOS-only** — accepted for v1; the FS backend remains the cross-platform/CI/headless-ruin
  path.
- **Bear body conventions** — first-heading title, `#hashtags`, attachment-link preservation on
  overwrite; ruin's Bear serializer must honor them and emit no YAML.

---

## 7. Decisions needed before building

1. **Baked child fate:** keep the child as its own Bear note (provenance + independent editing)
   or fold it entirely into the parent (true merge, child ceases to exist)?
2. **Parent on Bear = baking only, or also a lightweight relationship record** so
   `compose`/`tree`/`children` and reparenting keep working like today? (Native `[[wikilinks]]`
   +`@backlinks` may cover navigation without any side-band.)
3. **Generated-region marker on Bear** (no invisible comments): a dedicated heading, a fenced
   code block, or a footnote — least intrusive while reliably machine-locatable?
4. **Task/line identity** without block-refs: content-hash of the line, or inject a short
   trailing marker (e.g. `⟨r:ab12⟩`) to disambiguate duplicates?
5. **Do any ruin features hard-require a stable `uuid`** that Bear's ID can't stand in for
   (e.g. cross-references in existing exports)? If not, drop ruin uuids entirely in Bear mode.
6. **Monitor poll interval** and single-writer coordination between `ruin monitor` and
   foreground `ruin` commands.

---

## 8. Biggest risks (v1)

- **Content-based line identity** (no block refs) makes duplicate task lines ambiguous for
  two-way sync; needs a disambiguation scheme.
- **macOS-only, opaque store** — no plain files, no CI path on Bear; the FS backend must remain
  first-class for those cases.
- **Monitor feedback loops** — mitigated by `hash`/`--base`/`--no-update-modified` + self-write
  suppression; build the guard in from day one.
- **iCloud sync races** — Bear may rewrite notes underneath ruin; rely on `--base` and re-read.
- **Behavior/compat drift** — omitting frontmatter and `.ruin` indices and retiring
  `linked-cards` changes search results and `tags list` counts vs the FS backend; gate behind
  the `bear` profile (per the "avoid breaking changes" rule in `CLAUDE.md`).
- **Losing ruin-only conveniences** — anything Bear can't express natively (spaced tags
  `#tag with space#`, `@date` tokens, ordering) either degrades or moves to a Bear-native
  equivalent; decide per-feature what to drop.

---

## 9. Later: Obsidian (design for it, don't build it yet)

The seams above (`BackendStore`, `MetadataProvider`, `SourceWatcher`, `deps.json`, generated-
region markers) are backend-agnostic, so Obsidian can be added later as a third impl without
reworking the core. What Obsidian would add when we want it:

- **Cross-platform** and **plain `.md` on disk** (headless read + `fsnotify` monitor without
  the app; the `obsidian` CLI itself requires the running app).
- **Native frontmatter** via `obsidian property` get/set — so if we ever want ruin's richer
  metadata to persist visibly-structured, Obsidian is the home for it (whereas Bear mode omits
  it by design).
- **Semantic tasks** (`obsidian tasks` with completion) and **`eval`** (arbitrary plugin-API JS)
  as an escape hatch — e.g. heading-targeted insert, which the Obsidian CLI lacks natively.
- **Invisible `%%…%%` markers** for generated regions.

Trade-offs to revisit then: the Obsidian CLI needs the app running (FS fallback when closed),
it's early-access (1.12+, syntax may change), and it has no native heading-insert or concurrency
guard (Bear's `edit`/`--base` are stronger there). Net: Bear is the better *write engine* and
the better *headless* target for v1; Obsidian is the better *cross-platform / structured-metadata*
option to layer on afterward.

---

*Investigation conducted via a 13-agent workflow (5 codebase readers, 3 research agents, 5
per-feature design agents), then focused on Bear as v1 against the full `bearcli help all`
reference. Code references are current as of this branch. Re-verify `bearcli` specifics against
`bearcli help all` at implementation time, as the CLI is new (2026) and evolving.*
