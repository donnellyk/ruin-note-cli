# Implementation Plan

## Progress Checklist

- [x] **Phase 1**: Project Setup & Foundation
  - [x] 1.1 Initialize Go module
  - [x] 1.2 Config system
  - [x] 1.3 Vault initialization
- [x] **Phase 2**: Note Data Model
  - [x] 2.1 Frontmatter parsing
  - [x] 2.2 Note struct
  - [x] 2.3 Tag extraction
- [x] **Phase 3**: `log` Command
- [x] **Phase 4**: `search` Command
- [x] **Phase 5**: `update` Command
- [x] **Phase 6**: Additional Commands
  - [x] 6.1 `init` command
  - [x] 6.2 `config` command
  - [x] 6.3 `doctor` command
- [ ] **Phase 7**: Metadata Management
  - [ ] 7.1 Tags index
  - [ ] 7.2 Saved queries (deferred)
- [ ] **Phase 8**: CLI Polish
  - [ ] 8.1 Root command flags
  - [ ] 8.2 Error handling
  - [ ] 8.3 Testing

---

## CLI Specification (Finalized)

> Design target: **Script-first** (embedded tool). Human usability secondary.

### Global Flags

| Flag | Short | Env Var | Description |
|------|-------|---------|-------------|
| `--help` | `-h` | — | Show help |
| `--version` | — | — | Print version to stdout |
| `--vault` | — | `RUIN_VAULT` | Override vault path |
| `--config` | — | `RUIN_CONFIG` | Override config file path |
| `--json` | — | — | Output JSON to stdout (where supported) |
| `--no-color` | — | `NO_COLOR` | Disable colored output |

**Config precedence** (high → low): flags > env > config file (`~/.config/ruin`)

### Exit Codes

| Code | Meaning |
|------|---------|
| `0` | Success |
| `1` | No matches (search) or general error |
| `2` | Invalid usage / parse error |
| `3` | User aborted (declined confirmation) |

### I/O Contract

| Stream | Content |
|--------|---------|
| stdout | Primary output (paths, content, JSON) |
| stderr | Errors, confirmations |

**TTY behavior**: Prompts only when stderr is a TTY. Non-interactive mode requires `--force` for destructive operations.

---

## Phase 1: Project Setup & Foundation

### 1.1 Initialize Go Module
- Run `go mod init`
- Add dependencies: cobra, yaml.v3, uuid
- Create directory structure

### 1.2 Config System (`internal/config/`)
- Define config struct with `vault_path` field
- Load from `~/.config/ruin` (YAML format)
- Support `RUIN_CONFIG` env var override
- Support `RUIN_VAULT` env var override
- Create default config if missing
- Validate vault path exists

### 1.3 Vault Initialization (`internal/vault/`)
- Create `.ruin/` directory if missing
- Initialize empty `tags.yml` and `queries.yml`
- Provide vault path resolution

---

## Phase 2: Note Data Model

### 2.1 Frontmatter Parsing (`internal/note/frontmatter.go`)
- Parse YAML frontmatter between `---` delimiters
- Handle existing user frontmatter (preserve unknown fields)
- Serialize frontmatter back to YAML

### 2.2 Note Struct (`internal/note/note.go`)
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

### 2.3 Tag Extraction (`internal/note/tags.go`)
- Regex for simple tags: `#[\w/]+`
- Regex for spaced tags: `#[^#]+#`
- Separate logic for:
  - Global tags (under title or at document end)
  - Inline tags (within content body)
- Deduplicate tags

---

## Phase 3: `log` Command

### 3.1 Command Specification

```
ruin log [flags] [content]
```

**Input**: Content from positional arg, or stdin if `-` or no arg provided.

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--title` | `-t` | string | — | Set filename explicitly |
| `--h1` | — | bool | false | Extract filename from first H1 in content |
| `--stdin` | — | bool | false | Read content from stdin (explicit) |

### 3.2 Filename Resolution
Priority order:
1. `--title` flag value
2. `--h1`: extract first H1 from content
3. Timestamp: `YYYY-MM-DDTHH-MM-SS`

Sanitize filename (remove invalid chars)

### 3.3 Note Processing Pipeline
1. Parse input for existing frontmatter
2. Extract H1 title from content
3. Extract all tags (global vs inline)
4. Generate UUID if missing
5. Set created/updated timestamps
6. Write frontmatter + content to file

### 3.4 Output
- Default: filepath of created note
- `--json`: `{"path": "...", "uuid": "...", "title": "..."}`

### 3.5 Metadata Update
- Update `.ruin/tags.yml` with any new tags

### 3.6 Examples
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

## Phase 4: `search` Command

### 4.1 Command Specification

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

### 4.2 Sort Specification

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

```go
type SortField struct {
    Field     string // "created", "updated", "title"
    Ascending bool   // true = asc, false = desc
}

func ParseSort(s string) ([]SortField, error)
```

### 4.3 Search Query Language (MVP)

For MVP, support:
- Tag search: `#tagname`
- Text search: plain strings (case-insensitive)
- Explicit AND: `&&`
- Implicit AND: space between terms

**Deferred to v1.1+**: OR (`||`), NOT (`!`), grouping `()`, date filters, phrase search.

### 4.4 Search Optimization
- If query only contains tags:
  - Only read frontmatter (not full file)
  - Use `.ruin/tags.yml` for quick tag lookup
- Otherwise: full-text search

### 4.5 Output Formats
- Default: one filepath per line
- `--json`: `[{"path": "...", "uuid": "...", "title": "...", "tags": [...]}]`
- `--bulk`: content blocks separated by `%%%% <uuid> %%%%` (no frontmatter)
- `--first`: raw content of first match (no frontmatter)

### 4.6 Exit Codes
- `0`: matches found
- `1`: no matches (not an error, but distinguishable)

### 4.7 Editor Integration (`--edit`)
- Write temp file with bulk export format
- Open `$EDITOR`
- On close, call update logic with original + modified content
- No confirmation needed for saves

### 4.8 Examples
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

## Phase 5: `update` Command

### 5.1 Command Specification

```
ruin update [flags]
```

| Flag | Short | Type | Required | Description |
|------|-------|------|----------|-------------|
| `--original` | `-o` | string | yes | Original bulk export (file path or `-`) |
| `--updated` | `-u` | string | yes | Modified bulk export (file path or `-`) |
| `--force` | `-f` | bool | no | Skip confirmation for deletions |
| `--dry-run` | `-n` | bool | no | Show what would change, don't write |

### 5.2 Diff Logic
1. Parse original into map: `uuid -> content`
2. Parse updated into map: `uuid -> content`
3. Identify:
   - **Modified**: uuid in both, content differs → update file
   - **Deleted**: uuid in original only → **requires confirmation or `--force`**
   - **New**: uuid in updated only → error (use `log` to create new notes)

### 5.3 File Operations
- Modified: update file, refresh metadata (tags, updated timestamp)
- Deleted: remove file from disk (after confirmation)
- Update `.ruin/tags.yml` accordingly

### 5.4 Output
- Default: summary of changes (`Modified: 3, Deleted: 1`)
- `--json`: `{"modified": [...], "deleted": [...], "errors": [...]}`
- `--dry-run`: prefixes output with `[dry-run]`

### 5.5 Exit Codes
- `0`: success
- `1`: error (file not found, parse error, etc.)
- `3`: user aborted (declined confirmation)

### 5.6 Examples
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

## Phase 6: Additional Commands

### 6.1 `init` Command

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

### 6.2 `config` Command

```
ruin config [key] [value]
```

**Behavior**:
- No args: print all config (respects `--json`)
- One arg: print value for key
- Two args: set key to value

### 6.3 `doctor` Command

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
  ```
  Scanned 42 notes
    3 notes: generated missing UUID
    7 notes: reindexed tags
  Updated .ruin/tags.yml
  ```
- `--json`:
  ```json
  {
    "scanned": 42,
    "uuid_generated": ["path1.md", "path2.md", "path3.md"],
    "tags_reindexed": ["path1.md", ...],
    "tags_yml_updated": true
  }
  ```
- `--dry-run`: prefix output with `[dry-run]`, no files written

**Exit codes**:
- `0`: success (even if no changes needed)
- `1`: error (vault not found, parse error, etc.)

---

## Phase 7: Metadata Management

### 7.1 Tags Index (`internal/metadata/tags.go`)
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

### 7.2 Saved Queries (`internal/metadata/queries.go`)
```yaml
# .ruin/queries.yml
queries:
  - name: "daily-notes"
    query: "#daily && #2025"
```
- CRUD operations for queries
- (Implementation deferred - query syntax TBD)

---

## Phase 8: CLI Polish

### 8.1 Root Command (`cmd/ruin/main.go`)
- `--version` flag
- `-h/--help` flag
- `--vault` override flag
- `--config` override flag
- `--json` global flag
- `--no-color` flag

### 8.2 Error Handling
- User-friendly error messages to stderr
- Exit codes per specification above
- No stack traces by default

### 8.3 Testing
- Unit tests for tag extraction
- Unit tests for frontmatter parsing
- Unit tests for sort parsing
- Integration tests for each command

---

## Spaced Tag Rules (Finalized)

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

---

## Implementation Order

1. Phase 1 (setup) - foundation
2. Phase 2 (note model) - core data structures
3. Phase 6.1 (`init`) - vault initialization
4. Phase 3 (`log`) - first working command
5. Phase 7.1 (tags.yml) - needed for search optimization
6. Phase 4 (`search`) - read operations
7. Phase 5 (`update`) - write operations from search
8. Phase 6.2 (`config`) - config management
9. Phase 8 (polish) - cleanup and testing

---

## v2 Roadmap (Deferred)

- **Shell completions**: `ruin completion bash/zsh/fish`
- **`ruin tags`**: list/manage tags directly
- **`ruin query`**: saved query management
- **Extended search syntax**:
  - OR: `||`
  - NOT: `!`
  - Grouping: `()`
  - Date filters: `created:2025-01`, `updated:7d`
  - Phrase search: `"exact phrase"`
  - Title filter: `title:foo`
- **Custom order field**: `order` for manual sorting
