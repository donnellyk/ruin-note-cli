# Compose: Advanced Features

This document covers three additions to `ruin compose`: YML-based composition files, inline embed expansion, and explain mode.

## YML-Based Composition

By default, `ruin compose` walks the parent-child tree stored in note frontmatter. YML composition files let you define a note tree independently, without modifying any note's `parent` field.

### File Format

```yaml
root: "Project Alpha Hub"
children:
  - note: "Introduction"
  - note: "Architecture"
    children:
      - note: "Backend Design"
      - note: "Frontend Design"
  - note: "Roadmap"
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `root` | string | Yes | Root note reference (title, UUID, or path) |
| `children` | list | No | Ordered list of child entries |
| `children[].note` | string | Yes | Child note reference (title, UUID, or path) |
| `children[].children` | list | No | Nested children (recursive) |

Note references use the same resolution as elsewhere in the CLI: titles, UUIDs, and vault-relative paths all work.

### Usage

```bash
ruin compose --file project.yml
ruin compose -F project.yml --strip-title --strip-global-tags
ruin compose -F project.yml --json --content
```

When `--file` is provided, no positional `<note>` argument is needed. Providing both is an error.

### Ordering

Children listed in the YML file retain their declared order. The `--sort` flag only applies to children sourced from frontmatter fallback (see below).

### Hybrid Mode: YML + Frontmatter Children

When a node in the YML file has no `children` key, its frontmatter-based children (from `titles.json`) are used automatically. This lets you control top-level structure via YML while subtrees use their existing parent relationships.

```yaml
root: "Project Alpha Hub"
children:
  - note: "Introduction"
  - note: "Architecture"    # Its frontmatter children appear beneath it
  - note: "Roadmap"
```

If "Architecture" has frontmatter children "Backend Design" and "Frontend Design", they are included in the compose output beneath "Architecture" and sorted by the `--sort` flag.

### Saving YML Files as Bookmarks

You can save a YML composition file as a named bookmark with `parent save`:

```bash
ruin parent save alpha --file project.yml
```

Then compose using just the bookmark name:

```bash
ruin compose alpha
```

This is equivalent to `ruin compose --file project.yml`. The file path is stored relative to the vault root when possible, or as an absolute path otherwise.

Listing bookmarks shows file-based entries:

```bash
ruin parent list
# alpha: project.yml (file)
# docs: d4e5f6a7-... "Documentation Root"
```

File-based bookmarks only work with `ruin compose`. Other commands that resolve parent bookmarks (e.g., `ruin parent children`, `ruin parent tree`) return an error directing you to use `ruin compose` instead.

### Unresolvable Notes

If a note reference in the YML file cannot be resolved, a warning is printed to stderr and that entry is skipped. Composition continues with the remaining notes.

## Embed Expansion

The `--expand-embeds` flag expands `![[Note Title]]` syntax inline, replacing the embed line with the referenced note's content.

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

Header matching is case-insensitive. A section includes the matched heading and all content up to the next heading of equal or higher level.

### Usage

```bash
ruin compose "Project Hub" --expand-embeds
ruin compose "Project Hub" --expand-embeds --normalize-headers
ruin compose -F project.yml --expand-embeds
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
ruin compose -F project.yml --explain
ruin compose "Project Hub" --explain --json
```

### Text Output

```
ROOT: "Project Hub" (uuid: abc-123)
  SOURCE: children of "Project Hub" from compose file
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
| `yml_source` | Children for this node came from the YML composition file |

### Flag Interactions

`--explain` is mutually exclusive with `--edit` and `--content`. It works with all other compose flags: `--file`, `--expand-embeds`, `--depth`, `--sort`, `--strip-title`, `--strip-global-tags`, `--normalize-headers`, and `--json`.

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
| `--file` | `-F` | Path to a YML composition file |
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
