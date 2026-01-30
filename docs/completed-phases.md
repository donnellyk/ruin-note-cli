# Completed Implementation Phases

Historical record of completed implementation phases. For current plan and upcoming work, see [IMPLEMENTATION_PLAN.md](../IMPLEMENTATION_PLAN.md).

---

## Milestone 1: Core CLI

### Phase 1: Project Setup & Foundation

#### 1.1 Initialize Go Module
- Run `go mod init`
- Add dependencies: cobra, yaml.v3, uuid
- Create directory structure

#### 1.2 Config System (`internal/config/`)
- Define config struct with `vault_path` field
- Load from `~/.config/ruin` (YAML format)
- Support `RUIN_CONFIG` env var override
- Support `RUIN_VAULT` env var override
- Create default config if missing
- Validate vault path exists

#### 1.3 Vault Initialization (`internal/vault/`)
- Create `.ruin/` directory if missing
- Initialize empty `tags.yml` and `queries.yml`
- Provide vault path resolution

---

### Phase 2: Note Data Model

#### 2.1 Frontmatter Parsing (`internal/note/frontmatter.go`)
- Parse YAML frontmatter between `---` delimiters
- Handle existing user frontmatter (preserve unknown fields)
- Serialize frontmatter back to YAML

#### 2.2 Note Struct (`internal/note/note.go`)
```go
type Note struct {
    UUID       string
    Created    time.Time
    Updated    time.Time
    Tags       []string
    InlineTags []string
    Title      string      // H1 header
    Content    string      // Full markdown content
    FilePath   string
    // Preserve other frontmatter fields
    ExtraFrontmatter map[string]interface{}
}
```

#### 2.3 Tag Extraction (`internal/note/tags.go`)
- Regex for simple tags: `#[\w/]+`
- Regex for spaced tags: `#[^#]+#`
- Separate logic for:
  - Global tags (under title or at document end)
  - Inline tags (within content body)
- Deduplicate tags

---

### Phase 3: `log` Command

#### 3.1 Command Specification

```
ruin log [flags] [content]
```

**Input**: Content from positional arg, or stdin if `-` or no arg provided.

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--title` | `-t` | string | — | Set filename explicitly |
| `--h1` | — | bool | false | Extract filename from first H1 in content |
| `--stdin` | — | bool | false | Read content from stdin (explicit) |

#### 3.2 Filename Resolution
Priority order:
1. `--title` flag value
2. `--h1`: extract first H1 from content
3. Timestamp: `YYYY-MM-DDTHH-MM-SS`

Sanitize filename (remove invalid chars)

#### 3.3 Note Processing Pipeline
1. Parse input for existing frontmatter
2. Extract H1 title from content
3. Extract all tags (global vs inline)
4. Generate UUID if missing
5. Set created/updated timestamps
6. Write frontmatter + content to file

#### 3.4 Output
- Default: filepath of created note
- `--json`: `{"path": "...", "uuid": "...", "title": "..."}`

#### 3.5 Metadata Update
- Update `.ruin/tags.yml` with any new tags

#### 3.6 Examples
```bash
# Pipe content
echo "# My Note\n\nContent here" | ruin log

# From argument
ruin log "Quick thought #idea"

# With explicit title
ruin log --title "Meeting Notes" "Discussion about X"

# Extract title from H1
ruin log --h1 "# Project Plan\n\nDetails..."

# JSON output for scripting
echo "content" | ruin log --json | jq -r '.uuid'
```

---

### Phase 4: `search` Command

#### 4.1 Command Specification

```
ruin search <query> [flags]
```

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--bulk` | `-b` | bool | false | Output content with `%%%% <uuid> %%%%` separators |
| `--first` | `-f` | bool | false | Output first match content only |
| `--edit` | `-e` | bool | false | Open matches in `$EDITOR`, pipe to update |
| `--sort` | `-s` | string | — | Sort order: `field:dir` |
| `--limit` | `-l` | int | 0 | Max results (0 = unlimited) |

**Mutual exclusivity**: `-b`, `-f`, `-e` are mutually exclusive.

#### 4.2 Sort Specification

**Syntax**: `field:direction[,field:direction]`

| Field | Description |
|-------|-------------|
| `created` | Note creation date |
| `updated` | Last modification date |
| `title` | Alphabetical by title/filename |

| Direction | Meaning |
|-----------|---------|
| `asc` | Ascending (oldest/A first) - **default** |
| `desc` | Descending (newest/Z first) |

#### 4.3 Search Query Language (MVP)

For MVP, support:
- Tag search: `#tagname`
- Text search: plain strings (case-insensitive)
- Explicit AND: `&&`
- Implicit AND: space between terms

#### 4.4 Output Formats
- Default: one filepath per line
- `--json`: `[{"path": "...", "uuid": "...", "title": "...", "tags": [...]}]`
- `--bulk`: content blocks separated by `%%%% <uuid> %%%%` (no frontmatter)
- `--first`: raw content of first match (no frontmatter)

#### 4.5 Exit Codes
- `0`: matches found
- `1`: no matches (not an error, but distinguishable)

#### 4.6 Editor Integration (`--edit`)
- Write temp file with bulk export format
- Open `$EDITOR`
- On close, call update logic with original + modified content
- No confirmation needed for saves

#### 4.7 Examples
```bash
# Tag search
ruin search "#daily"

# Text + tag
ruin search "#meeting project-alpha"

# JSON for scripting
ruin search "#todo" --json | jq '.[].uuid'

# Bulk export for editing
ruin search "#draft" --bulk > drafts.txt

# First match content
ruin search "#readme" --first

# Edit matching notes
ruin search "#blog !#published" --edit

# Sorted by newest
ruin search "#log" -s created:desc -l 10
```

---

### Phase 5: `update` Command

#### 5.1 Command Specification

```
ruin update [flags]
```

| Flag | Short | Type | Required | Description |
|------|-------|------|----------|-------------|
| `--original` | `-o` | string | yes | Original bulk export (file path or `-`) |
| `--updated` | `-u` | string | yes | Modified bulk export (file path or `-`) |
| `--force` | `-f` | bool | no | Skip confirmation for deletions |
| `--dry-run` | `-n` | bool | no | Show what would change, don't write |

#### 5.2 Diff Logic
1. Parse original into map: `uuid -> content`
2. Parse updated into map: `uuid -> content`
3. Identify:
   - **Modified**: uuid in both, content differs → update file
   - **Deleted**: uuid in original only → **requires confirmation or `--force`**
   - **New**: uuid in updated only → error (use `log` to create new notes)

#### 5.3 File Operations
- Modified: update file, refresh metadata (tags, updated timestamp)
- Deleted: remove file from disk (after confirmation)
- Update `.ruin/tags.yml` accordingly

#### 5.4 Output
- Default: summary of changes (`Modified: 3, Deleted: 1`)
- `--json`: `{"modified": [...], "deleted": [...], "errors": [...]}`
- `--dry-run`: prefixes output with `[dry-run]`

#### 5.5 Exit Codes
- `0`: success
- `1`: error (file not found, parse error, etc.)
- `3`: user aborted (declined confirmation)

#### 5.6 Examples
```bash
# Standard workflow
ruin search "#draft" --bulk > /tmp/original.txt
cp /tmp/original.txt /tmp/edited.txt
$EDITOR /tmp/edited.txt
ruin update -o /tmp/original.txt -u /tmp/edited.txt

# Preview changes
ruin update -o orig.txt -u new.txt --dry-run

# Non-interactive (script)
ruin update -o orig.txt -u new.txt --force --json

# Stdin for one side
cat edited.txt | ruin update -o orig.txt -u -
```

---

### Phase 6: Additional Commands

#### 6.1 `init` Command

```
ruin init [path]
```

| Flag | Type | Description |
|------|------|-------------|
| `--force` | bool | Overwrite existing `.ruin/` directory |

**Behavior**:
- Creates `.ruin/` with `tags.yml` and `queries.yml`
- If path provided, also sets `vault_path` in config
- Idempotent unless `--force`

**Output**:
- Default: `Initialized vault at /path/to/vault`
- `--json`: `{"vault": "/path/to/vault", "created": ["tags.yml", "queries.yml"]}`

#### 6.2 `config` Command

```
ruin config [key] [value]
```

**Behavior**:
- No args: print all config (respects `--json`)
- One arg: print value for key
- Two args: set key to value

#### 6.3 `doctor` Command

```
ruin doctor [flags]
```

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--dry-run` | `-n` | bool | false | Show what would change, don't write |

**Behavior**:
- Scans all `*.md` files in the vault
- For each note:
  - Generate `uuid` if missing
  - Reindex `tags` and `inline-tags` from document content
  - Do **NOT** update `created` or `updated` timestamps
  - Write updated frontmatter only if changes were made
- Rebuild `.ruin/tags.yml` from all notes

**Output**:
- Default: human-readable summary
- `--json`: detailed JSON with paths affected
- `--dry-run`: prefix output with `[dry-run]`, no files written

---

### Phase 7: Metadata Management

#### 7.1 Tags Index (`internal/vault/vault.go`)
```yaml
# .ruin/tags.yml
tags:
  - name: "#ruin"
    count: 5
  - name: "#log"
    count: 3
```
- Add/remove tags on note save/delete
- Maintain usage counts

#### 7.2 `query` Command

```
ruin query <subcommand> [args] [flags]
```

Subcommands:
- `query save <name> <query>` - Save a query
- `query list` - List saved queries
- `query delete <name>` - Delete a query
- `query run <name>` - Execute a saved query

---

### Phase 8: CLI Polish

#### 8.1 Root Command (`cmd/ruin/main.go`)
- `--version` flag
- `-h/--help` flag
- `--vault` override flag
- `--config` override flag
- `--json` global flag
- `--no-color` flag

#### 8.2 Error Handling
- User-friendly error messages to stderr
- Exit codes per specification
- No stack traces by default

#### 8.3 Testing
- Unit tests for tag extraction
- Unit tests for frontmatter parsing
- Unit tests for sort parsing
- Integration tests for each command

---

## Milestone 2: Enhanced Features (Completed Phases)

### Phase 9: Date Utilities

#### 9.1 Date Parsing Library (`internal/dateparse/`)

Natural language date parsing supporting:
- `today`, `yesterday`, `tomorrow`
- `this-week`, `last-week`, `this-month`, `last-month`, `this-year`
- Relative: `7d`, `2w`, `3m` (days/weeks/months ago)
- ISO dates: `YYYY-MM-DD`

#### 9.2 `today` Command

```
ruin today [flags]
```

Shows notes where `created` timestamp is today (local timezone).

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--bulk` | `-b` | bool | false | Output with `%%%% <uuid> %%%%` separators |
| `--first` | `-f` | bool | false | Output first match content only |
| `--edit` | `-e` | bool | false | Open matches in `$EDITOR` |
| `--sort` | `-s` | string | `created:desc` | Sort order (default newest first) |
| `--limit` | `-l` | int | 0 | Max results |
| `--updated` | `-u` | bool | false | Match on `updated` instead of `created` |

#### 9.3 `yesterday` Command

Same flags as `today`. Matches notes from the previous calendar day.

---

### Phase 10: Enhanced Search

Extended search query language with date filtering.

#### 10.1 Date Filter Syntax

| Filter | Description | Examples |
|--------|-------------|----------|
| `created:YYYY-MM-DD` | Exact date match | `created:2025-01-28` |
| `before:DATE` | Before date (exclusive) | `before:2025-01-28` |
| `after:DATE` | After date (exclusive) | `after:2025-01-01` |
| `on:DATE` | On date (alias for exact) | `on:2025-01-28` |
| `between:DATE,DATE` | Date range (inclusive) | `between:2025-01-01,2025-01-31` |
| `updated:...` | Same filters for updated field | `updated:2025-01` |

#### 10.2 Natural Language Dates

Supports: `today`, `yesterday`, `this-week`, `last-week`, `this-month`, `last-month`, `7d`, `2w`, `3m`

#### 10.3 Additional Filters

- `title:TEXT` - Filter by title/filename
- `path:TEXT` - Filter by file path

---

### Phase 11: Search Performance

#### 11.1 Concurrent File Reading

**Implementation**:
- Worker pool pattern using goroutines (capped at NumCPU, max 8 workers)
- Parallel file parsing across all search modes
- Channel-based result collection

#### 11.2 Early Termination for `--limit`

**Implementation**:
- `SearchOptions` struct with `Limit` field
- Early termination when limit is reached and no sorting is requested

#### Benchmark Results (Apple M3 Pro, 3s benchtime)

| Benchmark | Before | After | Speedup |
|-----------|--------|-------|---------|
| TagOnly_1000 | 19.5ms | 8.4ms | **2.3x** |
| TextSearch_1000 | 22.1ms | 10.8ms | **2.0x** |
| TagOnly_10000 | 254ms | 98ms | **2.6x** |
| TextSearch_10000 | 249ms | 98ms | **2.5x** |

#### Removed: Tag-Only Optimization

We initially implemented a tag-only optimization that parsed only frontmatter (first 1KB) for tag-only queries. This was removed because:
- Benchmarks showed minimal benefit (~10% at best) over full file parsing
- At 10,000 notes, tag-only and text search had nearly identical performance
- OS file caching made full-file reads almost as fast as partial reads
- The optimization added complexity without meaningful improvement

The concurrent I/O (worker pool) provides the majority of the speedup (~2x).

---

### Phase 12: Frontmatter Enhancements

#### 12.1 `--frontmatter` Flag

Unified flag for controlling frontmatter visibility in output.

```
--frontmatter[=MODE]   Include frontmatter in output
                       Modes: extra (default), full, none
```

| Mode | Description |
|------|-------------|
| `extra` | Show only user-defined fields (default when flag present) |
| `full` | Show complete frontmatter block |
| `none` | Hide frontmatter (default when flag absent) |

**Applies to**: `search`, `query run`, `today`, `yesterday`

#### 12.2 Update Command Frontmatter Handling

When `update` receives bulk content with frontmatter:
- Parse frontmatter from each section
- Apply frontmatter changes (merge with existing)
- Protect managed fields: `uuid` changes are errors, `created` is immutable
- Allow changes to: `tags` (overrides extraction), user fields

---

### Phase 13: Tag Management

#### 13.1 `tags list` Subcommand

```
ruin tags list [flags]
```

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--sort` | `-s` | string | `count:desc` | Sort by `name` or `count` |
| `--min` | | int | 0 | Only show tags with at least N uses |

**Output**: `#daily (15)` per line, or JSON array.

#### 13.2 `tags rename` Subcommand

```
ruin tags rename <old> <new> [flags]
```

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--force` | `-f` | bool | false | Skip confirmation |
| `--dry-run` | `-n` | bool | false | Show changes without applying |

#### 13.3 `tags delete` Subcommand

```
ruin tags delete <tag> [flags]
```

Same flags as `rename`. Removes tag from all notes.

---

### Phase 14: Documentation

#### 14.1 Man Pages - Skipped

Man page generation via `cobra-doc` was skipped in favor of markdown documentation.

#### 14.2 Markdown Documentation - Moved to Future Ideas

Full documentation site with guides and concepts moved to Future Ideas section.

#### 14.3 Enhanced `--help`

All commands include detailed help text with examples in their Cobra command definitions.

#### Actual Implementation

Created `docs/cli-reference.md` with minimal command documentation covering all commands, flags, and formats.

---

## Reference

### Spaced Tag Rules

**Rule**: Spaced tags cannot contain `#`. The `#` character ends the tag.

**Parsing algorithm**:
1. Find `#` that starts a tag
2. If next char is space or alphanumeric, check for closing `#`
3. If closing `#` found before newline → spaced tag
4. Otherwise → simple tag (ends at whitespace)

| Input | Parsed as |
|-------|-----------|
| `#simple` | Tag: "simple" |
| `#daily note#` | Tag: "daily note" |
| `#broken # tag#` | Tag: "broken", then text "# tag#" |
| `#foo#bar` | Tag: "foo", then text "bar" |

### Original Implementation Order

1. Phase 1 (setup) - foundation
2. Phase 2 (note model) - core data structures
3. Phase 6.1 (`init`) - vault initialization
4. Phase 3 (`log`) - first working command
5. Phase 7.1 (tags.yml) - needed for search optimization
6. Phase 4 (`search`) - read operations
7. Phase 5 (`update`) - write operations from search
8. Phase 6.2 (`config`) - config management
9. Phase 8 (polish) - cleanup and testing
