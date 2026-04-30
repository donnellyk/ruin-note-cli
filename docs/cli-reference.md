# CLI Reference

## Global Flags

| Flag | Short | Env Var | Description |
|------|-------|---------|-------------|
| `--help` | `-h` | | Show help |
| `--version` | | | Print version |
| `--vault` | | `RUIN_VAULT` | Override vault path |
| `--config` | | `RUIN_CONFIG` | Override config file path |
| `--json` | | | Output JSON (where supported) |
| `--no-color` | | `NO_COLOR` | Disable colored output |

## Environment Variables

| Var | Description |
|-----|-------------|
| `RUIN_VAULT` | Override vault path |
| `RUIN_CONFIG` | Override config file path |
| `RUIN_VERSIONING` | Set to `false` to disable git auto-versioning |
| `RUIN_TAG_INHERITANCE` | Set to `false` to disable inherited tags from parent notes |
| `NO_COLOR` | Disable colored output |

## Exit Codes

| Code | Meaning |
|------|---------|
| `0` | Success |
| `1` | No matches or general error |
| `2` | Invalid usage |
| `3` | User aborted |

## Commands

### init

Initialize a notes vault.

```
ruin init [path]
```

| Flag | Short | Description |
|------|-------|-------------|
| `--force` | `-f` | Overwrite existing metadata files and skip the doctor confirmation prompt when notes already exist |
| `--no-git` | | Skip git repository initialization |
| `--config` | | Create `~/.config/ruin/` directory and `config.yml` with vault path |

Creates `.ruin/` directory with `tags.yml` and `queries.yml`. Initializes a git repository for automatic version history (unless `--no-git`). If path provided, updates config. Use `--config` to also create the config directory and file when initializing in the current directory (no path argument).

If the target directory already contains markdown notes (e.g., when migrating from Obsidian), init offers to run `ruin doctor` to build the tags and titles indices. Doctor may rewrite frontmatter — adding a `uuid` to notes that don't have one, normalizing tags (a leading `#` is added if missing), and rebuilding the `inline-tags` and `dates` fields from note bodies. Other frontmatter fields, key order, and comments are preserved. Decline the prompt to skip; run `ruin doctor` later when ready. `--force` skips the prompt and always runs doctor. Non-interactive invocations (no TTY) require `--force` to opt in.

### log

Create a new note.

```
ruin log [content]
```

| Flag | Short | Description |
|------|-------|-------------|
| `--title` | `-t` | Set filename explicitly |
| `--h1` | | Extract filename from first header |
| `--stdin` | | Read content from stdin |
| `--parent` | | Set parent note (UUID, title, or path substring) |
| `--order` | | Set manual sort order (integer) |

Content can be provided as argument, via stdin, or piped.

#### log extract

Extract and classify tags from content without creating a note.

```
ruin log extract [content]
```

| Flag | Short | Description |
|------|-------|-------------|
| `--title` | `-t` | Title for tag classification context |

Tags are classified as **global** (on tag-only lines) or **inline** (on lines with other content).

```
$ ruin log extract "#project #idea

Some thoughts here #wip"
global:
  #project
  #idea
inline:
  #wip
```

Supports `--json` for structured output with `global` and `inline` arrays.

### search

Search for notes.

```
ruin search <query>
```

**Query syntax:**
- Tag: `#tagname`
- Date: `@date` (matches dates referenced in note body)
- Text: `word` (case-insensitive)
- AND: `term1 && term2` or `term1 term2`

**Date tokens (`@` syntax):**
- `@today`, `@tomorrow`, `@yesterday`
- `@this-week`, `@last-week`, `@next-week`
- `@this-month`, `@last-month`, `@next-month`
- `@2026-02-13` (exact date)

Date tokens in queries are resolved dynamically. In note content, they are resolved to `@YYYY-MM-DD` for consistency. See [Date Tokens](date-tokens.md) for details.

**Date filters (metadata):**
- `created:DATE`, `updated:DATE`, `on:DATE`
- `before:DATE`, `after:DATE`
- `between:DATE,DATE`

**Date formats** (for filters):
- Exact: `2025-01-28`, `2025-01`, `2025`
- Natural: `today`, `yesterday`, `tomorrow`
- Relative arithmetic: `today+N`, `today-N` (rolling windows; N is a non-negative integer of days)
- Periods: `this-week`, `last-week`, `next-week`, `this-month`, `last-month`, `next-month`

**Other filters:**
- `title:TEXT`, `path:TEXT`
- `parent:UUID` — notes with specific parent
- `parent:none` — notes with no parent
- `tags:none` — notes with no tags (respects `--global-tags`/`--inline-tags`)
- `link:TEXT` — notes with URL containing text
- `todo:open` — notes with unchecked checkboxes (`- [ ]`)
- `todo:done` — notes with checked checkboxes (`- [x]`)
- `todo:any` — notes with any checkboxes

#### Negation

Prefix any search term with `!` to exclude matching notes.

```bash
ruin search "#project !#archived"      # #project but not #archived
ruin search "#followup !#done"         # open follow-ups
ruin search "meeting !title:draft"     # "meeting" but not drafts
ruin search "#daily !created:today"    # past daily notes, not today's
```

Negation works with all term types: tags, text, dates, title/path filters,
parent filters, and todo filters.

Multiple negations combine with AND — all exclusions must hold:

```bash
ruin search "#project !#archived !#draft"   # exclude both archived and draft
```

Negation-only queries are valid. `ruin search "!#archived"` returns every note
in the vault that is not tagged `#archived`.

The `--global-tags` and `--inline-tags` flags apply to negated tag terms the
same way they apply to positive terms.

| Flag | Short | Description |
|------|-------|-------------|
| `--bulk` | `-b` | Output with `%%%% <uuid> %%%%` separators |
| `--first` | | Output first match content only |
| `--edit` | `-e` | Open matches in `$EDITOR` |
| `--force` | `-f` | Skip confirmation for deletions in edit mode |
| `--frontmatter` | | Include frontmatter (modes: `extra`, `full`, `none`) |
| `--sort` | `-s` | Sort order (e.g., `created:desc`, `order:asc`) |
| `--limit` | `-l` | Max results |
| `--content` | | Include note content in JSON output (requires `--json`) |
| `--strip-global-tags` | | Remove global tags from content (requires `--content`) |
| `--strip-title` | | Remove title header from content (requires `--content`) |
| `--everything` | | Return all notes (no query required) |
| `--global-tags` | | Only match global tags (categorization) |
| `--inline-tags` | | Only match inline tags (contextual annotations) |
| `--link` | | Only match link notes (notes with a URL) |
| `--notes` | | Constrain to specific note UUIDs (comma-separated) |

By default, tag searches check both global and inline tags. Use `--global-tags` or `--inline-tags` to restrict scope (mutually exclusive).

### pick

Extract lines annotated with inline tags or markdown checkboxes.

```
ruin pick [inline-tags...] [@date...] [flags]
```

| Flag | Short | Description |
|------|-------|-------------|
| `--any` | | Match lines with any of the given tags (OR mode) |
| `--all` | | Include lines marked `#done` or `[x]` (default: excluded) |
| `--done` | | Show only lines marked `#done` or `[x]` |
| `--todo` | | Also match markdown checkbox lines (`- [ ]` / `- [x]`) |
| `--filter` | | Filter notes using search query syntax (e.g., `created:today`, `@tomorrow`, `before:2025-06`) |
| `--notes` | | Scope to specific notes by UUID (comma-separated or repeated) |
| `--parent` | | Scope to all descendants of a parent note (bookmark, UUID, or title) |
| `--sort` | `-s` | Sort order for notes (default `created:desc`). Fields: `created`, `updated`, `title`, `order` |

By default, multiple tags are combined with AND (lines must contain all tags).

Tags can be negated with `!` to exclude lines:

```bash
ruin pick "#followup" "!#done"         # open follow-ups, excluding #done lines
ruin pick "#task" "!#done" "!#deferred"  # exclude both #done and #deferred
```

Exclusions always apply as AND regardless of `--any`.

Tags are optional when `@date` is provided — `ruin pick @today` returns all lines with today's date annotation. Use `--todo` to also match markdown checkbox lines. When `--todo` or `@date` is provided, tags become optional. If tags are also provided, checkbox lines must contain those tags. The done filter applies uniformly: checked checkboxes (`[x]`) and `#done` lines are both treated as "done".

`@date` arguments filter at the line level — lines must contain an `@YYYY-MM-DD` date that falls within the resolved date range (e.g., `@2026-03` matches any date in March 2026, `@this-week` matches dates in the current week). When `@date` is the only argument (no tags, no `--todo`), all content lines are candidates and the date filter selects matching ones. `--filter "@date"` filters at the note level — only notes whose `dates` frontmatter includes the date are searched.

`@between:D1,D2` matches any line containing an `@`-tagged date token whose date falls within `[D1, D2]` inclusive. Endpoints accept all `dateparse` forms, including relative arithmetic (`today+6`, `today-30`). Mirrors `between:` in `ruin search`. Multiple date arguments AND together (a line must satisfy every range), so `@between:` mixes naturally with single `@date` arguments.

Examples:
```
ruin pick @today                                # All lines with today's date annotation
ruin pick @2026-03-15                           # All lines referencing March 15
ruin pick --todo @today --all                   # All checkboxes (open + done) with today's date
ruin pick "#followup" "@between:today,today+6"  # Followups dated within the next 7 days
ruin pick "@between:today-30,today"             # Any line dated within the last 30 days
```

Lines containing `#done` are excluded by default, `#done` is reserved to mark a line as resolved/completed. Use `--all` to include both open and done lines, or `--done` to show only completed lines.

Use `--notes` to scope pick to specific notes by UUID, or `--parent` to scope to all descendants of a given note (children, grandchildren, etc.). These flags are mutually exclusive and act as a pre-filter before tag matching and `--filter`. `--parent` resolves identifiers via bookmarks, UUIDs, title substrings, or path substrings. Unknown UUIDs in `--notes` produce a stderr warning (partial results are returned).

Tag-only lines (lines containing only tags and separators like commas) are treated as global tags and excluded from pick results.

**JSON output** (`--json`): Results grouped by note with matches array containing line number, content, all tags on each line, and a `done` boolean.

### get

Get a single note by path or title.

```
ruin get --path <path-substring>
ruin get --title <title-substring>
ruin get --uuid <uuid-or-identifier>
```

Requires one of `--path`, `--title`, or `--uuid` (mutually exclusive).

| Flag | Short | Description |
|------|-------|-------------|
| `--path` | | Match by file path (substring) |
| `--title` | | Match by title (case-insensitive substring) |
| `--uuid` | | Match by UUID (exact or via resolve) |
| `--frontmatter` | | Include frontmatter (modes: `extra`, `full`, `none`) |
| `--content` | | Include note content in JSON output (requires `--json`) |
| `--strip-global-tags` | | Remove global tags from content (requires `--content`) |
| `--strip-title` | | Remove title header from content (requires `--content`) |
| `--edit` | `-e` | Open note in `$EDITOR` for editing |
| `--force` | `-f` | Skip confirmation for deletions in edit mode |

Returns the first match if multiple notes match. Returns an error if no match found.

### update

Apply changes from a `--bulk --edit`.

```
ruin update -o <original> -u <updated>
```

| Flag | Short | Description |
|------|-------|-------------|
| `--original` | `-o` | Original bulk export file (required) |
| `--updated` | `-u` | Updated bulk export file (required) |
| `--force` | `-f` | Skip confirmation for deletions |
| `--dry-run` | `-n` | Show changes without writing |

Compares original and updated bulk exports to identify modified and deleted notes.

### today

Show notes from today.

```
ruin today
```

| Flag | Short | Description |
|------|-------|-------------|
| `--created` | `-c` | Only notes created today |
| `--updated` | `-u` | Only notes updated today |

Supports same output flags as `search`.

### yesterday

Show notes from yesterday.

```
ruin yesterday
```

Same flags as `today`.

### query

Manage saved queries.

#### query save

```
ruin query save <name> <query>
```

| Flag | Short | Description |
|------|-------|-------------|
| `--force` | `-f` | Skip confirmation |

#### query list

```
ruin query list
```

#### query delete

```
ruin query delete <name>
```

| Flag | Short | Description |
|------|-------|-------------|
| `--force` | `-f` | Skip confirmation |

#### query run

```
ruin query run <name>
```

Supports same output flags as `search`.

### tags

Manage tags in the vault.

#### tags list

```
ruin tags list
```

Lists all tags with usage counts and scope. Each tag is annotated with its scope: `global` (categorization tags), `inline` (contextual annotations within content), or `both`.

| Flag | Short | Description |
|------|-------|-------------|
| `--sort` | `-s` | Sort by: `name`, `name:desc`, `count`, `count:asc`, `count:desc` |
| `--min` | | Only show tags with at least N uses |

#### tags rename

```
ruin tags rename <old-tag> <new-tag>
```

| Flag | Short | Description |
|------|-------|-------------|
| `--force` | `-f` | Skip confirmation |
| `--dry-run` | `-n` | Show changes without applying |

#### tags delete

```
ruin tags delete <tag>
```

| Flag | Short | Description |
|------|-------|-------------|
| `--force` | `-f` | Skip confirmation |
| `--dry-run` | `-n` | Show changes without applying |

### config

View or modify configuration.

```
ruin config [key] [value]
```

- No args: show all config
- One arg: show key value
- Two args: set key to value

Available keys: `vault_path`, `versioning`, `tag_inheritance`

See [configuration.md](configuration.md) for details on each key.

### doctor

Scan and repair vault metadata.

```
ruin doctor [paths...]
```

| Flag | Short | Description |
|------|-------|-------------|
| `--dry-run` | `-n` | Show changes without writing |

With no arguments, performs a full vault scan:
- Generate UUIDs for notes missing one
- Reindex tags from document content
- Resolve date tokens and rebuild `dates` frontmatter
- Resolve [[wiki links]] and rebuild `linked-cards`
- Rebuild `.ruin/tags.yml`
- Rebuild `.ruin/titles.json`
- Detect orphaned parent references

With file path arguments, reindexes only the specified files:
- Same per-file operations (UUID, tags, dates, `linked-cards`)
- Incremental index updates (no full rebuild)
- Useful after manual edits outside of ruin

```bash
# Full vault scan
ruin doctor

# Reindex specific files after manual edits
ruin doctor notes/edited-file.md
ruin doctor file1.md file2.md
```

### note

Mutate individual notes (metadata, content, merging).

#### note set

```
ruin note set <note> [flags]
```

| Flag | Short | Description |
|------|-------|-------------|
| `--add-tag` | | Add a tag (global by default, inline with `--line`; repeatable, `#` auto-added) |
| `--remove-tag` | | Remove a tag (all lines by default, specific line with `--line`; repeatable) |
| `--line` | | Target content line (1-indexed, after frontmatter) |
| `--add-date` | | Add a `@YYYY-MM-DD` date reference (repeatable, accepts `today`/`tomorrow`/etc) |
| `--remove-date` | | Remove a specific `@YYYY-MM-DD` date (repeatable) |
| `--remove-dates` | | Remove all `@YYYY-MM-DD` dates |
| `--order` | | Set `order` frontmatter field |
| `--no-order` | | Unset `order` field |
| `--field` | | Set extra frontmatter field (`key=value`, empty value deletes) |
| `--parent` | | Set parent (UUID, title, path, or bookmark) |
| `--no-parent` | | Remove parent |
| `--toggle-todo` | | Flip checkbox state `[ ]` ↔ `[x]` (requires `--line`) |
| `--sink` | | Reposition toggled item: completed items move below open todos, uncompleted items move to bottom of open todos (requires `--toggle-todo`) |
| `--force` | `-f` | Skip confirmation |

At least one mutation flag required. Use `--line N` to target a specific content line for tag, date, and todo operations.

#### note append

```
ruin note append <note> [text] [flags]
```

| Flag | Short | Description |
|------|-------|-------------|
| `--line` | | Target content line (1-indexed, after frontmatter) |
| `--suffix` | | Append to end of line (requires `--line`) |
| `--raw-line` | | Line numbers count from top of file (including frontmatter) |
| `--stdin` | | Read text from stdin |
| `--force` | `-f` | Skip confirmation |

Without `--line`: appends at end. With `--line N`: inserts before line N. With `--line N --suffix`: appends to end of line N. With `--raw-line`: line numbers include frontmatter lines.

#### note delete

```
ruin note delete <note>
```

| Flag | Short | Description |
|------|-------|-------------|
| `--force` | `-f` | Skip confirmation |

Delete a note from the vault. Resolves the note by UUID, title, or path substring.

#### note merge

```
ruin note merge <target> <source> [flags]
```

| Flag | Short | Description |
|------|-------|-------------|
| `--delete-source` | | Delete source note after merge |
| `--strip-title` | | Strip source's H1 title before appending |
| `--dry-run` | `-n` | Preview changes without writing |
| `--force` | `-f` | Skip confirmation |

Merges source into target: frontmatter Extra fields (target takes precedence), global tags (deduplicated), content appended. Source's children are reparented to target.

### parent

Read-only parent-child queries and bookmark management.

#### parent get

```
ruin parent get <note>
```

Shows the parent of a note. Returns the parent's path (default) or JSON `{uuid, title, path}`.

#### parent children

```
ruin parent children <note>
```

| Flag | Short | Description |
|------|-------|-------------|
| `--recursive` | `-r` | Include all descendants |

Lists direct children of a note. With `--recursive`, shows the full subtree.

#### parent save

```
ruin parent save <name> <note>
```

| Flag | Short | Description |
|------|-------|-------------|
| `--force` | `-f` | Skip confirmation when overwriting |

Save a named bookmark mapping `<name>` to a note's UUID. The bookmark can be used anywhere a note reference is accepted (e.g., `--parent`, `compose`, other `parent` subcommands).

#### parent list

```
ruin parent list
```

List all saved parent bookmarks. Shows `name: title (uuid)` per line, or JSON array with `--json`.

#### parent delete

```
ruin parent delete <name>
```

| Flag | Short | Description |
|------|-------|-------------|
| `--force` | `-f` | Skip confirmation |

Delete a saved parent bookmark by name.

#### parent tree

```
ruin parent tree [note]
```

| Flag | Short | Description |
|------|-------|-------------|
| `--depth` | | Max tree depth (0 = unlimited) |

Without arguments, shows the full forest. With a note, shows the subtree rooted at that note.

### suggest

```
ruin suggest <prefix>
```

| Flag | Short | Description |
|------|-------|-------------|
| `--limit` | `-l` | Max results (default 10) |

Case-insensitive prefix match on note titles. Default output: `<uuid>\t<title>` per line.

### compose

```
ruin compose <note>
```

| Flag | Short | Description |
|------|-------|-------------|
| `--depth` | | Max recursion depth (0 = unlimited) |
| `--strip-title` | | Remove title header from root note |
| `--strip-global-tags` | | Remove global tag lines |
| `--sort` | `-s` | Child ordering: `field[:dir]` e.g. `created:desc`. Fields: `title` (default), `created`, `order` |
| `--edit` | `-e` | Open tree notes in `$EDITOR` |
| `--force` | `-f` | Skip confirmation for deletions in edit mode |
| `--content` | | Include per-node `content` fields in JSON output (requires `--json`) |
| `--normalize-headers` | | Normalize child headings so siblings share the same top-level |
| `--expand-embeds` | | Expand `![[note]]` embeds inline with referenced note content |
| `--explain` | | Print a decision log instead of composed content |

Recursively assembles a document from a note and its children. Headings in children are adjusted by depth level (capped at H6).

With `--normalize-headers`, each child's headings are rebased so its minimum heading level maps to its tree depth + 1. This ensures sibling notes at the same depth share the same top-level heading regardless of their original heading levels.

**Embed expansion** (`--expand-embeds`): Expands `![[Note Title]]` embeds inline with the referenced note's content. Supports `![[Note#Header]]` to embed only a specific header section. Embeds are only recognized on standalone lines (not inline within prose).

When a note is both embedded via `![[...]]` and a direct child, the embed takes precedence and the child is deduplicated. Embedded notes get their children expanded recursively beneath them.

Without `--expand-embeds`, `![[...]]` lines pass through as-is.

**Explain mode** (`--explain`): Outputs a decision log showing how the composition tree is resolved, without emitting actual note content. Useful for debugging. Works with `--json` for machine-readable output.

**JSON output** (`--json`): Always includes `composed_content` (flat composed text) and `source_map` (line-level mapping from composed output to source notes). The `--content` flag controls whether per-node `content` fields are populated (omitted by default). Embedded nodes include `"embedded": true`.

**Source map**: Each entry maps a range of composed output lines to a source note:

| Field | Type | Description |
|-------|------|-------------|
| `uuid` | string | Source note UUID |
| `path` | string | Source note file path |
| `title` | string | Source note title |
| `start_line` | int | First line in composed output (1-indexed) |
| `end_line` | int | Last line in composed output (1-indexed) |

Separator lines between notes fall in gaps not covered by any entry. To map a composed line back to the original note content: `original_line = (composed_line - start_line) + 1`. With embed expansion, a note's content may be split across non-contiguous line ranges (multiple source map entries with the same UUID).

**List merging**: When two adjacent sibling notes at the same depth both contain only list content (lines starting with `-`, `*`, `+`, or `1.`), they are separated by a single newline instead of a blank line. This causes the lists to merge into one contiguous list in the composed output.

### embed

Work with dynamic embeds.

```
ruin embed eval <embed-string>
ruin embed eval -                          # read embed from stdin
```

`embed eval` evaluates a single dynamic embed standalone and emits its results. It accepts the canonical full-delimiter form `![[type: query | options]]` and the bare inner form (`type: query | options`). A surrounding `![[ ]]` wrapper is stripped if present; canonical examples use the full form.

| Flag | Description |
|------|-------------|
| `--json` | Emit a typed JSON envelope instead of plain-text rendering |

Default output is plain-text rendering matching what each embed produces in compose-time output (search lists, pick groupings, compose expansion). With `--json`, emits a discriminated envelope:

```json
{
  "type": "search",
  "query": "#daily",
  "options": {"limit": "5"},
  "results": [...]
}
```

`results` shape per type:

| Embed type | `results` |
|------------|-----------|
| `search:` | `[]SearchResult` (same as `ruin search --json`) |
| `query:` | `[]SearchResult` (same as `ruin query run --json`) |
| `pick:` | `[]PickResult` (same as `ruin pick --json`) |
| `compose:` | `{ "expanded_markdown": string, "source_map": [...] }` |

`compose:` always includes `source_map` so callers can attribute composed lines back to source notes (same shape as `ruin compose --json`).

**Embed options in JSON mode**: Query-shaping options (`limit`, `sort`, `tag-scope`, ...) are honored identically in plain-text and JSON modes. Rendering options (`format=`) are silently ignored in `--json` mode so JSON callers receive a stable shape.

**Stdin**: Pass `-` as the argument to read the embed string from stdin. Useful for long embeds where shell quoting is awkward.

Examples:

```bash
# Plain-text rendering of a search embed
ruin embed eval "![[search: #daily | limit=5]]"

# Bare inner form (delimiters auto-added)
ruin embed eval "search: #daily | limit=5"

# JSON envelope for programmatic consumers
ruin embed eval "![[search: #daily]]" --json

# Read embed from stdin
echo "![[pick: #followup]]" | ruin embed eval -
```

Errors (malformed embed, unknown type, query syntax error, missing referenced note for `compose:` or saved query for `query:`) return non-zero exit with a structured error message.

## Note Format

Notes are markdown files with YAML frontmatter:

```markdown
---
uuid: abc-123
created: 2025-01-28T10:00:00-08:00
updated: 2025-01-28T10:00:00-08:00
tags:
  - "#daily"
  - "#work"
dates:
  - "2025-02-03"
---

# Note Title

Content with #tags inline and a date reference @2025-02-03.
```

**Managed fields** (set by CLI): `uuid`, `created`, `updated`, `tags` (global only), `inline-tags` (inline only), `inherited-tags` (global tags from ancestor notes), `dates` (referenced dates), `parent`, `order`, `linked-cards`

**User fields**: Any other YAML keys are preserved.

## Tag Syntax

- Simple: `#foo`, `#bar`, `#2025/may`
- Spaced: `#daily note#`
- Valid non-alphanumeric characters: `_`, `/`, `-` (hyphen cannot lead; trailing hyphens are stripped).

## Inherited Tags

When a note has a parent, the parent's global tags (and grandparent's, etc.) are automatically propagated to the child's `inherited-tags` frontmatter field. This is transitive — the entire ancestor chain is walked.

- `inherited-tags` is computed on save (`log`, `search --edit`, `update`, `note set/append/merge`)
- When a parent's global tags change, all descendants are cascade-updated
- `doctor` recomputes inherited tags for all notes and strips redundant inherited tags from content (tag-only lines)
- `search` matches inherited tags (via `EffectiveGlobalTags`)
- JSON output includes both `tags` (own + inherited merged) and `inherited_tags` (inherited only)
- Inherited tags are NOT counted in `tags.yml` (the parent already counts them)
- `AllTags()` (used for tag index) excludes inherited tags

## Wiki Links

Reference other notes using wiki-style links:

- Basic: `[[Note Title]]`
- With display text: `[[Note Title|display text]]`

Wiki links are resolved to UUIDs via exact case-insensitive title matching against the titles index. Resolved UUIDs are stored in the `linked-cards` frontmatter field. Unresolvable links produce a stderr warning and are omitted.

Wiki links are resolved during `log`, `search --edit`, `update`, and `doctor`.

## Bulk Format

Used by `--bulk` flag and `update` command:

```
%%%% uuid-1 %%%%
Content of first note...

%%%% uuid-2 %%%%
Content of second note...
```

## Vault Structure

```
<vault>/
├── .git/             # Auto-versioning (created by ruin init)
├── .gitignore        # Ignores .ruin/
├── .ruin/
│   ├── tags.yml      # Tag index (name, count, scope: global/inline/both)
│   ├── queries.yml   # Saved queries
│   ├── parents.yml   # Saved parent bookmarks
│   └── titles.json   # Titles index (UUID to title/path/parent)
├── Note Title.md
└── 2025-01-28T10-30-00.md
```

## Versioning

Ruin automatically commits note changes to git after each write operation. This provides version history for notes with no manual effort.

- **Enabled by default** when vault has a `.git/` directory
- **`ruin init`** creates the git repo automatically (opt out with `--no-git`)
- **`.ruin/`** is gitignored (indexes are derived data, rebuilt by `ruin doctor`)
- **One commit per command** (e.g., `ruin update` modifying 10 notes = 1 commit)
- **Failures are warnings** - a git error never blocks a note operation
- **Disable**: set `versioning: false` in config or `RUIN_VERSIONING=false` env var

Config (`~/.config/ruin/config.yml`):
```yaml
vault_path: ~/notes
versioning: true        # default if omitted
tag_inheritance: true   # default if omitted
```

See [configuration.md](configuration.md) for full configuration reference.
