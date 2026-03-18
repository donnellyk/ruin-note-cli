# Link Notes

Link notes are notes whose primary content is a web URL -- bookmarks with annotations.

## What Makes a Note a Link Note

A note is a "link note" if either:

1. Its frontmatter has a `url` field (takes precedence), OR
2. Its first non-empty, non-title, non-tag-only line is a bare URL (http:// or https://)

## Managed Frontmatter Field: `url`

Link notes have a `url` field in frontmatter:

```yaml
---
uuid: abc-123
url: https://example.com/article
tags:
  - "#link"
---
```

The save pipeline automatically promotes body URLs to the `url` frontmatter field, so all link notes end up with a consistent `url` field regardless of how they were created.

## Auto-Tagging

The save pipeline automatically adds `#link` to any detected link note. This happens in `EnsureLinkTag()`, which:

1. Detects if the note is a URL note
2. Promotes the body URL to the `url` frontmatter field if not already set
3. Adds `#link` to the first tag-only line (or inserts a new tag line after the title)

This runs in all write paths: `log`, `link new`, `search --edit`, `update`, and `doctor`.

## Commands

### `ruin link new <url>`

Create a link note from a URL. Fetches the page title and description automatically.

```bash
ruin link new https://go.dev/blog/go1.22
ruin link new https://example.com --title "My Title" --no-fetch
ruin link new https://example.com --tags "code,reference" --comment "Great article"
ruin link new https://example.com --parent alpha
```

| Flag | Short | Description |
|------|-------|-------------|
| `--title` | `-t` | Override the resolved page title |
| `--tags` | | Additional tags (comma-separated, `#` auto-added) |
| `--parent` | | Set parent note |
| `--order` | | Set manual sort order |
| `--no-fetch` | | Skip URL title/description resolution |
| `--comment` | `-c` | Add personal commentary below the URL |

JSON output includes `resolved_title` and `resolved_summary` fields.

Warns on stderr if a duplicate URL already exists in the vault (note is still created).

### `ruin link resolve <url>`

Fetch and display URL metadata without creating a note.

```bash
ruin link resolve https://example.com
ruin link resolve https://example.com --json
```

### `ruin link list`

List all link notes (notes with a URL). Equivalent to `ruin search --link`.

Supports all standard search flags (`--sort`, `--limit`, `--edit`, `--bulk`, `--first`, `--json`, etc.). Default sort: `created:desc`.

## Search Filter: `link:`

Search notes by URL using the `link:` prefix:

```bash
ruin search "link:go.dev"
ruin search "link:example.com #link"
ruin search "link:github.com/acme"
```

Case-insensitive substring match against the frontmatter `url` field. This is a frontmatter-only operation (no body read required).

## `ruin log` URL Auto-Detection

When `ruin log` receives content that starts with a bare URL and has no title:

```bash
ruin log "https://go.dev/blog/go1.22"
```

It automatically:
1. Resolves the URL to get the page title
2. Prepends `# ResolvedTitle` to the content
3. The save pipeline adds `#link` and promotes the URL to frontmatter

Use `--no-fetch` to skip auto-resolution:

```bash
ruin log --no-fetch "https://example.com"
```

## URL Resolution

The `HTMLResolver` fetches pages via HTTP and extracts:
- `<title>` tag content
- `<meta name="description">` or `<meta property="og:description">` content

Constraints:
- 10 second timeout
- 1MB body size limit
- Title truncated to 200 characters, description to 500
- Common HTML entities decoded (`&amp;`, `&lt;`, etc.)
