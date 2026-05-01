# Obsidian Compatibility

Ruin and Obsidian share a vault format — Markdown files with YAML
frontmatter — and most things work in both tools without ceremony. This
doc covers the common ground, the one place the two tools differ
(where tags live), and the config flag that brings ruin in line with
Obsidian's convention.

## Shared ground

Both tools read and preserve the same surface:

- **Markdown bodies** — standard CommonMark.
- **YAML frontmatter** — between `---` delimiters at the top of the
  file. Ruin's parser preserves key order, comments, and quote styles
  for fields it doesn't manage.
- **Wiki links** — `[[Note Title]]` and `[[Note Title|display]]`. Ruin
  resolves them to UUIDs and caches the mapping in `linked-cards:` for
  fast lookup.
- **Embeds** — `![[Note Title]]` and `![[Note Title#section]]`. Ruin
  expands embeds at compose time; Obsidian renders them inline.
- **Body tags** — `#tag`, `#category/sub`, and `#multi word#` (ruin
  extension; Obsidian doesn't parse the spaced form).

## Where tags live

This is the one place ruin and Obsidian use different conventions:

- **Obsidian** extracts `#tag` from body content at read time and
  surfaces them in its tag pane and search. It does not write body tags
  back into frontmatter.
- **Ruin** mirrors body tags into frontmatter on save: `tags:` for
  global (tag-only line) tags, `inline-tags:` for tags on lines with
  other content. It also mirrors them into `.ruin/titles.json` as a
  hot-path index.

Both representations carry the same information; they just live in
different places. When ruin and Obsidian share a vault, the mismatch
shows up as ruin adding `tags:` and `inline-tags:` keys that Obsidian
doesn't write — visible noise in Obsidian's properties panel.

## Recommended config for shared vaults

```yaml
# ~/.config/ruin/config.yml
tag_frontmatter: false
```

Or per-invocation:

```bash
RUIN_TAG_FRONTMATTER=false ruin init ~/Obsidian-vault
```

With this setting, ruin reads Obsidian-style frontmatter normally but
does **not** write `tags:` or `inline-tags:` back. Body content remains
the source of truth for own tags; `.ruin/titles.json` carries the index
ruin's own queries need.

## A few things ruin still adds to frontmatter

Even with `tag_frontmatter=false`, ruin needs a few keys to function:

- **`uuid:`** — required for ruin's identity model. Obsidian ignores
  unknown frontmatter keys, so this is harmless.
- **`inherited-tags:`** — added when a note's parent has global tags.
  Set `RUIN_TAG_INHERITANCE=false` (or `tag_inheritance: false` in
  config) to disable inheritance entirely.
- **`linked-cards:`** — resolved UUIDs from `[[wiki links]]` in
  content.
- **`updated:`** and **`dates:`** — set on edits; not Obsidian-managed
  but unobtrusive.

## A few Obsidian conventions ruin doesn't interpret today

- **Aliases** (`aliases:`) — preserved as user-defined frontmatter
  (Extra), but not used for ruin's wiki-link resolution.
- **Inline metadata** (`key:: value` in body content) — not parsed.
- **Daily-note plugin conventions** — not interpreted.
- **Block references** (`^block-id`) — preserved as text in the body.

## When in doubt

If a workflow needs ruin and Obsidian to round-trip without surprise,
start with `tag_frontmatter=false` and let the body be the only place
tags live. Ruin's queries continue to work via `.ruin/titles.json`;
Obsidian's tag pane continues to work via body extraction; both tools
see the same tags without stepping on each other's writes.
