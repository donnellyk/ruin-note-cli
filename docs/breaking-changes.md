# Breaking Changes

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
