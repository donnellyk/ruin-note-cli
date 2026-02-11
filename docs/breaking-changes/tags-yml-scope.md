# Breaking Change: Tag Index Scope Field

## Summary

Each entry in `.ruin/tags.yml` now includes a `scope` field — a list indicating where the tag is used across the vault: `global`, `inline`, or both.

## What changed

### tags.yml

**Before:**
```yaml
tags:
  - name: '#daily'
    count: 29
  - name: '#followup'
    count: 20
```

**After:**
```yaml
tags:
  - name: '#daily'
    count: 29
    scope:
      - global
      - inline
  - name: '#followup'
    count: 20
    scope:
      - inline
```

`scope` is a list of strings. Possible values:
- `["global"]` — tag is only used as a global tag (categorization, at top/end of notes)
- `["inline"]` — tag is only used as an inline tag (contextual annotations within content)
- `["global", "inline"]` — tag is used in both contexts across different notes

The list format is extensible — new scope values may be added in the future without changing the field type.

### tags list output

**Before:**
```
#daily (29)
#followup (20)
```

**After:**
```
#daily (29) [global, inline]
#followup (20) [inline]
```

### JSON output (tags list --json)

**Before:**
```json
[
  {"name": "#daily", "count": 29},
  {"name": "#followup", "count": 20}
]
```

**After:**
```json
[
  {"name": "#daily", "count": 29, "scope": ["global", "inline"]},
  {"name": "#followup", "count": 20, "scope": ["inline"]}
]
```

### UpdateTagsIndex / DecrementTagsIndex API

These methods now accept separate global and inline tag slices:

**Before:**
```go
vlt.UpdateTagsIndex(allTags)
vlt.DecrementTagsIndex(allTags)
```

**After:**
```go
vlt.UpdateTagsIndex(globalTags, inlineTags)
vlt.DecrementTagsIndex(globalTags, inlineTags)
```

### RebuildTagsIndex API

**Before:**
```go
vlt.RebuildTagsIndex(tagCounts)
```

**After:**
```go
vlt.RebuildTagsIndex(totalCounts, globalTagSet, inlineTagSet)
```

Where `globalTagSet` and `inlineTagSet` are `map[string]bool` indicating which scopes each tag has been observed in.

## Migration

Run `ruin doctor` to rebuild `tags.yml` with accurate scope data. Scope is computed from the content of every note in the vault.

## Who is affected

- **Consumers parsing `tags.yml` directly**: the YAML structure has a new `scope` field on each entry. Existing `name` and `count` fields are unchanged.
- **Consumers parsing `tags list --json`**: the JSON objects now include a `scope` array. Existing fields are unchanged.
- **Go callers of `UpdateTagsIndex` / `DecrementTagsIndex` / `RebuildTagsIndex`**: function signatures changed. Pass global and inline tags separately instead of a merged list.
- **Search, log, pick commands**: no user-facing behavior change. These commands are unaffected.
