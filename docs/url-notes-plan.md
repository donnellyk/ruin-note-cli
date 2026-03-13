# URL Notes Plan

## Overview and Motivation

Notes whose primary content is a web URL are a common pattern in Zettelkasten systems -- bookmarks with annotations. Currently, ruin has no first-class support for these: users must manually create notes, manually tag them, and have no way to resolve page titles or search by URL.

This plan adds:

- **URL detection and auto-tagging**: Recognize link notes and auto-tag them with `#link` in the save pipeline
- **`ruin link` command tree**: Create, resolve, and list link notes
- **`link:` search filter**: Search notes by URL

## Link Note Detection

A note is a "link note" if either:

1. Its frontmatter has a `url` field (takes precedence), OR
2. Its first non-empty, non-title line is a bare URL (http:// or https://)

The `url` frontmatter field is a new managed field. When a link note is detected via body URL, the save pipeline automatically promotes the URL to the `url` frontmatter field. This ensures all link notes have a consistent `url` field regardless of how they were created (e.g., `ruin log "https://example.com"` or `ruin link new https://example.com`).

### New `url` Field

Add `URL string` to both `Frontmatter` and `Note` structs:

```go
// In Frontmatter struct
URL string `yaml:"url,omitempty"`

// In Note struct
URL string
```

Propagate in `Parse()`, `Serialize()`, and the fast parser (`ParseFrontmatterFast` / `LoadFrontmatterOnly`). The fast parser needs a `case "url"` branch in `parseFMLines`, and `LoadFrontmatterOnly` must copy `fm.URL` to `note.URL`.

### Core Functions (`internal/note/url.go`)

```go
// IsURLNote returns true if the note is a link note.
func (n *Note) IsURLNote() bool

// ExtractURL returns the URL from a link note.
// Checks frontmatter url field first, then body.
func (n *Note) ExtractURL() string

// EnsureLinkTag adds #link to the note body and promotes the body URL
// to the frontmatter url field if not already set.
// Returns true if the note was modified (caller should RefreshTags).
func (n *Note) EnsureLinkTag() bool

// IsValidURL checks if a string is a valid http/https URL.
func IsValidURL(s string) bool
```

**`EnsureLinkTag` algorithm:**

1. If note is not a URL note, return false
2. If `n.URL` is empty but a body URL is detected, set `n.URL` to the extracted URL
3. If `#link` already present in tags or content, return (may still return true if URL was promoted)
4. Find the first tag-only line in content and append `#link` using the same separator (comma+space or space)
5. If no tag-only line exists, insert `\n#link\n` after the title header (or at the end if no title)
6. Return true

### Save Pipeline Integration

`EnsureLinkTag` is called in the save pipeline after `RefreshTags()`. Every write path must include this:

```go
n.RefreshTags()
if n.EnsureLinkTag() {
    n.RefreshTags()  // Re-extract since we modified content
}
```

Write paths to update:

| File | Location |
|------|----------|
| `notecmd_save.go` | `saveWithIndexUpdate()` |
| `log.go` | `NewLogCmd` RunE |
| `search_edit.go` | `handleEditSingle()` |
| `search_edit.go` | `applyBulkChanges()` |
| `update.go` | modification loop |
| `doctor.go` | `doctorFiles()` and `doctorFullScan()` |

The `if` guard is important -- always call `RefreshTags()` after `EnsureLinkTag()` returns true, because the content was modified.

## URL Metadata Resolution (`internal/urlresolve/`)

### Package Structure

| File | Purpose |
|------|---------|
| `resolver.go` | `Resolver` interface, `URLMetadata` struct |
| `html.go` | `HTMLResolver` -- HTTP fetch + HTML title/description extraction |

### Types

```go
type URLMetadata struct {
    URL         string `json:"url"`
    Title       string `json:"title,omitempty"`
    Summary     string `json:"summary,omitempty"`
    ResolvedVia string `json:"resolved_via"`
}

type Resolver interface {
    Resolve(ctx context.Context, url string) (*URLMetadata, error)
}
```

### HTMLResolver

Fetches the page via HTTP and extracts metadata from HTML using stdlib string operations (no external HTML parsing dependency).

Key constraints:
- 10 second timeout
- 1MB body size limit via `io.LimitReader`
- `ruin-note-cli` User-Agent header
- Title truncated to 200 runes, description to 500 runes (use `[]rune` slicing, not byte slicing, to avoid splitting multi-byte UTF-8 characters)
- Extracts `<title>` tag content
- Extracts `<meta name="description">` or `<meta property="og:description">` content attribute
- Decodes common HTML entities (`&amp;`, `&lt;`, `&gt;`, `&quot;`, `&#39;`, `&apos;`, `&#x27;`, `&nbsp;`)
- Case-insensitive HTML tag matching (lowercase the HTML for searching, but extract content from original to preserve case)

## `ruin link` Command Tree (`internal/commands/link.go`)

Register in `cmd/ruin/main.go`:

```go
rootCmd.AddCommand(commands.NewLinkCmd(getVault, &jsonOut))
```

### `link new <url>`

Create a link note from a URL.

| Flag | Short | Description |
|------|-------|-------------|
| `--title` | `-t` | Override the resolved page title |
| `--tags` | | Additional tags (comma-separated, `#` auto-added) |
| `--parent` | | Set parent note |
| `--order` | | Set manual sort order |
| `--no-fetch` | | Skip URL title/description resolution |
| `--comment` | `-c` | Add personal commentary below the URL |

**Behavior:**

1. Validate URL (must be http:// or https://)
2. Unless `--no-fetch`, resolve metadata via `HTMLResolver`
3. Sanitize resolved title (strip newlines, collapse whitespace)
4. Build note content: `# Title\n`, optional comment, optional summary, `\n#link` + extra tags
5. Parse content via `note.Parse()`
6. Set `n.URL = rawURL`
7. Create and save note via shared `createNote` helper (see below)
8. Warn on duplicate URLs (scan vault via `LoadFrontmatterOnly` checking `n.URL`)

**Shared `createNote` helper:** Extract a common "create note" function used by both `link new` and `log`. This helper handles: `EnsureUUID`, `SetTimestamps`, save pipeline (`RefreshTags`, `EnsureLinkTag`, `ResolveDateTokens`, `RefreshDates`, `RefreshLinkedCards`, `RefreshInheritedTags`), filename generation, file write, and index creation. Command-specific setup (content building, URL resolution, title extraction) happens before calling the helper.

**Title sanitization** (`sanitizeTitle`): Resolved HTML titles may contain newlines which would corrupt the `# ` header line. Strip `\n` and `\r`, collapse runs of whitespace via `strings.Fields` + `strings.Join`.

**JSON output:**

```json
{
  "path": "...",
  "uuid": "...",
  "title": "...",
  "url": "https://...",
  "resolved_title": "...",
  "resolved_summary": "..."
}
```

### `link resolve <url>`

Fetch and display URL metadata without creating a note.

```
ruin link resolve https://example.com
ruin link resolve https://example.com --json
```

Human-readable output: `Title: ...`, `URL: ...`, `Summary: ...`

**JSON output:**

```json
{
  "url": "https://...",
  "title": "...",
  "summary": "...",
  "resolved_via": "html"
}
```

### `link list`

Convenience alias for `ruin search "#link"`. Uses the full `SearchFlags` (sort, limit, edit, bulk, first, content, frontmatter, etc.). Default sort: `created:desc`.

Delegates to `searchNotesWithOptions` + the same output dispatch logic as search (JSON, bulk, first, edit, paths).

## `link:` Search Filter

Add a `link:` prefix to the search query parser in `parseTermMatcher`:

```go
case "link":
    return linkMatcher(value), fmOnly, nil
```

The `linkMatcher` function (in `search_matchers.go`) checks the frontmatter `URL` field for a case-insensitive substring match. Since all link notes have `url` in frontmatter (promoted by the save pipeline), this is a frontmatter-only operation (`needsBody` is false).

**Usage:**

```bash
ruin search "link:go.dev"
ruin search "link:example.com #link"
```

## `log` Command: URL Auto-Detection

When `ruin log` receives content that starts with a bare URL and has no title:

1. Detect via `n.IsURLNote()` (only when `title == ""`, `--h1` not set, `n.Title == ""`)
2. Unless `--no-fetch`, resolve the URL via `HTMLResolver`
3. If a title is resolved, sanitize it and prepend `# ResolvedTitle\n\n` to content
4. Continue with normal save pipeline (which includes `EnsureLinkTag`, promoting the URL to frontmatter)

Add `--no-fetch` flag to log command to skip this auto-resolution.

This means `ruin log "https://go.dev/blog/go1.22"` auto-creates a titled, `#link`-tagged link note with the URL in frontmatter.

Both `log` and `link new` use the shared `createNote` helper for the save-and-write portion.

## Doctor Integration

Add `n.EnsureLinkTag()` + conditional `n.RefreshTags()` to both `doctorFiles` and `doctorFullScan`, before tag comparison. This ensures existing link notes in the vault get the `#link` tag and `url` frontmatter field on the next `ruin doctor` run.

## Seed Data (`dev seed`)

Add 5 link note examples to the seed vault covering different patterns:

- Link note with title and tags
- Link note (minimal, URL only)
- Link note with annotation
- Link note with comment
- Link note with parent relationship

Update the seed summary counts to include the new notes.

## File Layout

### New files

| File | Purpose |
|------|---------|
| `internal/note/url.go` | `IsURLNote`, `ExtractURL`, `EnsureLinkTag`, `IsValidURL` |
| `internal/note/url_test.go` | URL detection and auto-tagging tests |
| `internal/urlresolve/resolver.go` | `Resolver` interface, `URLMetadata` |
| `internal/urlresolve/html.go` | `HTMLResolver` with HTML title/description extraction |
| `internal/urlresolve/html_test.go` | Tests using `httptest.NewServer` |
| `internal/commands/link.go` | `NewLinkCmd` with `new`, `resolve`, `list` subcommands |
| `internal/commands/link_test.go` | Link command tests |

### Modified files

| File | Change |
|------|--------|
| `internal/note/frontmatter.go` | Add `URL` field to `Frontmatter`, update `parseFrontmatterYAML`, `Serialize`, `IsEmpty`, `Merge` |
| `internal/note/note.go` | Add `URL` field to `Note`, update `Parse()`, `Serialize()` |
| `internal/note/fastparse.go` | Add `case "url"` to fast parser, copy `URL` in `LoadFrontmatterOnly` |
| `internal/commands/log.go` | Add `--no-fetch` flag, URL auto-detection, refactor to use shared `createNote` helper |
| `internal/commands/notecmd_save.go` | Add `EnsureLinkTag` call in `saveWithIndexUpdate`, add shared `createNote` helper |
| `internal/commands/search_edit.go` | Add `EnsureLinkTag` to `handleEditSingle` and `applyBulkChanges` |
| `internal/commands/update.go` | Add `EnsureLinkTag` to modification loop |
| `internal/commands/doctor.go` | Add `EnsureLinkTag` to both doctor paths |
| `internal/commands/search_query.go` | Add `case "link"` filter prefix |
| `internal/commands/search_matchers.go` | Add `linkMatcher` function |
| `internal/commands/dev_seed.go` | Add link note seed examples |
| `cmd/ruin/main.go` | Register `link` command |

## Implementation Sequence

### Phase 1: URL Detection and Auto-Tagging

1. Add `URL` field to `Frontmatter` and `Note` structs
2. Update `parseFrontmatterYAML`, `Serialize`, `IsEmpty`, `Merge` for URL field
3. Update fast parser (`parseFMLines` and `LoadFrontmatterOnly`)
4. Implement `internal/note/url.go`: `IsURLNote`, `ExtractURL`, `EnsureLinkTag` (with URL promotion), `IsValidURL`
5. Write tests in `internal/note/url_test.go`
6. Add `EnsureLinkTag` to all save pipeline paths (notecmd_save, log, search_edit, update, doctor)

### Phase 2: URL Resolution Package

1. Create `internal/urlresolve/resolver.go` with interface and types
2. Implement `HTMLResolver` in `html.go`
3. Write tests using `httptest.NewServer`

### Phase 3: `ruin link` Command Tree and Shared Helper

1. Extract shared `createNote` helper from `log` command
2. Refactor `log` to use `createNote`
3. Implement `NewLinkCmd` with `new`, `resolve`, `list` subcommands (using `createNote`)
4. Add `sanitizeTitle` helper
5. Add `warnDuplicateURL` helper
6. Register in `main.go`
7. Write tests

### Phase 4: Search Integration

1. Add `linkMatcher` to `search_matchers.go`
2. Add `case "link"` to `parseTermMatcher` in `search_query.go`
3. Add `--no-fetch` flag to `log` command with URL auto-detection

### Phase 5: Seed Data and Documentation

1. Add link note examples to `dev_seed.go`
2. Update `CLAUDE.md` project structure
3. Write a `docs/link-notes.md` document describing the link notes feature for downstream consumers (commands, flags, search filter, auto-tagging behavior, URL frontmatter field, save pipeline changes)

## Edge Cases

### Duplicate URL detection
`link new` scans the vault via `LoadFrontmatterOnly` for matching `URL` fields. Since all link notes have `url` in frontmatter (promoted by the save pipeline), this catches all duplicates. A stderr warning is emitted but the note is still created (duplicates are allowed, as users may intentionally want multiple notes about the same URL).

### Auto-title resolution failure
If `HTMLResolver` fails (network error, non-HTML content, timeout), the note is still created with no title. A stderr warning is emitted.

### EnsureLinkTag idempotency
`EnsureLinkTag` checks for existing `#link` and existing `url` frontmatter before modifying. Multiple calls are safe. The `if n.EnsureLinkTag() { n.RefreshTags() }` pattern avoids unnecessary tag re-extraction.

## Changelog

- 2026-03-13: Renamed search filter from `url:` to `link:` for consistency with `ruin link` command and `#link` tag
- 2026-03-13: Removed body-URL concept; all link notes get `url` promoted to frontmatter by the save pipeline
- 2026-03-13: Removed `--body-url` flag from `link new`
- 2026-03-13: Added shared `createNote` helper to eliminate duplication between `log` and `link new`
- 2026-03-13: Removed Claude API stub, ChainResolver, ErrNotConfigured, and ClaudeConfig (will be specced when needed)
- 2026-03-13: `link:` filter uses frontmatter-only search (`needsBody: false`) since all link notes have `url` in frontmatter
