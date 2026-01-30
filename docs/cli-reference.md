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

| Flag | Short | Description |
|------|-------|-------------|
| `--bulk` | `-b` | Output with `%%%% <uuid> %%%%` separators |
| `--first` | | Output first match content only |
| `--edit` | `-e` | Open matches in `$EDITOR` |
| `--force` | `-f` | Skip confirmation for deletions in edit mode |
| `--frontmatter` | | Include frontmatter (modes: `extra`, `full`, `none`) |
| `--sort` | `-s` | Sort order (e.g., `created:desc`) |
| `--limit` | `-l` | Max results |

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
- Rebuild `.ruin/tags.yml`

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

**Managed fields** (set by CLI): `uuid`, `created`, `updated`, `tags`, `inline-tags`

**User fields**: Any other YAML keys are preserved.

## Tag Syntax

- Simple: `#foo`, `#bar`, `#2025/may`
- Spaced: `#daily note#`

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
│   ├── tags.yml      # Tag index
│   └── queries.yml   # Saved queries
├── Note Title.md
└── 2025-01-28T10-30-00.md
```
