# Breaking Change: Tag Field Separation

## Summary

The `tags` frontmatter field now stores **global tags only** (categorization tags). A separate `inline-tags` field stores tags found within content body paragraphs. Previously, `tags` contained both merged together.

## What changed

### Frontmatter

**Before:**
```yaml
tags: ["#meeting", "#work", "#followup"]
inline-tags: ["#followup"]
```

**After:**
```yaml
tags: ["#meeting", "#work"]
inline-tags: ["#followup"]
```

`tags` no longer includes inline tags. The two fields are now fully disjoint.

### JSON output

Search and get commands now include an `inline_tags` field in JSON output.

**Before:**
```json
{
  "uuid": "...",
  "title": "...",
  "tags": ["#meeting", "#work", "#followup"]
}
```

**After:**
```json
{
  "uuid": "...",
  "title": "...",
  "tags": ["#meeting", "#work"],
  "inline_tags": ["#followup"]
}
```

### Search behavior

Default `ruin search "#tag"` still matches both global and inline tags -- no change for basic searches. Two new flags allow narrowing:

- `--global-tags` -- only match global tags
- `--inline-tags` -- only match inline tags

### Tags index

`.ruin/tags.yml` continues to contain all tags (global + inline). No change here.

## Migration

Run `ruin doctor` to rewrite all note frontmatter with the new separation. No data is lost -- tags are re-extracted from content.

## Who is affected

- **Consumers parsing `tags` from frontmatter** (YAML): the field is now smaller. If you were relying on `tags` to find inline annotations like `#followup` or `#todo`, check `inline-tags` as well.
- **Consumers parsing JSON output**: the `tags` array is smaller. Use the new `inline_tags` field for inline tags, or combine both for the previous behavior.
- **Saved queries**: queries using `#tag` syntax are unaffected (search checks both fields by default). No action needed.

## New command: `pick`

A new `ruin pick` command extracts individual lines annotated with inline tags:

```
ruin pick "#followup"
ruin pick "#followup" "#todo"       # lines with BOTH tags (AND)
ruin pick "#followup" "#todo" --any # lines with EITHER tag (OR)
ruin pick "#followup" --json        # grouped by note
```

This enables line-level queries for action items, follow-ups, and other contextual annotations scattered across notes.
