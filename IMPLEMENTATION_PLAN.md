# Implementation Plan

## Project Info

- **Module**: `kvnd/ruin-note-cli`
- **License**: MIT

## Todos
- [ ] Handle commas in tags (non-alphanumeric end the tag?)
- [ ] Wiki link parsing and indexing in YML (and how titles will work for wikilinks)
- [ ] Parents links to compose full documents
- [ ] Inline search w/ basic editing and todo support (might be more of a TUI feature)
- [ ] Ability, from bulk edit, to make a new file - inserting `%%% %%%` moves it to a new file or something like that.
- [ ] Option to filter 'global' tags out of the note and return seperately 
  - So the UI can display them consistently, when viewing...

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
- [x] **Phase 12**: Frontmatter Enhancements
  - [x] 12.1 `--frontmatter[=MODE]` flag (extra, full, none)
  - [x] 12.2 Frontmatter editing in `update` command
- [x] **Phase 13**: Tag Management
  - [x] 13.1 `tags list` subcommand
  - [x] 13.2 `tags rename` subcommand
  - [x] 13.3 `tags delete` subcommand
- [x] **Phase 14**: Documentation
  - [x] 14.1 CLI reference (`docs/cli-reference.md`)
  - [x] 14.3 Enhanced `--help` with examples (already in command definitions)
- [x] **Phase 15**: Parent Links & Document Composition
  - [x] 15.1 Data model: `parent` field in frontmatter + `titles.json` index
  - [x] 15.2 `parent` command group (set, get, remove, children, tree)
  - [x] 15.3 `suggest` command (title prefix matching via titles index)
  - [x] 15.4 `compose` command (recursive document assembly)
  - [x] 15.5 Existing command integration (log --parent, search parent:, get --uuid, doctor titles.json)
  - [x] 15.6 Note resolution (UUID, title substring, path substring)

### Milestone 3: Advanced Features (Future)

- [ ] **Phase 16**: Graph & Links
  - [x] 16.0 Wiki link parsing (`[[title]]`, `[[title|display]]`) and `linked-cards` frontmatter
  - [ ] 16.1 Backlinks command
  - [ ] 16.2 Graph export (DOT format)
  - [ ] 16.3 `links-to:` search filter
  - [ ] 16.4 Links index (`.ruin/links.yml` or similar)
- [x] **Phase 17**: Tag Separation & Pick
  - [x] 17.1 Custom sort order (`order` field)
  - [x] 17.2 Separate global/inline tags in frontmatter (`tags` = global only, `inline-tags` = inline only)
  - [x] 17.3 `--global-tags` / `--inline-tags` search flags
  - [x] 17.4 `pick` command (inline tag line extraction with AND/OR mode)
- [ ] **Phase 18**: Extended Functionality
  - [ ] 18.1 Note archiving
  - [ ] 18.2 Extended search operators (OR, NOT, grouping, phrase)

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

### Shell Completions

```
ruin completion <shell>
```

Generate completion scripts for bash, zsh, fish, powershell using Cobra's built-in generation.

### Extended Markdown Documentation

Full documentation site with guides and concepts:
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

### Note Templates

```
ruin log --template <name> [content]
```

**Template Location**: `.ruin/templates/<name>.md`

**Variables**: `{{.Date}}`, `{{.DateTime}}`, `{{.Content}}`, `{{.Title}}`

### Version Control Integration

Git integration for vault history:
```
ruin history [note-path]
ruin diff [note-path] [revision]
ruin restore <note-path> <revision>
ruin sync
```

Consider Jujutsu as alternative for automatic snapshotting.

---

## Completed Phase Details

See [docs/completed-phases.md](docs/completed-phases.md) for detailed specifications of completed phases.
