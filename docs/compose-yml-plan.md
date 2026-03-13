# Compose Enhancement Plan: YML-based Composition and Embed Expansion

## Overview and Motivation

The current `ruin compose` command assembles documents by walking the parent-child tree stored in note frontmatter. This works well for hierarchical structures but has two limitations:

1. **Rigid ordering**: The composition order is determined entirely by the parent-child relationships stored in frontmatter. There is no way to compose arbitrary notes together, or to include a note in multiple compositions without changing its `parent` field.

2. **No inline embedding**: Users cannot control where a child's content appears within the parent's body. Children are always appended after the parent, sorted by the `--sort` field.

This plan introduces two features to address these limitations:

- **YML-based composition files** that declare a note tree without requiring frontmatter parent relationships
- **Embed expansion** (`![[note]]` syntax) that lets users inline note content at a specific location within a parent note, with full recursive child expansion

## Feature 1: YML-based Composition

### YML File Format Specification

A composition file is a YML file (`.yml` or `.yaml`) that declares a tree of notes to compose. Notes are referenced by title, UUID, or path -- the same resolution logic used by `ResolveNote` today.

```yaml
# compose-spec.yml
root: "Project Alpha Hub"    # The root note (required)
children:                     # Ordered list of children (optional)
  - note:"Chapter 1"         # Note reference (title, UUID, or path)
    children:                 # Nested children (optional)
      - note:"Section 1.1"
      - note:"Section 1.2"
  - note:"Chapter 2"
  - note:"d4e5f6a7-..."      # UUID reference
  - note:"notes/appendix.md" # Path reference
```

**Fields:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `root` | string | Yes | Reference to the root note (title, UUID, or path) |
| `children` | list | No | Ordered list of child entries |
| `children[].note` | string | Yes | Reference to a child note |
| `children[].children` | list | No | Recursive child entries |

**Key design decisions:**

- The `children` order in the YML file determines composition order. The `--sort` flag is ignored for YML-specified children (they are already explicitly ordered).
- Notes referenced in the YML file do NOT need `parent` fields in their frontmatter. The YML file defines the tree structure independently.
- A note can appear in multiple YML composition files.
- If `children` is omitted for a node, that node's frontmatter-based children (from `titles.json`) are used as fallback. This allows hybrid usage: YML defines top-level structure, frontmatter defines subtrees.

### How YML Composition Maps to Existing Compose Logic

The YML file is translated into the same `childrenMap map[string][]string` structure that the existing compose functions consume. The implementation:

1. Parse the YML file
2. Resolve each `note` reference to a UUID via `ResolveNote`
3. Build a `childrenMap` from the YML tree structure
4. For any node without explicit YML children, fall back to `index.ChildrenMap()` entries
5. Pass the merged `childrenMap` to the existing `composeTextWithSourceMap` / `composeJSON` functions

This approach reuses all existing heading adjustment, list merging, source map, strip-title, strip-global-tags, and normalize-headers logic with zero duplication.

### CLI Interface

```
ruin compose --file compose-spec.yml [flags]
```

When `--file` is provided, the positional `<note>` argument is not required (the root comes from the YML file). Validation rules:
- `--file` provided + positional arg → error (mutually exclusive)
- Neither `--file` nor positional arg → error ("provide a note or --file")
- Exactly one of the two → proceed

| Flag | Short | Description |
|------|-------|-------------|
| `--file` | `-F` | Path to a YML composition file |

All existing compose flags (`--depth`, `--strip-title`, `--strip-global-tags`, `--sort`, `--edit`, `--force`, `--content`, `--normalize-headers`, `--json`) continue to work. The `--sort` flag only applies to children sourced from frontmatter fallback (not YML-specified order).

**Sort interaction with YML:** `BuildChildrenMapFromSpec` returns a `ymlParents set[string]` identifying parent UUIDs whose children came from the YML file. The sort loop skips these entries, preserving the explicit YML ordering:

```go
for parent := range childrenMap {
    if ymlParents[parent] {
        continue // YML order is explicit
    }
    sortChildUUIDs(vlt, index, childrenMap[parent], sortField)
}
```

### Changes to `ruin parent save` for Named YML-based Parents

Currently `parent save` maps a name to a note UUID. The enhancement adds support for mapping a name to a YML composition file path instead.

```
ruin parent save docs --file ./compose-docs.yml
```

**Updated `ParentEntry` in `parents.yml`:**

```yaml
parents:
  - name: alpha
    uuid: "d4e5f6a7-..."         # Existing: points to a note
  - name: docs
    file: "./compose-docs.yml"    # New: points to a YML composition file
```

**Vault struct change:** Add an optional `File` field to `ParentEntry`:

```go
type ParentEntry struct {
    Name string `yaml:"name" json:"name"`
    UUID string `yaml:"uuid,omitempty" json:"uuid,omitempty"`
    File string `yaml:"file,omitempty" json:"file,omitempty"`
}
```

**Resolution change:** `LookupParent` returns a `ParentBookmark` struct (not a bare UUID string):

```go
type ParentBookmark struct {
    Name string
    UUID string // populated for note-based bookmarks
    File string // populated for file-based bookmarks
}
```

Callers check which field is set:
- **`ruin compose`**: if `File` is set, load the YML composition file. If `UUID` is set, use existing behavior.
- **All other commands** (e.g., `ruin parent children`, `ruin parent tree`): if `File` is set, return a clear error: `bookmark "<name>" is a composition file — use "ruin compose <name>"`. If `UUID` is set, use existing behavior.

**`parent save` args change:** Currently `ExactArgs(2)` for `<name> <note>`. With `--file`, only `<name>` is needed. Update to custom validation: require 2 args normally, require 1 arg + `--file`, error if both `<note>` and `--file` are provided (mutually exclusive).

**File path storage:** At save time, resolve the `--file` path:
- If the file is inside the vault directory, store as a relative path from vault root (portable).
- If the file is outside the vault, store as an absolute path.

At load time, resolve relative paths against vault root, absolute paths as-is. This matches how `vault_path` is handled in config.

## Feature 2: Embed Expansion

### Syntax Detection and Parsing

The embed syntax follows the Obsidian convention:

- `![[Note Title]]` -- embed the full content of the referenced note
- `![[Note Title#Header]]` -- embed only the section under the specified header (and its sub-headers, up to the next header of equal or higher level)

**Regex pattern:**

```go
var embedPattern = regexp.MustCompile(`!\[\[([^\[\]#|]+?)(?:#([^\[\]|]+?))?\]\]`)
```

Group 1: note reference (title, UUID, or path)
Group 2: optional header reference

Embeds are only recognized when the `![[...]]` token appears on its own line (possibly with leading whitespace). Inline `![[...]]` within prose is not expanded -- this avoids accidental expansion of embed-like syntax in running text.

### Content Resolution

**Full note embed** (`![[Note Title]]`):
- Resolve the note via `ResolveNote`
- Use the note's `Content` field (markdown body without frontmatter)

**Header section embed** (`![[Note Title#Header]]`):
- Resolve the note
- Find the first heading matching the header text (case-insensitive)
- Extract from that heading through all content until the next heading of equal or higher level (fewer `#` marks) (or end of file)

```go
func extractSection(content string, header string) (string, error)
```

### Heading Level Adjustment Rules

Embeds follow the same heading adjustment rules as children:

1. Determine the embed's effective depth in the compose tree
2. If `--normalize-headers` is set, use `normalizeHeadings(content, depth)`
3. Otherwise, use `adjustHeadings(content, depth)`

The depth of an embed is the depth of the note it appears in, plus one. For example, if a root note (depth 0) contains `![[Child]]`, the embed renders at depth 1 -- the same as a normal child.

For header-section embeds, the extracted section already starts at a specific heading level. The adjustment is applied to that section's content as-is.

### Deduplication: Tracking Inline Embeds

When `![[Note Title]]` appears in a parent note's body, and that same note is also a direct child of the parent (via frontmatter or YML), the note should NOT appear twice. The embed takes precedence for ordering; the child is suppressed from the normal children section.

**Implementation:** During the compose walk, before appending children:

1. Scan the current note's content for embed references
2. Resolve each embed reference to a UUID
3. Add those UUIDs to the `visited` map (or a parallel `embeddedSet`)
4. When iterating `childrenMap[uuid]`, skip any UUID that was already embedded inline

This ensures the user's explicit embed placement is respected without duplication.

### Recursive Expansion of Embedded Note's Children

A note embedded via `![[...]]` gets its own children expanded beneath it, following the same rules as the normal compose walk. This means:

1. Expand the embed's content at the embed site
2. Immediately after, recurse into the embedded note's children (from `childrenMap`)
3. Children inherit the embedded note's depth + 1

This gives the embed full tree semantics -- it behaves as if the embedded note were a child at that position, but the user controls exactly where it appears.

### Compose Walk with Embeds: Detailed Design

**Why this is the hardest part of the implementation.** The current `composeTextWithSourceMap` walk writes each note's content in one shot to a `strings.Builder` (`b.WriteString(content)` at compose.go:346), then recurses into children sequentially. The source map tracks `startLine`/`endLine` per note, computed from `nextLine` and `strings.Count(content, "\n")`.

Embed expansion breaks this model: a note's content must be split around `![[...]]` lines, with each embed expanding into a full subtree (the embedded note + its children) before the next segment of the parent continues. Line counts are not knowable upfront because each embed's expansion is recursive. The walk must be restructured from "write content, then children" to "write content segments interleaved with embed subtrees, then non-embedded children."

#### Segment-based approach

Instead of writing content as a single string, split it into **segments** delimited by embed lines:

```go
// contentSegment represents a piece of a note's content between embeds.
type contentSegment struct {
    Text string   // The content text for this segment
}

// embedRef represents a resolved embed within a note's content.
type embedRef struct {
    NoteRef  string // Original reference text from ![[...]]
    Header   string // Optional header for ![[Note#Header]]
    UUID     string // Resolved UUID (empty if unresolvable)
    LineNum  int    // Line number in the original (pre-transform) content
}

// splitContentByEmbeds splits transformed content into segments and embeds.
// Returns alternating: segment[0], embed[0], segment[1], embed[1], ..., segment[N]
// There is always one more segment than embeds (segments before first embed,
// between each pair, and after last embed). Segments may be empty strings.
func splitContentByEmbeds(content string) ([]contentSegment, []embedRef)
```

#### Revised walk (pseudocode)

```
walk(uuid, depth):
    if visited[uuid]: return
    visited[uuid] = true

    content = loadNote(uuid).Content
    content = applyTransforms(content, depth)  // strip-title, strip-global-tags, heading adjust

    if !expandEmbeds:
        // Original path: write content, recurse children
        writeSeparator(depth)
        recordSourceMap(uuid, content)
        write(content)
        for childUUID in childrenMap[uuid]:
            walk(childUUID, depth+1)
        return

    // Split content around ![[...]] lines
    segments, embeds = splitContentByEmbeds(content)
    embeddedUUIDs = set()

    // Resolve all embeds upfront (needed for deduplication)
    for each embed in embeds:
        embed.UUID = resolveEmbed(embed.NoteRef)
        if embed.UUID != "":
            embeddedUUIDs.add(embed.UUID)

    // Write interleaved segments and embed expansions
    for i, segment in enumerate(segments):
        if segment.Text != "":
            if i == 0:
                // First segment: normal separator + source map for this note
                writeSeparator(depth)
                // Source map: record this segment as belonging to uuid
                recordSourceMapSegment(uuid, segment.Text)
            else:
                // Continuation segment (after an embed): still belongs to uuid
                recordSourceMapSegment(uuid, segment.Text)
            write(segment.Text)

        // After each segment (except the last), expand the corresponding embed
        if i < len(embeds):
            embed = embeds[i]
            if embed.UUID == "" || visited[embed.UUID]:
                // Unresolvable or circular: leave ![[...]] line as-is
                write(embed.originalLine)
                continue

            visited[embed.UUID] = true

            // Load and transform the embedded note
            embedContent = loadAndTransform(embed.UUID, depth+1, embed.Header)
            writeSeparator(depth+1)
            recordSourceMap(embed.UUID, embedContent)
            write(embedContent)

            // Recurse into embedded note's children
            for childUUID in childrenMap[embed.UUID]:
                walk(childUUID, depth+2)

    // Append non-embedded children (deduplication)
    for childUUID in childrenMap[uuid]:
        if childUUID not in embeddedUUIDs:
            walk(childUUID, depth+1)
```

#### Source map changes

The current `sourceEntry` maps one contiguous line range per note. With embeds, a single note's content may be split across non-contiguous line ranges (segment before embed, segment after embed). Two options:

**Option A: Multiple entries per note.** A note like "Project Hub" that has content before and after an embed gets two `sourceEntry` records with the same UUID but different line ranges. Downstream consumers already iterate the source map linearly, so this works without breaking the contract. The source map simply has more entries.

**Option B: Nest embed entries inside parent entries.** Add an `Embeds` field to `sourceEntry` that contains child source entries. This preserves the one-entry-per-note invariant but changes the schema.

**Recommendation: Option A.** It is simpler, backward compatible (source map is still a flat list of `{uuid, path, title, start_line, end_line}`), and downstream consumers don't need changes. The only difference is that a UUID may appear multiple times in the source map.

#### Transform ordering

The sequencing is: (1) apply transforms (strip-title, strip-global-tags, heading adjustment) to the host note, (2) scan for `![[...]]` lines, (3) apply transforms to each embedded note independently. This is correct because `adjustHeadings` operates on `^(#{1,6})\s` which does not match `![[...]]` lines, so transforms do not corrupt embed syntax. This should be documented as an invariant with a test that verifies it.

#### Unifying `composeTextWithSourceMap` and `composeJSON`

Currently the codebase has two separate recursive walk functions that duplicate the same traversal logic:

- `composeTextWithSourceMap` (compose.go:275-363): builds a flat string + source map
- `composeJSON` (compose.go:459-503): builds a tree of `composeNode` structs

Both duplicate visited tracking, content transforms, depth limiting, and child recursion. Adding embed expansion to both independently would double the implementation and maintenance burden.

**Refactor: single walk, multiple renderers.** Replace both with a single walk that produces an intermediate tree structure, then render text or JSON from that tree:

```go
// composeTree is the intermediate representation produced by the unified walk.
type composeTree struct {
    UUID     string
    Title    string
    Path     string
    Depth    int
    Content  string         // Transformed content (after strip-title, heading adjust, etc.)
    Segments []composeSegment // Only populated when --expand-embeds; nil otherwise
    Children []*composeTree
}

// composeSegment represents one piece of a note's content between embeds.
type composeSegment struct {
    Text  string       // Content text for this segment
    Embed *composeTree // If non-nil, an embed subtree to render after this text segment
}
```

The walk populates this tree once. Then:

- **Text output:** Flatten the tree depth-first, writing segments and separators, tracking `nextLine` for source map entries.
- **JSON output:** Convert `composeTree` to `composeNode` for serialization. Add `Embedded bool` field to `composeNode` so embeds are distinguishable from regular children in the JSON tree. Embeds appear in the `Children` array at their position (interleaved with non-embedded children based on where `![[...]]` appeared in the parent content).
- **Explain output:** Walk the tree and emit decision log entries instead of content.

This refactor should be done as a prerequisite step (Phase 3.5) that passes all existing tests with no behavior change before any embed logic is added. The existing `composeText` and `composeJSON` tests serve as the regression suite.

```go
// Updated composeNode for JSON output
type composeNode struct {
    UUID            string        `json:"uuid"`
    Title           string        `json:"title"`
    Path            string        `json:"path"`
    Embedded        bool          `json:"embedded,omitempty"`  // true if this node was inlined via ![[...]]
    Content         string        `json:"content,omitempty"`
    Children        []composeNode `json:"children,omitempty"`
    ComposedContent string        `json:"composed_content,omitempty"`
    SourceMap       []sourceEntry `json:"source_map,omitempty"`
}
```

### CLI Flags

```
ruin compose <note> --expand-embeds
ruin compose <note> --explain
```

| Flag | Short | Description |
|------|-------|-------------|
| `--expand-embeds` | | Expand `![[note]]` embeds inline with referenced note content |
| `--explain` | | Print a decision log instead of composed content |

`--expand-embeds` defaults to off, preserving backward compatibility. When off, `![[...]]` lines pass through as-is.

### Explain Mode (`--explain`)

When `--explain` is passed, the compose command performs the full composition walk but **omits the actual note content**. Instead, it outputs a recursive, human-readable decision log describing every action the logic takes.

This is a debugging and transparency tool -- it lets users understand exactly how compose resolves the tree without wading through the full output.

**Decision types logged:**

| Decision | Example output |
|----------|---------------|
| Root resolution | `ROOT: "Project Hub" (uuid: abc-123, path: Project Hub.md)` |
| Include child | `CHILD: "Architecture" (uuid: def-456) -- child of "Project Hub", depth 1` |
| Expand embed | `EMBED: "Architecture" (uuid: def-456) -- expanded from ![[Architecture]] in "Project Hub" at line 5, depth 1` |
| Header section embed | `EMBED: "Roadmap#Q1 Goals" (uuid: ghi-789) -- section extract from ![[Roadmap#Q1 Goals]] in "Project Hub" at line 9, depth 1` |
| Skip duplicate | `SKIP: "Architecture" (uuid: def-456) -- already embedded inline in "Project Hub", not appending as child` |
| Skip visited | `SKIP: "Architecture" (uuid: def-456) -- already visited (circular reference)` |
| Depth limit | `SKIP: "Deep Note" (uuid: jkl-012) -- max depth 3 reached` |
| Missing note | `WARN: ![[Missing Note]] -- could not resolve, left unexpanded` |
| Ambiguous ref | `WARN: ![[Ambiguous]] -- matched multiple notes, left unexpanded` |
| YML source | `SOURCE: children order from compose-spec.yml (not frontmatter)` |
| Frontmatter fallback | `SOURCE: children of "Architecture" from frontmatter (no YML override)` |
| Heading adjustment | `HEADING: "Architecture" adjusted from H1 to H2 (depth 1)` |
| Sort applied | `SORT: children of "Architecture" sorted by created:desc (frontmatter-sourced)` |
| Sort skipped | `SORT: children of "Project Hub" -- preserved YML order` |

**Output format (default):**

Indented tree with one line per decision:

```
ROOT: "Project Hub" (uuid: abc-123)
  EMBED: "Architecture" (uuid: def-456) -- from ![[Architecture]] at line 5, depth 1
    CHILD: "Architecture Details" (uuid: mno-345) -- child of "Architecture", depth 2
  SKIP: "Architecture" (uuid: def-456) -- already embedded inline, not appending as child
  EMBED: "Roadmap#Q1 Goals" (uuid: ghi-789) -- section extract at line 9, depth 1
```

**JSON output (`--explain --json`):**

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
      "source": "![[Architecture]]",
      "source_note": "Project Hub",
      "source_line": 5,
      "depth": 1
    },
    {
      "type": "skip_duplicate",
      "note": "Architecture",
      "uuid": "def-456",
      "reason": "already embedded inline in Project Hub"
    }
  ]
}
```

**Interaction with other flags:**

- `--explain` is mutually exclusive with `--edit` (no content to edit) and `--content` (no content to include)
- `--explain` works with `--file` (shows YML-sourced decisions)
- `--explain` works with `--expand-embeds` (shows embed decisions); without `--expand-embeds`, embeds are not listed as decisions
- `--explain` respects `--depth` (shows depth-limit skip decisions)
- `--explain` respects `--strip-title`, `--strip-global-tags`, `--normalize-headers` -- the explain log reports the decisions that *would* be made with those flags (e.g., HEADING adjustment entries)

## File/Module Layout

### New files

| File | Purpose |
|------|---------|
| `internal/commands/compose_walker.go` | `composeWalker` struct, `Walk()` method, `composeTree`/`composeSegment` types |
| `internal/commands/compose_walker_test.go` | Tests for unified walker (regression suite from existing tests) |
| `internal/commands/compose_render.go` | `renderText`, `renderJSON`, `renderEditList`, `renderExplain` |
| `internal/commands/compose_render_test.go` | Tests for renderers |
| `internal/note/embed.go` | Embed syntax parsing: `FindEmbeds`, `ExtractSection` |
| `internal/note/embed_test.go` | Tests for embed parsing and section extraction |
| `internal/commands/compose_yml.go` | YML composition file parsing and tree building |
| `internal/commands/compose_yml_test.go` | Tests for YML composition |

### Modified files

| File | Change |
|------|--------|
| `internal/commands/compose.go` | Slim down to flag setup + `RunE` that delegates to walker + renderers; add `--file`, `--expand-embeds`, `--explain` flags. Remove `composeTextWithSourceMap`, `composeJSON`, `collectTreeNotes` (moved to walker/render). Keep `adjustHeadings`, `normalizeHeadings`, `isListOnlyContent` helpers. |
| `internal/commands/parent.go` | Add `--file` flag to `parent save`; update `ParentEntry` handling |
| `internal/vault/vault.go` | Add `File` field to `ParentEntry` struct |
| `internal/commands/resolve.go` | Handle file-based parent bookmarks (resolve to root note) |
| `cmd/ruin/main.go` | No changes needed (compose and parent commands already registered) |
| `docs/cli-reference.md` | Document new flags and YML format |

## Edge Cases

### Circular references
Already handled by the `visited` map in the compose walk. If note A embeds note B which embeds note A, the second visit to A is a no-op. Emit a stderr warning: `warning: skipping circular embed of "Note A"`.

### Missing notes
If an embed `![[Missing Note]]` cannot be resolved, emit a stderr warning and leave the `![[Missing Note]]` line as-is in the output (do not expand, do not error). This matches the existing behavior of `RefreshLinkedCards` which warns on unresolvable links.

### Deeply nested embeds
Embeds can nest: note A embeds B which embeds C. The `visited` map prevents infinite loops. The `--depth` flag applies to the total tree depth including embed-introduced levels. If max depth is reached, remaining embeds are left unexpanded.

### Notes that are both embedded and children
The embed takes precedence. The note appears at the embed site, not in the children section. If a note is embedded in a grandparent but is a child of a different parent in the tree, it appears at whichever point the walk visits it first.

### Header section not found
If `![[Note#Nonexistent Header]]` references a header that does not exist in the note, emit a stderr warning and leave the embed line unexpanded.

### YML file references missing notes
If a `note` entry in the YML file cannot be resolved, emit a stderr warning and skip that entry (do not halt composition).

### Ambiguous note references in embeds
If an embed reference matches multiple notes, emit a stderr error and leave the embed unexpanded. This matches `ResolveNote` behavior which returns an error on ambiguous matches.

## Example YML Files and Usage

### Basic YML composition

```yaml
# project-alpha.yml
root: "Project Alpha Hub"
children:
  - note:"Introduction"
  - note:"Architecture"
    children:
      - note:"Backend Design"
      - note:"Frontend Design"
  - note:"Roadmap"
```

Usage:
```bash
ruin compose --file project-alpha.yml --strip-title --strip-global-tags
ruin compose --file project-alpha.yml --json --content
```

### Saving as a named parent

```bash
ruin parent save alpha --file project-alpha.yml
ruin compose alpha   # Equivalent to: ruin compose --file project-alpha.yml
```

### Hybrid YML + frontmatter children

```yaml
# When "Architecture" has frontmatter children, they are included automatically
root: "Project Alpha Hub"
children:
  - note:"Introduction"
  - note:"Architecture"    # Its frontmatter children appear beneath it
  - note:"Roadmap"
```

## Example Compose Output with Embed Expansion

### Input notes

**Project Hub.md:**
```markdown
# Project Hub

Overview of the project.

![[Architecture]]

Next steps are outlined below.

![[Roadmap#Q1 Goals]]
```

**Architecture.md** (child of Project Hub, with its own child "Architecture Details"):
```markdown
# Architecture

The system uses a microservice architecture.
```

**Architecture Details.md** (child of Architecture):
```markdown
# Architecture Details

Service mesh configuration and deployment.
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

### Output: `ruin compose "Project Hub" --expand-embeds --normalize-headers`

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

**What happened:**

1. Root note content starts rendering at depth 0
2. `![[Architecture]]` is encountered -- Architecture renders at depth 1 (H1 becomes H2)
3. Architecture's child "Architecture Details" renders at depth 2 (H1 becomes H3)
4. Architecture is marked as visited, so it does NOT appear again in the normal children section
5. The root note content continues ("Next steps...")
6. `![[Roadmap#Q1 Goals]]` extracts only the Q1 Goals section from Roadmap, rendered at depth 1
7. Roadmap is not a child of Project Hub, so no duplication concern

## Implementation Sequence

### Phase 1: Unify compose walks (prerequisite refactor)

The current codebase has three walk functions that duplicate traversal logic:
- `composeTextWithSourceMap` -- flat string + source map
- `composeJSON` -- tree of `composeNode` structs
- `collectTreeNotes` -- flat list for `--edit`

Refactor into a single `composeWalker` that produces a `composeTree`, then render text/JSON/edit from that tree.

1. Introduce `composeWalker` struct encapsulating shared state: `visited`, `childrenMap`, `maxDepth`, transform options (`stripTitle`, `stripGlobalTags`, `normalizeHeaders`)
2. Implement `walker.Walk(uuid, depth) *composeTree` -- single recursive walk producing the intermediate tree
3. Implement `renderText(tree) (string, []sourceEntry)` -- flattens tree to text + source map
4. Implement `renderJSON(tree, includeContent) composeNode` -- converts tree to JSON-serializable struct
5. Implement `renderEditList(tree) []SearchResult` -- converts tree to flat list for `--edit`
6. Update `NewComposeCmd.RunE` to use walker + renderers
7. All existing compose tests must pass with no behavior change. This is a pure refactor.

This phase eliminates the duplicated walk logic and creates the foundation for embed expansion, YML injection, and explain mode to be implemented once in the walker rather than per-renderer.

### Phase 2: Embed Parsing (`internal/note/embed.go`)
1. Implement `FindEmbeds(content string) []EmbedRef` -- finds `![[...]]` on standalone lines
2. Implement `ExtractSection(content, header string) (string, error)` -- extracts a header section
3. Full test coverage for both functions

### Phase 3: YML Composition Parsing (`internal/commands/compose_yml.go`)
1. Define `ComposeSpec` struct matching the YML format
2. Implement `ParseComposeFile(path string) (*ComposeSpec, error)`
3. Implement `BuildChildrenMapFromSpec(spec, vault, index) (rootUUID, childrenMap, ymlParents, error)` -- returns a `ymlParents set[string]` of parent UUIDs whose children were YML-specified
4. Test with various YML structures

### Phase 4: Wire YML into Compose Command
1. Add `--file` flag to compose command
2. When `--file` is set, parse YML and build `childrenMap`, pass to `composeWalker`
3. Update `Args` validation (0 or 1 args depending on `--file`)

### Phase 5: Embed Expansion in Compose (largest phase)
1. Add `--expand-embeds` flag
2. Implement `splitContentByEmbeds` to split note content into segments and embed refs
3. Extend `composeWalker.Walk` to populate `composeTree.Segments` when `--expand-embeds` is set
4. Implement deduplication logic (embeddedUUIDs set, skip in children loop)
5. Handle recursive embed expansion with depth tracking
6. Update `renderText` to write segments interleaved with embed subtrees, source map uses multiple entries per note (Option A)
7. Update `renderJSON` to mark embedded nodes with `Embedded: true`
8. Add invariant test: heading adjustment does not corrupt `![[...]]` lines

Since the walker is already unified (Phase 1), embed logic lives in one place. The renderers just need to handle the `Segments` field.

### Phase 6: Parent Save Enhancement
1. Add `File` field to `ParentEntry`
2. Add `--file` flag to `parent save` subcommand
3. Update resolve logic to handle file-based bookmarks
4. Update `parent list` display to show file-based entries

### Phase 7: Explain Mode
1. Add `--explain` flag to compose command
2. Implement `renderExplain(tree) (string, []ExplainDecision)` -- walks the `composeTree` and emits decision log entries instead of content
3. Text and JSON formatters for the decision log
4. Tests for explain output across all decision types

### Phase 8: Documentation and Integration Tests
1. Update `docs/cli-reference.md` with new flags and YML format
2. Create `docs/compose-advanced.md` -- a standalone user-facing document covering YML composition, embed expansion, and explain mode, with examples and recipes for downstream consumers
3. Add integration-style tests covering the full compose pipeline with YML and embeds
