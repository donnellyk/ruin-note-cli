# Ruin Note CLI

A Zettelkasten-inspired note-taking CLI written in Go.

## Important
- This is a CLI with downstream dependencies. Avoid breaking changes unless necessary. Highlight breaking changes.
- Do not edit `Todos` section in "IMPLEMENTATION_PLAN.md`
- When running `go test` directly, always set `EDITOR=true` to prevent editor popups (or use `make test` which does this automatically).
- When running the `ruin` binary for manual testing, always use `--vault /tmp/ruin-test-vault` (create it first with `./scripts/test-vault.sh create` if needed). Never run against the user's real vault.
- Never modify `~/.config/ruin` when testing. Use `--vault` to point at a test vault instead of running `ruin config` or `ruin init` without an explicit path.

## Project Setup

### Prerequisites
- Go 1.21+ (install via `brew install go` on macOS or from https://go.dev/dl/)
- Set up Go environment: ensure `$GOPATH/bin` is in your `$PATH`

### Build & Run
```bash
make build    # or: go build -o ruin ./cmd/ruin
./ruin --help
```

### Run Tests
```bash
make test                    # Preferred: sets EDITOR=true to prevent editor popups
EDITOR=true go test ./...    # If running go test directly
```

### Install Locally
```bash
make install  # or: go install ./cmd/ruin
```

### All Make Targets
```bash
make help     # Show all available targets
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
│   │   ├── vault.go          # Vault operations + tags/queries index
│   │   └── titles.go         # Titles index (JSON, UUID->title/path/parent)
│   ├── note/
│   │   ├── note.go           # Note struct and operations
│   │   ├── frontmatter.go    # YAML frontmatter parsing/writing
│   │   ├── bulk.go           # Bulk export/import format
│   │   ├── tags.go           # Tag extraction logic
│   │   └── links.go          # Wiki link extraction ([[title]])
│   └── commands/
│       ├── log.go            # log command implementation
│       ├── search.go         # search command implementation
│       ├── update.go         # update command implementation
│       ├── query.go          # query command (save, list, delete, run)
│       ├── init.go           # init command implementation
│       ├── config.go         # config command implementation
│       ├── doctor.go         # doctor command implementation
│       ├── parent.go         # parent command (set, get, remove, children, tree)
│       ├── suggest.go        # suggest command (title prefix matching)
│       ├── compose.go        # compose command (recursive document assembly)
│       ├── resolve.go        # Note resolution (UUID, title, path lookup)
│       └── links.go          # Wiki link resolution (RefreshLinkedCards)
├── scripts/
│   └── test-vault.sh         # Test vault helper script
├── go.mod
├── go.sum
├── Makefile
├── CLAUDE.md
└── IMPLEMENTATION_PLAN.md
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
│   ├── queries.yml   # Saved search queries
│   └── titles.json   # Titles index (UUID to title/path/parent)
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
- `parent`: UUID of parent note (optional, omitted if empty)
- `linked-cards`: resolved UUIDs from `[[wiki links]]` (optional, omitted if empty)

## Common Commands

### Lint
```bash
golangci-lint run
```

### Format
```bash
go fmt ./...
```

### Test Vault
Use the helper script to create a test vault for manual testing:
```bash
./scripts/test-vault.sh create           # Create at /tmp/ruin-test-vault
./scripts/test-vault.sh create ~/my-vault # Create at custom path
./scripts/test-vault.sh clean            # Remove test vault
./scripts/test-vault.sh reset            # Clean and recreate
```

Then test commands against it:
```bash
./ruin --vault /tmp/ruin-test-vault log
./ruin --vault /tmp/ruin-test-vault search "#daily"

# When testing --edit flag, use EDITOR=cat to see output without opening an editor
EDITOR=cat ./ruin --vault /tmp/ruin-test-vault search "#daily" --edit
```

## Implementation Status

See `IMPLEMENTATION_PLAN.md` for the detailed implementation plan and progress checklist.

**Important**: When completing implementation work, update the progress checklist at the top of `IMPLEMENTATION_PLAN.md` by marking completed items with `[x]`.
