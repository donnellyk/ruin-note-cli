# Ruin Note CLI

A Zettelkasten-inspired note-taking CLI written in Go.

## Important
- This is a CLI with downstream dependencies. Avoid breaking changes unless necessary. Highlight breaking changes.
- When running `go test` directly, always set `EDITOR=true` to prevent editor popups (or use `mise run test` which sets it automatically).

## Project Setup

### Prerequisites
- Go 1.21+ (install via `brew install go` on macOS or from https://go.dev/dl/)
- Set up Go environment: ensure `$GOPATH/bin` is in your `$PATH`

### Build & Run
```bash
mise run build    # or: go build -o ruin ./cmd/ruin
./ruin --help
```

### Run Tests
```bash
mise run test                # Preferred: sets EDITOR=true via mise.toml [env]
EDITOR=true go test ./...    # If running go test directly
```

### Install Locally
```bash
mise run install  # or: go install ./cmd/ruin
```

### All Tasks
```bash
mise tasks     # Show all available tasks
```

## Project Structure
```
ruin-note-cli/
├── cmd/
│   └── ruin/
│       └── main.go           # CLI entry point
├── internal/
│   ├── config/
│   │   └── config.go         # Config file handling (~/.config/ruin/config.yml)
│   ├── vault/
│   │   ├── vault.go          # Vault operations + tags/queries index
│   │   └── titles.go         # Titles index (JSON, UUID->title/path/parent)
│   ├── note/
│   │   ├── note.go           # Note struct and operations
│   │   ├── frontmatter.go    # YAML frontmatter parsing/writing
│   │   ├── bulk.go           # Bulk export/import format
│   │   ├── tags.go           # Tag extraction (wrappers delegating to pkg/notetext)
│   │   ├── links.go          # Wiki link extraction ([[title]])
│   │   └── url.go            # URL note detection, extraction, auto-tagging
│   ├── urlresolve/
│   │   ├── resolver.go       # Resolver interface, URLMetadata struct
│   │   └── html.go           # HTMLResolver (HTTP fetch + HTML metadata extraction)
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
│       ├── pick.go           # pick command (inline tag line extraction)
│       ├── link.go           # link command (new, resolve, list)
│       ├── resolve.go        # Note resolution (UUID, title, path lookup)
│       └── links.go          # Wiki link resolution (RefreshLinkedCards)
├── pkg/
│   └── notetext/
│       ├── tags.go           # Tag extraction (public, importable by external tools)
│       └── ranges.go         # Embed/code/link range detection
├── scripts/
│   └── create-benchmark-vault.sh  # Benchmark vault generator
├── go.mod
├── go.sum
├── mise.toml
└── CLAUDE.md
```

## Key Dependencies
- `github.com/spf13/cobra` - CLI framework
- `gopkg.in/yaml.v3` - YAML parsing for frontmatter and metadata files
- `github.com/google/uuid` - UUID generation

## Architecture Notes

### Config File Location
- `~/.config/ruin/config.yml` - YAML file containing (legacy: `~/.config/ruin` as a file is also supported):
  - `vault_path`: path to notes directory
  - Other user preferences

### Vault Structure
```
<vault_path>/
├── .ruin/
│   ├── tags.yml      # All tags index (name, count, scope)
│   ├── queries.yml   # Saved search queries
│   └── titles.json   # Titles index (UUID to title/path/parent)
├── Note Title 1.md
├── Note Title 2.md
└── 2025-01-28T10-30-00.md  # Timestamp-named note
```

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
Use the `dev seed` command to create a test vault for manual testing (build first with `mise run build`):
```bash
./ruin dev seed                          # Create at /tmp/ruin-test-vault
./ruin dev seed ~/my-vault               # Create at custom path
./ruin dev seed --clean                  # Remove test vault
./ruin dev seed --reset                  # Clean and recreate
```

Then test commands against it:
```bash
./ruin --vault /tmp/ruin-test-vault log
./ruin --vault /tmp/ruin-test-vault search "#daily"

# When testing --edit flag, use EDITOR=cat to see output without opening an editor
EDITOR=cat ./ruin --vault /tmp/ruin-test-vault search "#daily" --edit
```
