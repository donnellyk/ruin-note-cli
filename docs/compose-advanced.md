# Compose: Advanced Features

This document covers embed expansion, dynamic embeds, and explain mode for `ruin compose`.

## Embed Expansion

The `--expand-embeds` flag expands `![[Note Title]]` syntax inline, replacing the embed line with the referenced note's content. It also enables dynamic embeds.

### Syntax

Embeds are only recognized on standalone lines (not inline within prose):

```markdown
Some introduction text.

![[Architecture]]

Continuing after the embed.

![[Roadmap#Q1 Goals]]
```

Two forms are supported:

| Syntax | Behavior |
|--------|----------|
| `![[Note Title]]` | Embed the full content of the referenced note |
| `![[Note Title#Header]]` | Embed only the section under the specified header |

### Dynamic Embeds

Dynamic embeds run queries at compose time and inline the results. They use `![[type: query | options]]` syntax on standalone lines:

```markdown
# Weekly Review

## Recent Daily Notes
![[search: #daily | format=list, limit=7, sort=created:desc]]

## Open Follow-ups
![[pick: #followup | format=grouped]]

## Saved Query Results
![[query: my-saved-query | format=summary]]

## Sub-Project
![[compose: "Project Alpha"]]
```

| Type | Description |
|------|-------------|
| `search` | Run a search query and inline results |
| `pick` | Extract tagged lines from notes |
| `query` | Run a saved query (resolves to search) |
| `compose` | Inline a full sub-compose tree |

#### Common Options

| Option | Applies to | Description |
|--------|-----------|-------------|
| `format` | search, pick | Output format. Search: `content` (default), `list`, `summary`. Pick: `grouped` (default), `flat`. |
| `sort` | search, pick | Sort order (e.g., `created:desc`, `title:asc`) |
| `limit` | search, pick | Maximum results |
| `filter` | pick | Note-level filter using search query syntax |
| `group` | pick | Grouping key for `grouped` format: `note` (default), `parent`, `root`, `tag` |
| `empty` | search, pick | Set to `hide` to suppress "No results" message |
| `depth` | compose | Max depth for sub-compose (e.g., `depth=2`) |
| `tag-scope` | search | Tag matching scope: `global`, `inline`, or omit for all |

The compose root note and the parent note containing the dynamic embed are excluded from search/pick results to avoid self-referential output.

#### Search Formats

`![[search: #daily | format=content]]` (default) — full note content, heading-adjusted:
```markdown
## Morning thoughts
Had an interesting idea...

## Meeting notes
Discussed the Q2 roadmap...
```

`![[search: #daily | format=list]]` — wiki-link bullet list:
```markdown
- [[Morning thoughts]]
- [[Meeting notes]]
```

`![[search: #daily | format=summary]]` — title, date, first content line:
```markdown
### Morning thoughts
*2026-04-15*
Had an interesting idea...

### Meeting notes
*2026-04-15*
Discussed the Q2 roadmap...
```

#### Pick Formats

`![[pick: #followup]]` (default `grouped`) — lines under source note headings:
```markdown
### Morning thoughts
- [ ] Follow up with Alex #followup
- [x] Check benchmark results #followup

### Project Alpha
- Review the draft spec #followup
```

`![[pick: #followup | format=flat]]` — flat list with note attribution:
```markdown
- [ ] Follow up with Alex #followup (Morning thoughts)
- [x] Check benchmark results #followup (Morning thoughts)
- Review the draft spec #followup (Project Alpha)
```

#### Pick Grouping

The `group` option controls how lines are organized under headings in `grouped` format:

| Value | Grouping | Heading |
|-------|----------|---------|
| `note` | Source note (default) | Note title |
| `parent` | Immediate parent note | Parent title (orphans use their own title) |
| `root` | Root ancestor | Top-level parent title |
| `tag` | Matching tag | Tag name |

`![[pick: #followup | group=parent]]` — lines from sibling notes merged under their shared parent:
```markdown
### Project Alpha
- Fix the build #followup
- Update docs #followup

### Orphan Note
- Standalone item #followup
```

`![[pick: #todo #followup | any, group=tag]]` — lines grouped by which tag matched (lines with both tags appear under each):
```markdown
### #todo
- Buy milk #todo
- Fix bug #todo #followup

### #followup
- Call Alex #followup
- Fix bug #todo #followup
```

Dynamic embeds that fail to resolve are left unexpanded in the output with a stderr warning.

Header matching is case-insensitive. A section includes the matched heading and all content up to the next heading of equal or higher level.

### Usage

```bash
ruin compose "Project Hub" --expand-embeds
ruin compose "Project Hub" --expand-embeds --normalize-headers
```

Without `--expand-embeds`, `![[...]]` lines pass through unchanged in the output.

### Heading Adjustment

Embedded content follows the same heading adjustment rules as regular children. An embed in a root note (depth 0) renders at depth 1: its H1 becomes H2 with `--normalize-headers`, or gains one `#` level with the default heading adjustment.

### Recursive Expansion

Embedded notes get their own children expanded beneath them, following the same rules as the normal compose walk. An embed behaves as if the note were a child at that position in the document.

If an embedded note itself contains `![[...]]` embeds, those are expanded recursively (subject to the `--depth` limit).

### Deduplication

When a note is both embedded via `![[...]]` and listed as a frontmatter child of the same parent, it appears only once, at the embed position. The child entry is suppressed.

### Example

Given these notes:

**Project Hub.md:**
```markdown
# Project Hub

Overview of the project.

![[Architecture]]

Next steps are outlined below.

![[Roadmap#Q1 Goals]]
```

**Architecture.md** (child of Project Hub, has child "Architecture Details"):
```markdown
# Architecture

The system uses a microservice architecture.
```

**Roadmap.md:**
```markdown
# Roadmap

## Q1 Goals

- Launch beta
- Hire 3 engineers

## Q2 Goals

- Scale to 1000 users
```

Running `ruin compose "Project Hub" --expand-embeds --normalize-headers` produces:

```markdown
# Project Hub

Overview of the project.

## Architecture

The system uses a microservice architecture.

### Architecture Details

Service mesh configuration and deployment.

Next steps are outlined below.

## Q1 Goals

- Launch beta
- Hire 3 engineers
```

What happened:
1. Root note content starts at depth 0
2. `![[Architecture]]` expands Architecture at depth 1 (H1 -> H2)
3. Architecture's child "Architecture Details" renders at depth 2 (H1 -> H3)
4. Architecture is already visited, so it does not appear again in the children section
5. Root note content continues ("Next steps...")
6. `![[Roadmap#Q1 Goals]]` extracts only the Q1 section from Roadmap at depth 1
7. Q2 Goals is excluded because only the Q1 section was requested

### Edge Cases

**Circular references.** If note A embeds note B which embeds note A, the second visit is skipped with a stderr warning. The `visited` map prevents infinite loops.

**Missing notes.** If `![[Missing Note]]` cannot be resolved, a stderr warning is printed and the `![[Missing Note]]` line is left as-is in the output.

**Header not found.** If `![[Note#Nonexistent]]` references a header that does not exist, a stderr warning is printed and the embed line is left unexpanded.

**Depth limit.** The `--depth` flag applies to the total tree depth including embed-introduced levels. If max depth is reached, remaining embeds are left unexpanded.

## Explain Mode

The `--explain` flag prints a decision log showing how compose resolves the note tree, without outputting note content.

### Usage

```bash
ruin compose "Project Hub" --explain
ruin compose "Project Hub" --explain --expand-embeds
ruin compose "Project Hub" --explain --json
```

### Text Output

```
ROOT: "Project Hub" (uuid: abc-123)
  EMBED: "Architecture" (uuid: def-456) -- from ![[...]] in "Project Hub", depth 1
    CHILD: "Architecture Details" (uuid: mno-345) -- child of "Architecture", depth 2
  CHILD: "Roadmap" (uuid: ghi-789) -- child of "Project Hub", depth 1
```

### JSON Output

With `--json`, explain mode outputs a structured decisions array:

```json
{
  "decisions": [
    {
      "type": "root",
      "note": "Project Hub",
      "uuid": "abc-123",
      "path": "Project Hub.md"
    },
    {
      "type": "embed",
      "note": "Architecture",
      "uuid": "def-456",
      "depth": 1,
      "source_note": "Project Hub"
    },
    {
      "type": "child",
      "note": "Architecture Details",
      "uuid": "mno-345",
      "depth": 2,
      "source_note": "Architecture"
    }
  ]
}
```

Decision types:

| Type | Description |
|------|-------------|
| `root` | The root note of the composition |
| `child` | A note included as a child of another note |
| `embed` | A note expanded from a `![[...]]` embed |
| `dynamic` | A dynamic embed result (`![[search: ...]]`, `![[pick: ...]]`, etc.) |

Dynamic decisions include an additional `dynamic` field with type, query, options, and result count.

### Flag Interactions

`--explain` is mutually exclusive with `--edit` and `--content`. It works with all other compose flags: `--expand-embeds`, `--depth`, `--sort`, `--strip-title`, `--strip-global-tags`, `--normalize-headers`, and `--json`.

## JSON Output Changes

The JSON output from `ruin compose --json` includes a new field on nodes that were inlined via embed expansion:

```json
{
  "uuid": "def-456",
  "title": "Architecture",
  "path": "Architecture.md",
  "embedded": true,
  "children": []
}
```

The `embedded` field is `true` for nodes that were expanded from `![[...]]` syntax. It is omitted (not `false`) for regular children.

Nodes produced by dynamic embeds also include a `dynamic` field:

```json
{
  "embedded": true,
  "dynamic": {
    "type": "search",
    "query": "#daily",
    "options": {"format": "list", "limit": "5"},
    "result_count": 3
  },
  "children": [...]
}
```

## Source Map with Embeds

When `--expand-embeds` is active, a single note's content may be split across non-contiguous line ranges (content before an embed, content after an embed). The source map reflects this with multiple entries for the same UUID:

```json
{
  "source_map": [
    {"uuid": "abc-123", "title": "Project Hub", "start_line": 1, "end_line": 3},
    {"uuid": "def-456", "title": "Architecture", "start_line": 5, "end_line": 7},
    {"uuid": "abc-123", "title": "Project Hub", "start_line": 9, "end_line": 10}
  ]
}
```

The source map remains a flat list of `{uuid, path, title, start_line, end_line}` entries. The only change is that a UUID may appear multiple times.

## Flag Reference

| Flag | Short | Description |
|------|-------|-------------|
| `--expand-embeds` | | Expand `![[note]]` embeds inline |
| `--explain` | | Print a decision log instead of content |
| `--depth` | | Max recursion depth (0 = unlimited) |
| `--strip-title` | | Remove H1 title from root note |
| `--strip-global-tags` | | Remove global tag lines |
| `--normalize-headers` | | Normalize child headings by depth |
| `--sort` | `-s` | Child ordering: `field[:dir]` (e.g., `created:desc`) |
| `--edit` | `-e` | Open tree notes in `$EDITOR` |
| `--force` | `-f` | Skip confirmation in edit mode |
| `--content` | | Include per-node content in JSON output |
| `--json` | | Output as JSON (global flag) |
