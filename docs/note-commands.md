# Note Commands

Programmatic single-note mutation commands for downstream consumers (e.g., TUI).

## note set

Set metadata and tags on a note.

```
ruin note set <note> [flags]
```

`<note>` is resolved via UUID, title substring, path substring, or parent bookmark.

### Flags

| Flag | Type | Description |
|------|------|-------------|
| `--add-tag` | string (repeatable) | Add global tag (`#` auto-added if missing). No-op if already present. |
| `--remove-tag` | string (repeatable) | Remove tag from all occurrences. No-op if not found. |
| `--order` | int | Set `order` frontmatter field |
| `--no-order` | bool | Unset `order` field |
| `--field` | string (repeatable) | Set extra frontmatter field (`key=value`). Empty value deletes. |
| `--parent` | string | Set parent (UUID, title, path, or bookmark) |
| `--no-parent` | bool | Remove parent |
| `--force` / `-f` | bool | Skip confirmation |

At least one mutation flag is required.

### Examples

```bash
# Add a tag
ruin note set <uuid> --add-tag "#urgent"
ruin note set <uuid> --add-tag urgent  # # prefix auto-added

# Remove a tag
ruin note set <uuid> --remove-tag "#wip"

# Batch multiple changes
ruin note set <uuid> --add-tag "#done" --remove-tag "#wip" --order 1

# Set/remove parent (replaces parent set/parent remove)
ruin note set <uuid> --parent "Hub Note"
ruin note set <uuid> --no-parent

# Set extra frontmatter field
ruin note set <uuid> --field "status=active"

# Delete extra frontmatter field
ruin note set <uuid> --field "status="
```

### JSON Output

```json
{
  "path": "path/to/note.md",
  "uuid": "...",
  "title": "...",
  "changes": [
    {"field": "tag", "action": "added", "value": "#urgent"},
    {"field": "order", "action": "set", "value": 5}
  ]
}
```

### Tag Insertion Algorithm

**`--add-tag`**: Finds the first tag-only line and appends the tag using the same separator (comma+space or space). If no tag-only line exists, inserts a new line after the title header.

**`--remove-tag`**: Removes all occurrences of the tag. Cleans up trailing separators and removes lines that become empty after removal.

## note append

Insert text into a note's content.

```
ruin note append <note> [text] [flags]
```

### Flags

| Flag | Type | Description |
|------|------|-------------|
| `--line` | int | Target content line (1-indexed, after frontmatter) |
| `--suffix` | bool | Append to end of line (requires `--line`) |
| `--raw-line` | bool | Line numbers count from top of file (including frontmatter) |
| `--stdin` | bool | Read text from stdin |
| `--force` / `-f` | bool | Skip confirmation |

Text from positional arg or `--stdin` (mutually exclusive, one required).

### Behavior

- No `--line`: append text as new line at end of content
- `--line N`: insert text as new line before line N (content lines only, after frontmatter)
- `--line N --suffix`: append text to end of line N
- `--line N --raw-line`: line numbers count from the top of the file including frontmatter. Errors if line N falls within the frontmatter block.

### Examples

```bash
# Append at end
ruin note append <uuid> "New paragraph"

# Insert before line 3 (content-relative, after frontmatter)
ruin note append <uuid> "Inserted line" --line 3

# Append to end of line 1
ruin note append <uuid> " (continued)" --line 1 --suffix

# Insert at file line 8 (including frontmatter lines)
ruin note append <uuid> "Inserted line" --line 8 --raw-line

# From stdin
echo "piped text" | ruin note append <uuid> --stdin
```

### JSON Output

```json
{
  "path": "path/to/note.md",
  "uuid": "...",
  "line": 5,
  "action": "appended"
}
```

## note merge

Merge source note into target.

```
ruin note merge <target> <source> [flags]
```

### Flags

| Flag | Type | Description |
|------|------|-------------|
| `--delete-source` | bool | Delete source note after merge |
| `--strip-title` | bool | Strip source's H1 title before appending |
| `--dry-run` / `-n` | bool | Preview changes without writing |
| `--force` / `-f` | bool | Skip confirmation |

### Behavior

1. Merge source's Extra frontmatter fields into target (target takes precedence)
2. Merge source's global tags into target (deduplicated)
3. Append source content to target with `\n\n` separator
4. Reparent source's children to target
5. If `--delete-source`: remove source file and clean up indexes

### Examples

```bash
# Basic merge
ruin note merge "Target Note" "Source Note" --force

# Merge with source deletion and title stripping
ruin note merge <target-uuid> <source-uuid> --strip-title --delete-source --force

# Preview
ruin note merge target source --dry-run
```

### JSON Output

```json
{
  "target_path": "...",
  "target_uuid": "...",
  "source_path": "...",
  "source_uuid": "...",
  "tags_merged": ["#foo"],
  "children_moved": 2,
  "source_deleted": false
}
```

## Post-Modification Pipeline

All three commands follow the same save pipeline:

1. `RefreshTags()` - re-extract tags from content
2. `ResolveDateTokens()` - resolve `@date` tokens
3. `RefreshDates()` - re-extract dates
4. `RefreshLinkedCards()` - resolve `[[wiki links]]`
5. `SetTimestamps()` - update `updated` field
6. `Save()` - write to disk
7. Update tags index (decrement old, increment new)
8. Update titles index
