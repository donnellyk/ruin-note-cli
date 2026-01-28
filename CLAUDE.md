# Ruin Note CLI

A Zettelkasten-inspired note-taking CLI written in Go.

## Project Setup

### Prerequisites
- Go 1.21+ (install via `brew install go` on macOS or from https://go.dev/dl/)
- Set up Go environment: ensure `$GOPATH/bin` is in your `$PATH`

### Initialize Project
```bash
go mod init github.com/kevin/ruin-note-cli
```

### Build & Run
```bash
go build -o ruin ./cmd/ruin
./ruin --help
```

### Run Tests
```bash
go test ./...
```

### Install Locally
```bash
go install ./cmd/ruin
```

## Project Structure
```
ruin-note-cli/
├── cmd/
│   └── ruin/
│       └── main.go           # CLI entry point
├── internal/
│   ├── config/
│   │   └── config.go         # Config file handling (~/.config/ruin)
│   ├── vault/
│   │   └── vault.go          # Vault directory operations
│   ├── note/
│   │   ├── note.go           # Note struct and operations
│   │   ├── frontmatter.go    # YAML frontmatter parsing/writing
│   │   └── tags.go           # Tag extraction logic
│   ├── commands/
│   │   ├── log.go            # log command implementation
│   │   ├── search.go         # search command implementation
│   │   └── update.go         # update command implementation
│   └── metadata/
│       ├── tags.go           # .ruin/tags.yml management
│       └── queries.go        # .ruin/queries.yml management
├── go.mod
├── go.sum
├── CLAUDE.md
└── plan.md
```

## Key Dependencies
- `github.com/spf13/cobra` - CLI framework
- `gopkg.in/yaml.v3` - YAML parsing for frontmatter and metadata files
- `github.com/google/uuid` - UUID generation

## Architecture Notes

### Config File Location
- `~/.config/ruin` - YAML file containing:
  - `vault_path`: path to notes directory
  - Other user preferences

### Vault Structure
```
<vault_path>/
├── .ruin/
│   ├── tags.yml      # All tags index
│   └── queries.yml   # Saved search queries
├── Note Title 1.md
├── Note Title 2.md
└── 2025-01-28T10-30-00.md  # Timestamp-named note
```

### Tag Syntax
- Simple: `#foo`, `#bar`, `#2025/may`
- With spaces: `#daily note#` (surrounded by `#`)

### Frontmatter Fields (managed by CLI)
- `uuid`: unique identifier
- `created`: creation timestamp
- `updated`: last modified timestamp
- `tags`: all tags found in document
- `inline-tags`: tags found within content body (not at top/end)

## Common Commands

### Lint
```bash
golangci-lint run
```

### Format
```bash
go fmt ./...
```

## Implementation Status

See `IMPLEMENTATION_PLAN.md` for the detailed implementation plan and progress checklist.

**Important**: When completing implementation work, update the progress checklist at the top of `IMPLEMENTATION_PLAN.md` by marking completed items with `[x]`.
