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
| `--force` | `-f` | Overwrite existing metadata files |

Creates `.ruin/` directory with `tags.yml` and `queries.yml`. If path provided, updates config.

### log

Create a new note.

```
ruin log [content]
```

| Flag | Short | Description |
|------|-------|-------------|
| `--title` | `-t` | Set filename explicitly |
| `--h1` | | Extract filename from first H1 |
| `--stdin` | | Read content from stdin |
| `--parent` | | Set parent note (UUID, title, or path substring) |
| `--order` | | Set manual sort order (integer) |

Content can be provided as argument, via stdin, or piped.

### search

Search for notes.

```
ruin search <query>
```

**Query syntax:**
- Tag: `#tagname`
- Text: `word` (case-insensitive)
- AND: `term1 && term2` or `term1 term2`

**Date filters:**
- `created:DATE`, `updated:DATE`, `on:DATE`
- `before:DATE`, `after:DATE`
- `between:DATE,DATE`

**Date formats:**
- Exact: `2025-01-28`, `2025-01`, `2025`
- Natural: `today`, `yesterday`, `tomorrow`
- Relative: `this-week`, `last-week`, `this-month`, `last-month`
- Duration: `7d`, `2w`, `3m`

**Other filters:**
- `title:TEXT`, `path:TEXT`
- `parent:UUID` — notes with specific parent
- `parent:none` — notes with no parent

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
| `--strip-title` | | Remove H1 title from content (requires `--content`) |
| `--global-tags` | | Only match global tags (categorization) |
| `--inline-tags` | | Only match inline tags (contextual annotations) |

By default, tag searches check both global and inline tags. Use `--global-tags` or `--inline-tags` to restrict scope (mutually exclusive).

### pick

Extract lines annotated with inline tags.

```
ruin pick <inline-tags...>
```

| Flag | Short | Description |
|------|-------|-------------|
| `--any` | | Match lines with any of the given tags (OR mode) |

By default, multiple tags are combined with AND (lines must contain all tags).

Lines are extracted from the content body only -- global tag lines at the top or bottom of a note are excluded.

**JSON output** (`--json`): Results grouped by note with matches array containing line number, content, and all tags on each line.

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
| `--strip-title` | | Remove H1 title from content (requires `--content`) |
| `--edit` | `-e` | Open note in `$EDITOR` for editing |
| `--force` | `-f` | Skip confirmation for deletions in edit mode |

Returns the first match if multiple notes match. Returns an error if no match found.

### update

Apply changes from bulk edit.

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

Available keys: `vault_path`

### doctor

Scan and repair vault metadata.

```
ruin doctor
```

| Flag | Short | Description |
|------|-------|-------------|
| `--dry-run` | `-n` | Show changes without writing |

Operations:
- Generate UUIDs for notes missing one
- Reindex tags from document content
- Resolve [[wiki links]] and rebuild linked-cards
- Rebuild `.ruin/tags.yml`
- Rebuild `.ruin/titles.json`
- Detect orphaned parent references

### parent

Manage parent-child note relationships.

#### parent set

```
ruin parent set <child> <parent>
```

| Flag | Short | Description |
|------|-------|-------------|
| `--force` | `-f` | Skip confirmation when overwriting existing parent |

Both `<child>` and `<parent>` are resolved via UUID, title substring, or path substring. Validates no self-reference or cycle.

#### parent get

```
ruin parent get <note>
```

Shows the parent of a note. Returns the parent's path (default) or JSON `{uuid, title, path}`.

#### parent remove

```
ruin parent remove <note>
```

Removes the parent relationship from a note.

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
| `--strip-title` | | Remove H1 from children |
| `--strip-global-tags` | | Remove global tag lines |
| `--sort` | | Child ordering: `title` (default), `created`, or `order` |
| `--edit` | `-e` | Open tree notes in `$EDITOR` |
| `--force` | `-f` | Skip confirmation for deletions in edit mode |
| `--content` | | Include full composed document in JSON `content` field (requires `--json`) |

Recursively assembles a document from a note and its children. Headings in children are adjusted by depth level (capped at H6).

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
---

# Note Title

Content with #tags inline.
```

**Managed fields** (set by CLI): `uuid`, `created`, `updated`, `tags` (global only), `inline-tags` (inline only), `parent`, `order`, `linked-cards`

**User fields**: Any other YAML keys are preserved.

## Tag Syntax

- Simple: `#foo`, `#bar`, `#2025/may`
- Spaced: `#daily note#`

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
├── .ruin/
│   ├── tags.yml      # Tag index (name, count, scope: global/inline/both)
│   ├── queries.yml   # Saved queries
│   ├── parents.yml   # Saved parent bookmarks
│   └── titles.json   # Titles index (UUID to title/path/parent)
├── Note Title.md
└── 2025-01-28T10-30-00.md
```
