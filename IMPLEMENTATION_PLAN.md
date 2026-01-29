# Implementation Plan

## Progress Checklist

### Milestone 1: Core CLI (Complete)

- [x] **Phase 1**: Project Setup & Foundation (config, vault init)
- [x] **Phase 2**: Note Data Model (frontmatter, tags, note struct)
- [x] **Phase 3**: `log` Command
- [x] **Phase 4**: `search` Command
- [x] **Phase 5**: `update` Command
- [x] **Phase 6**: Additional Commands (`init`, `config`, `doctor`)
- [x] **Phase 7**: Metadata Management (tags index, `query` command)
- [x] **Phase 8**: CLI Polish (flags, error handling, testing)

### Milestone 2: Enhanced Features

- [x] **Phase 9**: Date Utilities
  - [x] 9.1 Date parsing library (`internal/dateparse`)
  - [x] 9.2 `today` command
  - [x] 9.3 `yesterday` command
- [x] **Phase 10**: Enhanced Search
  - [x] 10.1 Date filters (`created:`, `before:`, `after:`, `on:`, `between:`)
  - [x] 10.2 Natural language dates (`today`, `last-week`, `7d`, etc.)
  - [x] 10.3 Additional filters (`title:`, `path:`)
- [x] **Phase 11**: Search Performance
  - [x] 11.1 Concurrent file reading (2x speedup)
  - [x] 11.2 Early termination for `--limit`
  - [~] 11.3 Tag-only optimization (tried, removed - minimal benefit)
- [ ] **Phase 12**: Frontmatter Enhancements
  - [ ] 12.1 `--frontmatter[=MODE]` flag (extra, full, none)
  - [ ] 12.2 Frontmatter editing in `update` command
- [ ] **Phase 13**: Tag Management
  - [ ] 13.1 `tags list` subcommand
  - [ ] 13.2 `tags rename` subcommand
  - [ ] 13.3 `tags delete` subcommand
- [ ] **Phase 14**: Documentation
  - [ ] 14.1 Man page generation (`cobra-doc`)
  - [ ] 14.2 Markdown documentation (`docs/`)
  - [ ] 14.3 Enhanced `--help` with examples
- [ ] **Phase 15**: Developer Experience
  - [ ] 15.1 Shell completions (`completion` command)
  - [ ] 15.2 Note templates (`--template` flag)

### Milestone 3: Advanced Features (Future)

- [ ] **Phase 16**: Graph & Links
  - [ ] 16.1 Backlinks command
  - [ ] 16.2 Graph export (DOT format)
- [ ] **Phase 17**: Extended Functionality
  - [ ] 17.1 Custom sort order (`order` field)
  - [ ] 17.2 Note archiving
  - [ ] 17.3 Extended search operators (OR, NOT, grouping, phrase)

---

## CLI Specification

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

## Upcoming Phase Specifications

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

**Output by Mode**

Default output with `--frontmatter`:
```
/path/to/note.md
  author=kevin, status=draft
```

JSON output with `--frontmatter`:
```json
{
  "path": "/path/to/note.md",
  "uuid": "...",
  "extra": { "author": "kevin", "status": "draft" }
}
```

Bulk output with `--frontmatter=full`:
```
%%%% uuid-1 %%%%
---
uuid: uuid-1
created: 2025-01-28T10:00:00-08:00
tags: ["#daily", "#work"]
author: kevin
---

# Note Title

Content here...
```

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

#### 14.1 Man Pages

Generate using `cobra/doc` package. Target structure:
```
ruin(1)       - Main command overview
ruin-log(1)   - Creating notes
ruin-search(1) - Searching notes
...
```

#### 14.2 Markdown Documentation

```
docs/
├── getting-started.md
├── commands/
│   ├── log.md, search.md, query.md, ...
├── concepts/
│   ├── vault.md, frontmatter.md, tags.md, bulk-format.md
└── guides/
    ├── daily-notes.md, scripting.md, editor-integration.md
```

#### 14.3 Enhanced `--help`

Every command should have:
- Clear one-line description
- Multiple examples showing common patterns
- Flag descriptions that explain *why*, not just *what*

---

### Phase 15: Developer Experience

#### 15.1 Shell Completions

```
ruin completion <shell>
```

Generate completion scripts for bash, zsh, fish, powershell using Cobra's built-in generation.

#### 15.2 Note Templates

```
ruin log --template <name> [content]
```

**Template Location**: `.ruin/templates/<name>.md`

**Variables**: `{{.Date}}`, `{{.DateTime}}`, `{{.Content}}`, `{{.Title}}`

---

## Milestone 3 Specifications

### Phase 16: Graph & Links

#### 16.1 Backlinks Command

```
ruin backlinks <note-path-or-uuid> [flags]
```

Find notes linking to the specified note. Detects:
- Markdown links: `[text](note.md)`
- Wiki-style links: `[[Note Title]]`
- UUID references: `uuid:abc-123`

#### 16.2 Graph Export

```
ruin graph [flags]
```

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--format` | string | `dot` | Output format: `dot`, `json`, `mermaid` |
| `--filter` | string | | Only include notes matching query |

---

### Phase 17: Extended Functionality

#### 17.1 Custom Sort Order

Add `order` frontmatter field for manual sorting.

#### 17.2 Note Archiving

```
ruin archive <query> [flags]
ruin unarchive <query> [flags]
```

Move notes to/from `.ruin/archive/`.

#### 17.3 Extended Search Operators

| Operator | Description | Example |
|----------|-------------|---------|
| `\|\|` | OR | `#work \|\| #personal` |
| `!` or `-` | NOT | `#draft !#published` |
| `()` | Grouping | `(#work \|\| #personal) && #urgent` |
| `"..."` | Phrase | `"exact phrase"` |

---

## Technical Debt

### High Priority

**DRY Violation: `query run` duplicates search logic**

Location: `internal/commands/query.go`

The `query run` subcommand copy-pastes search command logic. Extract into shared `ExecuteSearch()` function.

### Medium Priority

**Direct `os.Exit(3)` in update command**

Location: `internal/commands/update.go`

Should return `ErrUserAborted` instead and let `main.go` handle exit code.

### Low Priority

**Confirmation prompt duplication**

Extract `confirmAction(prompt string) (bool, error)` helper.

---

## Future Ideas

### Version Control Integration

Git integration for vault history:
```
ruin history [note-path]
ruin diff [note-path] [revision]
ruin restore <note-path> <revision>
ruin sync
```

Consider Jujutsu as alternative for automatic snapshotting.

### Performance Benchmarking Infrastructure

Already implemented:
- `scripts/create-benchmark-vault.sh` - Create test vaults
- `PERFORMANCE.md` - Track results over time
- `make bench`, `make bench-baseline`, `make bench-compare`

Future: CI integration, regression tests.

---

## Completed Phase Details

See [docs/completed-phases.md](docs/completed-phases.md) for detailed specifications of completed phases.
