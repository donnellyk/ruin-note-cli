# Breaking Changes

## Date grammar simplified

**Version**: Current

The date parser has been simplified to only support ISO formats and simple helpers. The following date syntax has been removed:

### Removed from date filters (`created:`, `updated:`, `before:`, `after:`)
- `this-year`, `last-year`, `next-year`
- Day names: `monday` through `sunday`
- Duration shorthand: `7d`, `2w`, `3m`, `2y`
- Duration longhand: `7-days`, `2-weeks`, `3-months`, `2-years`

### Removed from date tokens (`@` syntax)
- `@monday` through `@sunday`
- `@next-year`
- `@2-days`, `@3-weeks`, `@2-months`, `@2-years`

### Restored
- `this-week`, `last-week`, `next-week` (week starts Monday, ISO standard)
- `this-month`, `last-month`, `next-month`

### What remains
- **ISO formats**: `2025-01-28`, `2025-01`, `2025`
- **Simple helpers**: `today`, `yesterday`, `tomorrow`
- **Period helpers**: `this-week`, `last-week`, `next-week`, `this-month`, `last-month`, `next-month`

### Migration

| Before | After |
|--------|-------|
| `created:7d` | `created:2025-01-22` (use explicit start date) |
| `@monday` | `@2025-02-03` (use explicit date) |
| `@2-days` | `@2025-01-30` (use explicit date) |

---

## `parent set` and `parent remove` removed

**Version**: Current (note commands)

The `parent set` and `parent remove` subcommands have been removed from the `parent` command group. Their functionality is now available through `note set`.

### Migration

| Before | After |
|--------|-------|
| `ruin parent set <child> <parent>` | `ruin note set <child> --parent <parent>` |
| `ruin parent set <child> <parent> -f` | `ruin note set <child> --parent <parent> -f` |
| `ruin parent remove <note>` | `ruin note set <note> --no-parent` |

### What stays the same

The `parent` command retains all read-only and bookmark operations:
- `parent get` - show parent of a note
- `parent children` - list children
- `parent tree` - show parent-child tree
- `parent save` - save a named bookmark
- `parent list` - list bookmarks
- `parent delete` - delete a bookmark

### Why

The new `note` command group consolidates all single-note mutations (`note set`, `note append`, `note merge`). This avoids confusion between vault-wide tag operations (`tags delete` removes a tag from all notes) and note-scoped operations (`note set --remove-tag` removes a tag from one note).

---

## Pick date flags consolidated into `--filter`

**Version**: Current

The `pick` command's 7 separate date flags have been replaced by a single `--filter` flag that accepts the same query syntax as `search`.

### Removed flags
`--date`, `--created`, `--updated`, `--before`, `--after`, `--on`, `--between`

### Migration

| Before | After |
|--------|-------|
| `ruin pick "#tag" --date @today` | `ruin pick "#tag" --filter "@today"` |
| `ruin pick "#tag" --created today` | `ruin pick "#tag" --filter "created:today"` |
| `ruin pick "#tag" --updated 2025-01` | `ruin pick "#tag" --filter "updated:2025-01"` |
| `ruin pick "#tag" --before 2025-06` | `ruin pick "#tag" --filter "before:2025-06"` |
| `ruin pick "#tag" --after 2025-01` | `ruin pick "#tag" --filter "after:2025-01"` |
| `ruin pick "#tag" --on 2025-01-15` | `ruin pick "#tag" --filter "on:2025-01-15"` |
| `ruin pick "#tag" --between 2025-01,2025-06` | `ruin pick "#tag" --filter "between:2025-01,2025-06"` |

### Why

The 7 date flags duplicated search's query syntax. The `--filter` flag reuses `parseQuery` directly, making pick consistent with search and supporting all the same filter terms (including combinations like `created:today @tomorrow`).
